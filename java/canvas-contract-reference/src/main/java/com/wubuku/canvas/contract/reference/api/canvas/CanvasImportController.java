package com.wubuku.canvas.contract.reference.api.canvas;

import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasImportDtos.ImportConnectionDto;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasImportDtos.ImportIssueDto;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasImportDtos.ImportNodeDto;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasImportDtos.ImportResponseDto;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasImportDtos.LibTvImportRequest;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasImportDtos.TapNowImportRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ApiEnvelope;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasImportDtos.ImportWarningDto;
import com.wubuku.canvas.contract.reference.contract.ApiException;
import com.wubuku.canvas.contract.reference.contract.ReferenceService;
import com.wubuku.canvas.contract.reference.contract.ReferenceStore;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.media.Content;
import io.swagger.v3.oas.annotations.media.Schema;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.tags.Tag;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.Valid;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import java.util.regex.Pattern;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/canvas-projects")
@Tag(name = "Canvas import reference", description = "Strongly typed deterministic import conversion endpoints.")
public class CanvasImportController {
    private static final Pattern LIBTV_UUID = Pattern.compile("[0-9a-fA-F]{32}");
    private final ReferenceService service;
    private final ReferenceStore store;

    public CanvasImportController(ReferenceService service, ReferenceStore store) {
        this.service = service;
        this.store = store;
    }

    @PostMapping("/{id}/import/libtv")
    @Operation(operationId = "reference.canvas.import.libtv", summary = "Convert a controlled LibTV project into typed canvas nodes")
    @ApiResponse(responseCode = "200", description = "Typed conversion result", content = @Content(schema = @Schema(implementation = ImportResponseDto.class)))
    @ApiResponse(responseCode = "400", description = "Invalid import identity or disabled provider")
    public ResponseEntity<ApiEnvelope<ImportResponseDto>> importLibTv(
            HttpServletRequest request,
            @PathVariable String id,
            @Valid @RequestBody LibTvImportRequest body) {
        String userId = service.requireUser(request);
        requireCanvas(userId, id);
        String identity = body == null || body.uuid() == null ? "" : body.uuid().trim();
        if (!LIBTV_UUID.matcher(identity).matches()) {
            throw new ApiException(400, 400, "bad_request", "LibTV project UUID is invalid");
        }
        JsonNodeSettings settings = libTvSettings();
        if (!settings.enabled || settings.token.isBlank()) {
            throw new ApiException(400, 400, "bad_request", "LibTV is not enabled");
        }
        String mode = libTvMode(identity);
        return convertedResponse("libtv", identity, mode);
    }

    @PostMapping("/{id}/import/tapnow")
    @Operation(operationId = "reference.canvas.import.tapnow", summary = "Convert a controlled TapNow share into typed canvas nodes")
    @ApiResponse(responseCode = "200", description = "Typed conversion result", content = @Content(schema = @Schema(implementation = ImportResponseDto.class)))
    @ApiResponse(responseCode = "400", description = "Invalid share identity")
    public ResponseEntity<ApiEnvelope<ImportResponseDto>> importTapNow(
            HttpServletRequest request,
            @PathVariable String id,
            @Valid @RequestBody TapNowImportRequest body) {
        String userId = service.requireUser(request);
        requireCanvas(userId, id);
        String identity = body == null || body.shareId() == null ? "" : body.shareId().trim();
        if (identity.isBlank() || identity.length() > 64 || !identity.matches("[A-Za-z0-9_-]+")) {
            throw new ApiException(400, 400, "bad_request", "TapNow share ID is invalid");
        }
        return convertedResponse("tapnow", identity, tapNowMode(identity));
    }

    private ResponseEntity<ApiEnvelope<ImportResponseDto>> convertedResponse(String provider, String identity, String mode) {
        if ("timeout".equals(mode)) {
            try {
                Thread.sleep(20_000L);
            } catch (InterruptedException interrupted) {
                Thread.currentThread().interrupt();
            }
        }
        if (isFailureMode(mode)) {
            throw new ApiException(500, 500, "internal", provider + " import fixture failed");
        }
        String batchId = provider + "-" + UUID.randomUUID();
        ImportResponseDto result = switch (mode) {
            case "rich-media" -> richResult(provider, identity, batchId);
            case "node-limit" -> limitResult(identity, batchId, 1000, 1, false);
            case "edge-limit" -> limitResult(identity, batchId, 2, 5000, false);
            case "edge-over-limit" -> limitResult(identity, batchId, 2, 5000, true);
            default -> basicResult(provider, identity, batchId);
        };
        return ResponseEntity.ok(ApiEnvelope.success(result));
    }

