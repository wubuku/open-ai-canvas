package com.wubuku.canvas.contract.reference.api.canvas;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;
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
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import org.springframework.http.HttpHeaders;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/canvas-projects")
@Tag(name = "Canvas contract reference", description = "Strongly typed deterministic canvas document endpoints used as a migration reference.")
public class CanvasController {
    private final ReferenceService service;
    private final ReferenceStore store;

    public CanvasController(ReferenceService service, ReferenceStore store) {
        this.service = service;
        this.store = store;
    }

    @GetMapping
    @Operation(operationId = "reference.canvas.list", summary = "List the current user's typed canvas summaries")
    @ApiResponse(responseCode = "200", description = "Canvas summary library", content = @Content(schema = @Schema(implementation = CanvasLibraryResponse.class)))
    public ResponseEntity<ApiEnvelope<CanvasLibraryResponse>> list(
            HttpServletRequest request,
            @RequestParam(required = false) String page,
            @RequestParam(required = false) String pageSize,
            @RequestParam(required = false) String projectId,
            @RequestParam(required = false) String q,
            @RequestParam(required = false) String sort) {
        String userId = service.requireUser(request);
        List<JsonNode> projects = new ArrayList<>();
        for (JsonNode canvas : store.snapshot().with("canvases")) {
            if (!userId.equals(ReferenceService.text(canvas, "ownerId")) || ReferenceService.bool(canvas, "deleted")) {
                continue;
            }
            JsonNode project = canvas.path("project");
            boolean independent = "independent".equalsIgnoreCase(projectId);
            String canvasProjectId = ReferenceService.text(project, "projectId");
            if (projectId != null) {
                if (independent && canvasProjectId != null && !canvasProjectId.isBlank()) {
                    continue;
                }
                if (!independent && !projectId.equals(canvasProjectId)) {
                    continue;
                }
            }
            String title = ReferenceService.text(project, "title");
            if (q != null && (title == null || !title.toLowerCase().contains(q.toLowerCase()))) {
                continue;
            }
            projects.add(project);
        }
        if ("name".equalsIgnoreCase(sort)) {
            projects.sort(Comparator.comparing((JsonNode node) -> ReferenceService.text(node, "title"), Comparator.nullsLast(String::compareTo))
                    .thenComparing(node -> ReferenceService.text(node, "id")));
        } else if ("nodes".equalsIgnoreCase(sort)) {
            projects.sort(Comparator.comparingInt((JsonNode node) -> node.path("nodeCount").asInt()).reversed()
                    .thenComparing(node -> ReferenceService.text(node, "id")));
        } else {
            projects.sort(Comparator.comparing((JsonNode node) -> ReferenceService.text(node, "updatedAt"), Comparator.nullsLast(String::compareTo))
                    .reversed().thenComparing(node -> ReferenceService.text(node, "id")));
        }
        List<CanvasSummaryDto> summaries = projects.stream().map(service::canvasSummary).toList();
        if (page == null && pageSize == null) {
            return ResponseEntity.ok(ApiEnvelope.success(new CanvasLibraryResponse(null, null, null, null, summaries)));
        }
        int currentPage = parsePage(page, 1);
        int limit = pageSize == null ? 40 : Math.min(parsePage(pageSize, 40), 50);
        if (currentPage < 1 || currentPage > 1_000_000 || limit < 1) {
            throw new ApiException(400, 400, "bad_request", "page and pageSize must be positive");
        }
        int from = Math.min((currentPage - 1) * limit, summaries.size());
        int to = Math.min(from + limit, summaries.size());
        return ResponseEntity.ok(ApiEnvelope.success(new CanvasLibraryResponse(
                currentPage, limit, summaries.size(), to < summaries.size(), summaries.subList(from, to))));
    }

