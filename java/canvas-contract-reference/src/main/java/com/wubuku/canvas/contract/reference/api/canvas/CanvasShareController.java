package com.wubuku.canvas.contract.reference.api.canvas;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ApiEnvelope;
import com.wubuku.canvas.contract.reference.api.common.IdentityDto;
import com.wubuku.canvas.contract.reference.contract.ApiException;
import com.wubuku.canvas.contract.reference.contract.ReferenceService;
import com.wubuku.canvas.contract.reference.contract.ReferenceStore;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.Parameter;
import io.swagger.v3.oas.annotations.media.Content;
import io.swagger.v3.oas.annotations.media.Schema;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.tags.Tag;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.Valid;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.concurrent.ConcurrentHashMap;
import org.springframework.http.ResponseEntity;
import org.springframework.http.HttpHeaders;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api")
@Tag(name = "Canvas public sharing", description = "Strongly typed share management and anonymous canvas read endpoints.")
public class CanvasShareController {
    private final ReferenceService service;
    private final ReferenceStore store;
    private final ConcurrentHashMap<String, Boolean> firstShareCreates = new ConcurrentHashMap<>();

    public CanvasShareController(ReferenceService service, ReferenceStore store) {
        this.service = service;
        this.store = store;
    }

    @GetMapping("/canvas-projects/{id}/share")
    @Operation(operationId = "reference.canvas.share.read", summary = "Read the current user's canvas share status")
    @ApiResponse(responseCode = "200", description = "Typed share status", content = @Content(schema = @Schema(implementation = CanvasShareResponse.class)))
    @ApiResponse(responseCode = "404", description = "Canvas not found")
    public ResponseEntity<ApiEnvelope<CanvasShareResponse>> read(
            HttpServletRequest request,
            @Parameter(description = "Stable canvas identity") @PathVariable String id) {
        String userId = service.requireUser(request);
        JsonNode canvas = ownedCanvas(id, userId);
        CanvasShareDto share = visibleShare(canvas.path("share"));
        return ResponseEntity.ok(ApiEnvelope.success(new CanvasShareResponse(share)));
    }

    @PostMapping("/canvas-projects/{id}/share")
    @Operation(operationId = "reference.canvas.share.create", summary = "Create or rotate a public canvas share")
    @ApiResponse(responseCode = "200", description = "Typed share status", content = @Content(schema = @Schema(implementation = CanvasShareResponse.class)))
    @ApiResponse(responseCode = "400", description = "Invalid expiry")
    @ApiResponse(responseCode = "404", description = "Canvas not found")
    public ResponseEntity<ApiEnvelope<CanvasShareResponse>> create(
            HttpServletRequest request,
            @Parameter(description = "Stable canvas identity") @PathVariable String id,
            @Valid @RequestBody(required = false) CanvasShareRequest body) {
        String userId = service.requireUser(request);
        if (request.getContentLengthLong() > 8192) {
            throw new ApiException(400, 400, "bad_request", "share request is too large");
        }
        int expiresDays = body == null || body.expiresDays() == null ? 0 : body.expiresDays();
        boolean rotate = body != null && Boolean.TRUE.equals(body.rotate());
        if (expiresDays < 0 || expiresDays > 365) {
            throw new ApiException(400, 400, "bad_request", "expiresDays must be between 0 and 365");
        }
        boolean firstCreate = !store.snapshot().path("canvases").path(id).path("share").has("token");
        if (firstCreate && !rotate && firstShareCreates.putIfAbsent(id, Boolean.TRUE) != null) {
            throw new ApiException(500, 500, "internal", "internal");
        }
        try {
            CanvasShareDto share = store.update(root -> {
                ObjectNode canvas = mutationCanvas(root, id, userId);
                ObjectNode current = canvas.with("share");
                if ((isActive(current) || current.has("token")) && !rotate) {
                    current.put("enabled", true);
                    writeExpiry(current, expiresDays);
                    return visibleShare(current);
                }
                current.removeAll();
                current.put("enabled", true).put("token", store.newId("share"));
                writeExpiry(current, expiresDays);
                current.put("createdAt", Instant.now().toString());
                return visibleShare(current);
            });
            return ResponseEntity.ok(ApiEnvelope.success(new CanvasShareResponse(share)));
        } finally {
            if (firstCreate) {
                firstShareCreates.remove(id);
            }
        }
    }

    @DeleteMapping("/canvas-projects/{id}/share")
    @Operation(operationId = "reference.canvas.share.revoke", summary = "Revoke a public canvas share")
    @ApiResponse(responseCode = "200", description = "Canvas identity", content = @Content(schema = @Schema(implementation = IdentityDto.class)))
    public ResponseEntity<ApiEnvelope<IdentityDto>> revoke(
            HttpServletRequest request,
            @Parameter(description = "Stable canvas identity") @PathVariable String id) {
        String userId = service.requireUser(request);
        store.update(root -> {
            ObjectNode canvas = mutationCanvas(root, id, userId);
            canvas.with("share").removeAll();
            return null;
        });
        return ResponseEntity.ok(ApiEnvelope.success(new IdentityDto(id)));
    }