    private ImportResponseDto basicResult(String provider, String identity, String batchId) {
        List<ImportNodeDto> nodes = new ArrayList<>();
        if ("libtv".equals(provider)) {
            nodes.add(node(batchId, "fixture-image", "image", "Fixture image", "https://fixture.invalid/first.png", 12, 34, 640, 360,
                    "fixture prompt", "fixture model", null, null, null, null, null, 0, 0, 0, null, "success", null,
                    metadata(provider, identity, "fixture-image", batchId, null)));
            nodes.add(node(batchId, "fixture-video", "video", "Fixture placeholder", "", 56, 78, 480, 300,
                    null, null, null, null, null, null, null, 0, 0, 0, null, "error", "fixture upstream failure",
                    metadata(provider, identity, "fixture-video", batchId, null)));
            return response(batchId, identity, null, "Contract LibTV fixture", nodes, List.of(connection(batchId, nodes.get(0), nodes.get(1), null, null)),
                    List.of(issue("fixture-unsupported", "fixture-unsupported", "unsupported node type")), List.of(), List.of(), 0, 0, 0, 0, 0);
        }
        nodes.add(node(batchId, "fixture-text-1", "text", "Fixture text 1", "contract fixture first text", 12, 34, 640, 360,
                null, null, null, null, null, null, null, 0, 0, 0, "text/plain", "success", null,
                metadata(provider, identity, "fixture-text-1", batchId, "text")));
        nodes.add(node(batchId, "fixture-text-2", "text", "Fixture text 2", "contract fixture second text", 56, 78, 640, 360,
                null, null, null, null, null, null, null, 0, 0, 0, "text/plain", "success", null,
                metadata(provider, identity, "fixture-text-2", batchId, "text")));
        return response(batchId, null, identity, "Contract TapNow fixture", nodes, List.of(connection(batchId, nodes.get(0), nodes.get(1), null, null)),
                List.of(issue("fixture-unsupported", "fixture-unsupported", "unsupported node type")), List.of(), List.of(), 0, 0, 0, 0, 0);
    }

    private ImportResponseDto richResult(String provider, String identity, String batchId) {
        List<ImportNodeDto> nodes = new ArrayList<>();
        if ("libtv".equals(provider)) {
            nodes.add(node(batchId, "fixture-image", "image", "fixture-image", "https://files.tapnow.media/contract/image.png", 12.5, -34.5, 640, 360,
                    "image prompt", "fixture-image-model", null, null, null, null, null, 3, 2, 0, "image/png", "success", null,
                    metadata(provider, identity, "fixture-image", batchId, "image")));
            nodes.add(node(batchId, "fixture-video", "video", "fixture-video", "https://files.tapnow.media/contract/video.mp4", 12.5, -34.5, 640, 360,
                    null, null, null, null, null, null, null, 16, 16, 40, "video/mp4", "success", null,
                    metadata(provider, identity, "fixture-video", batchId, "video")));
            nodes.add(node(batchId, "fixture-reused", "image", "fixture-reused", "https://files.tapnow.media/contract/image.png", 12.5, -34.5, 640, 360,
                    null, null, null, null, null, null, null, 0, 0, 0, "image/png", "success", null,
                    metadata(provider, identity, "fixture-reused", batchId, "image")));
            ObjectNode styleMetadata = metadata(provider, identity, "fixture-style", batchId, "material-style");
            styleMetadata.put("styleAssetUuid", "fixture-style-asset").put("styleVersionUuid", "fixture-style-version").put("styleName", "Fixture style");
            nodes.add(node(batchId, "fixture-style", "image", "fixture-style", "https://files.tapnow.media/contract/image.png", 12.5, -34.5, 640, 360,
                    null, null, null, null, null, null, null, 3, 2, 0, "image/png", "success", null, new com.wubuku.canvas.contract.reference.api.common.OpaqueJsonValue(styleMetadata)));
            nodes.add(placeholder(batchId, provider, identity, "fixture-failed", "controlled placeholder failure"));
            nodes.add(idle(batchId, provider, identity, "fixture-unsafe"));
            return response(batchId, identity, null, "Contract LibTV rich media", nodes,
                    List.of(connection(batchId, nodes.get(0), nodes.get(1), null, null), connection(batchId, nodes.get(1), nodes.get(3), null, null), connection(batchId, nodes.get(3), nodes.get(4), null, null)),
                    skipped(5), skippedConnections(3), warnings(3), 1, 1, 2, 1, 1);
        }
        nodes.add(node(batchId, "fixture-image", "image", "fixture-image", "https://files.tapnow.media/contract/image.png", 12.5, -34.5, 640, 360,
                "image prompt", "fixture-image-model", "3:2", "2k", null, null, null, 3, 2, 0, "image/png", "success", null,
                metadata(provider, identity, "101", batchId, "image")));
        nodes.add(node(batchId, "fixture-video", "video", "fixture-video", "https://files.tapnow.media/contract/video.mp4", 12.5, -34.5, 640, 360,
                null, null, null, null, "0.04", "720p", "false", 16, 16, 40, "video/mp4", "success", null,
                metadata(provider, identity, "102", batchId, "video")));
        nodes.add(node(batchId, "103", "audio", "audio 节点", "https://files.tapnow.media/contract/audio.wav", 12.5, -34.5, 640, 360,
                null, null, null, null, null, null, null, 0, 0, 10, "audio/wav", "success", null,
                metadata(provider, identity, "103", batchId, "audio")));
        nodes.add(node(batchId, "fixture-text", "text", "fixture-text", "fallback text prompt", 12.5, -34.5, 640, 360,
                "fallback text prompt", null, null, null, null, null, null, 0, 0, 0, "text/plain", "success", null,
                metadata(provider, identity, "fixture-text", batchId, "text")));
        nodes.add(placeholder(batchId, provider, identity, "fixture-failed", "controlled placeholder failure"));
        nodes.add(idle(batchId, provider, identity, "fixture-unsafe"));
        return response(batchId, null, identity, "Contract TapNow rich media", nodes,
                List.of(connection(batchId, nodes.get(0), nodes.get(1), "output", "image"), connection(batchId, nodes.get(1), nodes.get(2), null, "explicit-input"), connection(batchId, nodes.get(3), nodes.get(4), null, null)),
                skipped(5), skippedConnections(3), warnings(2), 1, 1, 2, 0, 0);
    }