    @GetMapping("/{id}")
    @Operation(operationId = "reference.canvas.read", summary = "Read a typed canvas document with conditional ETag support")
    @ApiResponse(responseCode = "200", description = "Canvas document", content = @Content(schema = @Schema(implementation = CanvasReadResponse.class)))
    @ApiResponse(responseCode = "304", description = "Document unchanged")
    public ResponseEntity<ApiEnvelope<CanvasReadResponse>> read(
            HttpServletRequest request,
            @Parameter(description = "Stable canvas identity") @PathVariable String id) {
        String userId = service.requireUser(request);
        JsonNode canvas = ownedCanvas(id, userId);
        String etag = "W/\"canvas-" + canvas.path("revision").asLong() + "\"";
        HttpHeaders headers = canvasHeaders(etag);
        String ifNoneMatch = request.getHeader(HttpHeaders.IF_NONE_MATCH);
        if (ifNoneMatch != null && (ifNoneMatch.contains(etag) || ifNoneMatch.contains("\"canvas-" + canvas.path("revision").asLong() + "\""))) {
            return ResponseEntity.status(304).headers(headers).build();
        }
        return ResponseEntity.ok().headers(headers).body(ApiEnvelope.success(new CanvasReadResponse(service.canvasDocument(canvas.path("project")))));
    }

    @PutMapping("/{id}")
    @Operation(operationId = "reference.canvas.write", summary = "Create or compare-and-swap a typed canvas document")
    @ApiResponse(responseCode = "200", description = "Canvas summary after write", content = @Content(schema = @Schema(implementation = CanvasWriteResponse.class)))
    @ApiResponse(responseCode = "409", description = "Revision or ownership conflict")
    @ApiResponse(responseCode = "428", description = "Revision precondition missing")
    public ResponseEntity<ApiEnvelope<CanvasWriteResponse>> write(
            HttpServletRequest request,
            @Parameter(description = "Stable canvas identity") @PathVariable String id,
            @Valid @RequestBody CanvasWriteRequest body) {
        String userId = service.requireUser(request);
        if (body == null || body.project() == null || body.project().nodes() == null || body.project().connections() == null) {
            throw new ApiException(400, 400, "bad_request", "project is required");
        }
        ObjectNode input = service.mapper().valueToTree(body.project());
        if (!id.equals(ReferenceService.text(input, "id"))) {
            throw new ApiException(400, 400, "bad_request", "canvas id does not match path");
        }
        ObjectNode result = store.update(root -> upsert(root, userId, id, input, Boolean.TRUE.equals(body.repairMissingResources())));
        return ResponseEntity.ok(ApiEnvelope.success(new CanvasWriteResponse(service.canvasSummary(result))));
    }

    @GetMapping("/{id}/history")
    @Operation(operationId = "reference.canvas.history.list", summary = "List typed retained canvas history snapshots")
    @ApiResponse(responseCode = "200", description = "Canvas history", content = @Content(schema = @Schema(implementation = CanvasHistoryResponse.class)))
    public ResponseEntity<ApiEnvelope<CanvasHistoryResponse>> history(HttpServletRequest request, @PathVariable String id) {
        String userId = service.requireUser(request);
        JsonNode canvas = ownedCanvas(id, userId);
        List<CanvasHistorySnapshotDto> snapshots = new ArrayList<>();
        for (JsonNode snapshot : canvas.path("history")) {
            snapshots.add(service.canvasHistorySnapshot(snapshot));
        }
        return ResponseEntity.ok(ApiEnvelope.success(new CanvasHistoryResponse(snapshots, canvas.path("revision").asInt())));
    }

    @GetMapping("/{id}/history/{snapshotId}")
    @Operation(operationId = "reference.canvas.history.read", summary = "Read one typed canvas history snapshot")
    @ApiResponse(responseCode = "200", description = "History snapshot and document", content = @Content(schema = @Schema(implementation = CanvasHistoryReadResponse.class)))
    public ResponseEntity<ApiEnvelope<CanvasHistoryReadResponse>> historySnapshot(HttpServletRequest request, @PathVariable String id, @PathVariable String snapshotId) {
        String userId = service.requireUser(request);
        JsonNode canvas = ownedCanvas(id, userId);
        JsonNode snapshot = findSnapshot(canvas, snapshotId);
        return ResponseEntity.ok(ApiEnvelope.success(new CanvasHistoryReadResponse(
                service.canvasHistorySnapshot(snapshot), service.canvasDocument(snapshot.path("project")))));
    }