    @GetMapping("/public/canvas-shares/{token}")
    @Operation(operationId = "reference.canvas.share.public-read", summary = "Read a shared canvas without a session")
    @ApiResponse(responseCode = "200", description = "Current typed canvas document", content = @Content(schema = @Schema(implementation = CanvasReadResponse.class)))
    @ApiResponse(responseCode = "404", description = "Share is missing, revoked, expired, or canvas deleted")
    public ResponseEntity<ApiEnvelope<CanvasReadResponse>> publicRead(
            @Parameter(description = "Opaque public share token") @PathVariable String token) {
        JsonNode canvas = findPublicCanvas(token);
        ObjectNode project = publicProject(canvas.path("project"), token);
        HttpHeaders headers = new HttpHeaders();
        headers.setCacheControl("no-store");
        headers.set("Referrer-Policy", "no-referrer");
        headers.set("X-Robots-Tag", "noindex, nofollow");
        return ResponseEntity.ok().headers(headers).body(ApiEnvelope.success(new CanvasReadResponse(service.canvasDocument(project))));
    }

    private JsonNode ownedCanvas(String id, String userId) {
        JsonNode canvas = store.snapshot().with("canvases").path(id);
        if (!canvas.isObject() || !userId.equals(ReferenceService.text(canvas, "ownerId")) || ReferenceService.bool(canvas, "deleted")) {
            throw new ApiException(404, 404, "not_found", "canvas not found");
        }
        return canvas;
    }

    private static ObjectNode ownedCanvas(ObjectNode root, String id, String userId) {
        JsonNode canvas = root.with("canvases").path(id);
        if (!canvas.isObject() || !userId.equals(ReferenceService.text(canvas, "ownerId")) || ReferenceService.bool(canvas, "deleted")) {
            throw new ApiException(404, 404, "not_found", "canvas not found");
        }
        return (ObjectNode) canvas;
    }

    private static ObjectNode mutationCanvas(ObjectNode root, String id, String userId) {
        JsonNode canvas = root.with("canvases").path(id);
        if (!canvas.isObject() || ReferenceService.bool(canvas, "deleted")) {
            throw new ApiException(404, 404, "not_found", "canvas not found");
        }
        if (!userId.equals(ReferenceService.text(canvas, "ownerId"))) {
            throw new ApiException(500, 500, "internal", "internal");
        }
        return (ObjectNode) canvas;
    }

    private JsonNode findPublicCanvas(String token) {
        JsonNode root = store.snapshot();
        for (JsonNode canvas : root.with("canvases")) {
            if (ReferenceService.bool(canvas, "deleted") || !token.equals(ReferenceService.text(canvas.path("share"), "token"))) {
                continue;
            }
            JsonNode share = canvas.path("share");
            if (!isActive(share)) {
                break;
            }
            return canvas;
        }
        throw new ApiException(404, 404, "not_found", "public canvas share not found");
    }

    private static ObjectNode publicProject(JsonNode project, String token) {
        ObjectNode copy = ReferenceService.copyObject(project);
        copy.remove("customExtension");
        for (JsonNode node : copy.withArray("nodes")) {
            if (!node.isObject()) {
                continue;
            }
            JsonNode metadata = node.path("metadata");
            String storageKey = ReferenceService.text(metadata, "storageKey");
            if (storageKey != null && storageKey.startsWith("resource:")) {
                String resourceId = storageKey.substring("resource:".length());
                ((ObjectNode) node.with("metadata")).put("content", "/api/public/canvas-shares/" + token + "/resources/" + resourceId + "/file");
            }
        }
        return copy;
    }

    private static CanvasShareDto visibleShare(JsonNode share) {
        if (!isActive(share)) {
            return disabledShare();
        }
        return new CanvasShareDto(true, ReferenceService.text(share, "token"), ReferenceService.text(share, "expiresAt"), ReferenceService.text(share, "createdAt"));
    }

    private static CanvasShareDto disabledShare() {
        return new CanvasShareDto(false, null, null, null);
    }

    private static boolean isActive(JsonNode share) {
        if (share == null || !share.isObject() || !ReferenceService.bool(share, "enabled")) {
            return false;
        }
        String expiresAt = ReferenceService.text(share, "expiresAt");
        return expiresAt == null || parseInstant(expiresAt).isAfter(Instant.now());
    }

    private static void writeExpiry(ObjectNode share, int expiresDays) {
        if (expiresDays == 0) {
            share.putNull("expiresAt");
        } else {
            share.put("expiresAt", Instant.now().plus(expiresDays, ChronoUnit.DAYS).toString());
        }
    }

    private static Instant parseInstant(String value) {
        try {
            return Instant.parse(value);
        } catch (RuntimeException exception) {
            return Instant.EPOCH;
        }
    }
}