    private ImportResponseDto limitResult(String identity, String batchId, int nodeCount, int connectionCount, boolean warning) {
        List<ImportNodeDto> nodes = new ArrayList<>(nodeCount);
        for (int index = 0; index < nodeCount; index++) {
            nodes.add(node(batchId, "boundary-" + index, "text", "boundary-" + index, "controlled boundary text", 12.5, -34.5, 640, 360,
                    null, null, null, null, null, null, null, 0, 0, 0, "text/plain", "success", null,
                    metadata("tapnow", identity, "boundary-" + index, batchId, "text")));
        }
        List<ImportConnectionDto> connections = new ArrayList<>(connectionCount);
        for (int index = 0; index < connectionCount; index++) {
            connections.add(new ImportConnectionDto("tapnow-" + batchId + "-boundary-edge-" + index, nodes.get(0).id(), nodes.get(nodes.size() - 1).id(), null, null));
        }
        return response(batchId, null, identity, "Contract TapNow conversion limits", nodes, connections, List.of(), List.of(), warning ? warnings(1) : List.of(), 0, 0, 0, 0, 0);
    }

    private ImportResponseDto response(String batchId, String projectUuid, String shareId, String projectName, List<ImportNodeDto> nodes,
                                       List<ImportConnectionDto> connections, List<ImportIssueDto> skippedNodes,
                                       List<ImportIssueDto> skippedConnections, List<ImportWarningDto> warnings,
                                       int multiResult, int reusedFailed, int placeholders, int convertedSpecial, int stale) {
        return new ImportResponseDto(batchId, Instant.now().toString(), projectUuid, shareId, projectName, nodes, connections,
                nodes.size(), connections.size(), skippedNodes, skippedConnections, warnings, multiResult, reusedFailed, placeholders, convertedSpecial, stale);
    }

    private ImportNodeDto node(String batchId, String sourceId, String type, String title, String content, double x, double y, double width, double height,
                               String prompt, String model, String size, String quality, String seconds, String vquality, String generateAudio,
                               int naturalWidth, int naturalHeight, long durationMs, String mimeType, String status, String errorDetails, ObjectNode metadata) {
        return node(batchId, sourceId, type, title, content, x, y, width, height, prompt, model, size, quality, seconds, vquality, generateAudio,
                naturalWidth, naturalHeight, durationMs, mimeType, status, errorDetails, new com.wubuku.canvas.contract.reference.api.common.OpaqueJsonValue(metadata));
    }