    @PostMapping("/{id}/history/{snapshotId}/restore")
    @Operation(operationId = "reference.canvas.history.restore", summary = "Restore a typed canvas history snapshot using CAS")
    @ApiResponse(responseCode = "200", description = "Restored canvas summary", content = @Content(schema = @Schema(implementation = CanvasWriteResponse.class)))
    public ResponseEntity<ApiEnvelope<CanvasWriteResponse>> restore(
            HttpServletRequest request,
            @PathVariable String id,
            @PathVariable String snapshotId,
            @Valid @RequestBody CanvasRestoreRequest body) {
        String userId = service.requireUser(request);
        if (body == null || body.revision() == null) {
            throw new ApiException(428, 428, "precondition_required", "revision is required");
        }
        ObjectNode result = store.update(root -> {
            ObjectNode canvas = ownedCanvas(root, id, userId);
            if (canvas.path("revision").asInt() != body.revision()) {
                throw new ApiException(409, 409, "conflict", "revision conflict");
            }
            JsonNode snapshot = findSnapshot(canvas, snapshotId);
            if (referencesMissingResource(root, snapshot.path("project"), userId)) {
                throw new ApiException(409, 409, "canvas_history_resources_missing", "canvas history references a missing resource");
            }
            addHistorySnapshot(canvas, snapshotSummary(canvas.path("project"), canvas.path("revision").asInt(), "before_restore"), true);
            ObjectNode project = ReferenceService.copyObject(snapshot.path("project"));
            int nextRevision = canvas.path("revision").asInt() + 1;
            project.put("revision", nextRevision).put("updatedAt", Instant.now().toString());
            addSummaryFields(project);
            canvas.set("project", project);
            canvas.put("revision", nextRevision);
            return project;
        });
        return ResponseEntity.ok(ApiEnvelope.success(new CanvasWriteResponse(service.canvasSummary(result))));
    }

    @DeleteMapping("/{id}")
    @Operation(operationId = "reference.canvas.delete", summary = "Soft-delete a canvas owned by the current user")
    @ApiResponse(responseCode = "200", description = "Canvas identity", content = @Content(schema = @Schema(implementation = IdentityDto.class)))
    public ResponseEntity<ApiEnvelope<IdentityDto>> delete(HttpServletRequest request, @PathVariable String id) {
        String userId = service.requireUser(request);
        store.update(root -> {
            JsonNode node = root.with("canvases").path(id);
            if (node.isObject() && userId.equals(ReferenceService.text(node, "ownerId"))) {
                ((ObjectNode) node).put("deleted", true);
            }
            return null;
        });
        return ResponseEntity.ok(ApiEnvelope.success(new IdentityDto(id)));
    }

    private JsonNode ownedCanvas(String id, String userId) {
        return ownedCanvas(store.snapshot(), id, userId);
    }

    private static ObjectNode ownedCanvas(ObjectNode root, String id, String userId) {
        JsonNode node = root.with("canvases").path(id);
        if (!node.isObject() || !userId.equals(ReferenceService.text(node, "ownerId")) || ReferenceService.bool(node, "deleted")) {
            throw new ApiException(404, 404, "not_found", "canvas not found");
        }
        return (ObjectNode) node;
    }