    private ImportNodeDto node(String batchId, String sourceId, String type, String title, String content, double x, double y, double width, double height,
                               String prompt, String model, String size, String quality, String seconds, String vquality, String generateAudio,
                               int naturalWidth, int naturalHeight, long durationMs, String mimeType, String status, String errorDetails,
                               com.wubuku.canvas.contract.reference.api.common.OpaqueJsonValue metadata) {
        return new ImportNodeDto(type + "-" + batchId + "-" + sourceId, type, title, content, x, y, width, height, prompt, model, size, quality,
                seconds, vquality, generateAudio, naturalWidth, naturalHeight, durationMs, mimeType, status, errorDetails, metadata);
    }

    private ImportNodeDto placeholder(String batchId, String provider, String identity, String sourceId, String error) {
        return node(batchId, sourceId, "video", sourceId, "", 12.5, -34.5, 480, 300, null, null, null, null, null, null, null,
                0, 0, 0, "video/mp4", "error", error, metadata(provider, identity, sourceId, batchId, "video"));
    }

    private ImportNodeDto idle(String batchId, String provider, String identity, String sourceId) {
        return node(batchId, sourceId, "image", sourceId, "", 12.5, -34.5, 640, 360, null, null, null, null, null, null, null,
                0, 0, 0, "image/*", "idle", null, metadata(provider, identity, sourceId, batchId, "image"));
    }

    private ImportConnectionDto connection(String batchId, ImportNodeDto from, ImportNodeDto to, String fromHandle, String toHandle) {
        return new ImportConnectionDto("connection-" + batchId + "-" + from.id() + "-" + to.id(), from.id(), to.id(), fromHandle, toHandle);
    }

    private ObjectNode metadata(String provider, String identity, String sourceId, String batchId, String sourceType) {
        ObjectNode result = JsonNodeFactory.instance.objectNode().put("provider", provider).put("batchId", batchId);
        if ("libtv".equals(provider)) {
            result.put("projectUuid", identity).put("nodeKey", sourceId);
        } else {
            result.put("shareId", identity).put("nodeId", sourceId);
        }
        if (sourceType != null) result.put("sourceType", sourceType);
        return result;
    }

    private List<ImportIssueDto> skipped(int count) {
        List<ImportIssueDto> issues = new ArrayList<>(count);
        for (int index = 0; index < count; index++) issues.add(issue("skipped-" + index, "skipped-" + index, "controlled fixture node was not importable"));
        return issues;
    }

    private List<ImportIssueDto> skippedConnections(int count) {
        List<ImportIssueDto> issues = new ArrayList<>(count);
        for (int index = 0; index < count; index++) issues.add(issue("connection-" + index, null, "controlled fixture connection was not importable"));
        return issues;
    }

    private List<ImportWarningDto> warnings(int count) {
        List<ImportWarningDto> result = new ArrayList<>(count);
        for (int index = 0; index < count; index++) result.add(new ImportWarningDto("warning-" + index, "controlled conversion warning"));
        return result;
    }

    private ImportIssueDto issue(String id, String name, String reason) {
        return new ImportIssueDto(id, name, reason);
    }

    private void requireCanvas(String userId, String id) {
        var canvas = store.snapshot().path("canvases").path(id);
        if (!canvas.isObject() || !userId.equals(ReferenceService.text(canvas, "ownerId")) || ReferenceService.bool(canvas, "deleted")) {
            throw new ApiException(500, 500, "internal", "canvas import target is unavailable");
        }
    }

    private JsonNodeSettings libTvSettings() {
        var settings = store.snapshot().path("settings").path("libtv");
        return new JsonNodeSettings(settings.path("enabled").asBoolean(false), settings.path("token").asText(""));
    }

    private String libTvMode(String identity) {
        int index = Integer.parseInt(identity.substring(identity.length() - 2), 16);
        return switch (index) {
            case 0 -> "success";
            case 1 -> "http-error";
            case 2 -> "business-error";
            case 3 -> "invalid-json";
            case 4 -> "oversized";
            case 5 -> "redirect";
            case 6 -> "truncated";
            case 7 -> "no-copy";
            case 8 -> "empty";
            case 9 -> "timeout";
            case 10 -> "rich-media";
            case 15 -> "no-read";
            default -> "failure";
        };
    }

    private String tapNowMode(String identity) {
        if (!identity.startsWith("contract_")) return "failure";
        return identity.substring("contract_".length()).replace('_', '-');
    }

    private boolean isFailureMode(String mode) {
        return switch (mode) {
            case "success", "rich-media", "node-limit", "edge-limit", "edge-over-limit" -> false;
            default -> true;
        };
    }

    private record JsonNodeSettings(boolean enabled, String token) {
    }
}