    private static ObjectNode upsert(ObjectNode root, String userId, String id, ObjectNode input, boolean repair) {
        JsonNode existingNode = root.with("canvases").path(id);
        boolean inputMissing = referencesMissingResource(root, input, userId);
        if (!inputMissing && referencesMissingAsset(root, input, userId)) {
            throw new ApiException(400, 400, "bad_request", "canvas media is not synchronized to the asset library");
        }
        boolean existingMissing = existingNode.isObject()
                && userId.equals(ReferenceService.text(existingNode, "ownerId"))
                && referencesMissingResource(root, existingNode.path("project"), userId);
        if (inputMissing) {
            String reason = existingMissing ? "canvas_history_resources_missing" : "missing_resource";
            String message = existingMissing
                    ? "canvas history references a missing resource"
                    : "canvas references a missing resource";
            throw new ApiException(409, 409, reason, message);
        }
        if (existingMissing && !repair) {
            throw new ApiException(409, 409, "canvas_history_resources_missing", "canvas history references a missing resource");
        }
        if (!existingNode.isObject()) {
            ObjectNode project = ReferenceService.copyObject(input);
            String now = Instant.now().toString();
            project.put("revision", 1).put("createdAt", now).put("updatedAt", now);
            addSummaryFields(project);
            ObjectNode canvas = root.with("canvases").putObject(id);
            canvas.put("id", id).put("ownerId", userId).put("revision", 1).put("deleted", false);
            canvas.set("project", project);
            canvas.set("history", canvas.objectNode().arrayNode());
            canvas.set("share", canvas.objectNode().objectNode());
            return project;
        }
        ObjectNode existing = (ObjectNode) existingNode;
        if (!userId.equals(ReferenceService.text(existing, "ownerId"))) {
            throw new ApiException(409, 409, "conflict", "canvas belongs to another user");
        }
        int currentRevision = existing.path("revision").asInt();
        JsonNode revisionNode = input.get("revision");
        if (revisionNode == null || !revisionNode.isNumber()) {
            throw new ApiException(428, 428, "precondition_required", "revision is required");
        }
        if (revisionNode.asLong() < 0 || revisionNode.asLong() >= 9007199254740991L) {
            throw new ApiException(400, 400, "bad_request", "revision is outside the safe integer range");
        }
        if (revisionNode.asInt() != currentRevision) {
            throw new ApiException(409, 409, "conflict", "revision conflict");
        }
        addHistorySnapshot(existing, snapshotSummary(existing.path("project"), currentRevision, repair ? "before_resource_repair" : "before_update"), repair);
        ObjectNode project = ReferenceService.copyObject(input);
        int nextRevision = currentRevision + 1;
        project.put("revision", nextRevision)
                .put("createdAt", ReferenceService.nonBlank(ReferenceService.text(existing.path("project"), "createdAt"), Instant.now().toString()))
                .put("updatedAt", Instant.now().toString());
        addSummaryFields(project);
        existing.set("project", project);
        existing.put("revision", nextRevision).put("deleted", false);
        return project;
    }

    private static boolean referencesMissingResource(ObjectNode root, JsonNode project, String userId) {
        Set<String> resourceIds = new HashSet<>();
        collectResourceIds(project, resourceIds);
        for (String resourceId : resourceIds) {
            JsonNode resource = root.with("resources").path(resourceId);
            if (!resource.isObject()
                    || !userId.equals(ReferenceService.text(resource, "ownerId"))
                    || !"ready".equals(ReferenceService.text(resource, "status"))) {
                return true;
            }
        }
        return false;
    }

    private static void collectResourceIds(JsonNode node, Set<String> resourceIds) {
        if (node == null) return;
        if (node.isTextual() && node.textValue().startsWith("resource:")) {
            resourceIds.add(node.textValue().substring("resource:".length()));
            return;
        }
        if (node.isObject()) {
            node.elements().forEachRemaining(child -> collectResourceIds(child, resourceIds));
        } else if (node.isArray()) {
            node.elements().forEachRemaining(child -> collectResourceIds(child, resourceIds));
        }
    }

    private static boolean referencesMissingAsset(ObjectNode root, JsonNode project, String userId) {
        JsonNode nodes = project == null ? null : project.path("nodes");
        if (nodes == null || !nodes.isArray()) {
            return false;
        }
        for (JsonNode node : nodes) {
            String type = ReferenceService.text(node, "type");
            if (!List.of("image", "video", "audio").contains(type)) {
                continue;
            }
            JsonNode metadata = node.path("metadata");
            String resourceId = resourceIdFromStorageKey(ReferenceService.text(metadata, "storageKey"));
            if (resourceId == null) {
                continue;
            }
            String assetId = ReferenceService.text(metadata, "assetId");
            JsonNode asset = root.with("assets").path(assetId == null ? "" : assetId);
            if (!asset.isObject() || !userId.equals(ReferenceService.text(asset, "ownerId"))
                    || !containsResourceReference(asset.path("payload"), resourceId)) {
                return true;
            }
        }
        return false;
    }

    private static String resourceIdFromStorageKey(String storageKey) {
        if (storageKey == null || !storageKey.startsWith("resource:")) {
            return null;
        }
        String resourceId = storageKey.substring("resource:".length()).trim();
        return resourceId.isEmpty() ? null : resourceId;
    }

    private static boolean containsResourceReference(JsonNode node, String resourceId) {
        if (node == null) {
            return false;
        }
        if (node.isTextual() && ("resource:" + resourceId).equals(node.textValue())) {
            return true;
        }
        if (node.isObject() || node.isArray()) {
            for (JsonNode child : node) {
                if (containsResourceReference(child, resourceId)) {
                    return true;
                }
            }
        }
        return false;
    }

    private static ObjectNode snapshotSummary(JsonNode project, int revision, String reason) {
        ObjectNode payload = ReferenceService.copyObject(project);
        ObjectNode result = payload.objectNode();
        result.put("id", "snapshot-" + java.util.UUID.randomUUID())
                .put("canvasId", ReferenceService.text(project, "id"))
                .put("revision", revision)
                .put("title", ReferenceService.nonBlank(ReferenceService.text(project, "title"), "Untitled"))
                .put("nodeCount", project.path("nodes").isArray() ? project.path("nodes").size() : 0)
                .put("connectionCount", project.path("connections").isArray() ? project.path("connections").size() : 0)
                .put("payloadBytes", project.toString().getBytes(java.nio.charset.StandardCharsets.UTF_8).length)
                .put("reason", reason)
                .put("contentUpdatedAt", ReferenceService.nonBlank(ReferenceService.text(project, "updatedAt"), Instant.now().toString()))
                .put("createdAt", Instant.now().toString());
        result.set("project", payload);
        return result;
    }

    private static void addSummaryFields(ObjectNode project) {
        int nodeCount = project.path("nodes").isArray() ? project.path("nodes").size() : 0;
        project.put("nodeCount", nodeCount);
        ArrayNode previewNodes = project.putArray("previewNodes");
        if (project.path("nodes").isArray()) {
            int limit = Math.min(3, project.path("nodes").size());
            for (int index = 0; index < limit; index++) {
                previewNodes.add(project.path("nodes").get(index));
            }
        }
    }

    private static void addHistorySnapshot(ObjectNode canvas, ObjectNode snapshot, boolean force) {
        ArrayNode history = canvas.withArray("history");
        if (!force && !history.isEmpty() && "before_update".equals(ReferenceService.text(history.get(0), "reason"))) {
            return;
        }
        history.insert(0, snapshot);
        while (history.size() > 20) {
            history.remove(history.size() - 1);
        }
    }

    private static JsonNode findSnapshot(JsonNode canvas, String snapshotId) {
        for (JsonNode snapshot : canvas.path("history")) {
            if (snapshotId.equals(ReferenceService.text(snapshot, "id"))) {
                return snapshot;
            }
        }
        throw new ApiException(404, 404, "not_found", "history snapshot not found");
    }

    private static int parsePage(String raw, int fallback) {
        if (raw == null) {
            return fallback;
        }
        try {
            return Integer.parseInt(raw);
        } catch (NumberFormatException exception) {
            throw new ApiException(400, 400, "bad_request", "pagination values must be integers");
        }
    }

    private static HttpHeaders canvasHeaders(String etag) {
        HttpHeaders headers = new HttpHeaders();
        headers.setETag(etag);
        headers.setCacheControl("private, no-cache");
        headers.add(HttpHeaders.VARY, "Accept-Encoding");
        return headers;
    }
}
