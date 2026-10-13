package com.wubuku.canvas.contract.reference.api.resource;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.fasterxml.jackson.databind.node.JsonNodeFactory;
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
import java.nio.charset.StandardCharsets;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.time.Instant;
import java.time.ZonedDateTime;
import java.time.format.DateTimeFormatter;
import java.time.temporal.ChronoUnit;
import java.util.ArrayList;
import java.util.Base64;
import java.util.Comparator;
import java.util.List;
import javax.imageio.ImageIO;
import javax.imageio.ImageReader;
import javax.imageio.stream.ImageInputStream;
import java.util.Iterator;
import java.util.Locale;
import java.util.Map;
import java.util.HashMap;
import java.util.concurrent.ConcurrentHashMap;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpRange;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PatchMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RequestPart;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.multipart.MultipartFile;

@RestController
@RequestMapping("/api")
@Tag(name = "Reference resources", description = "Typed mock resource metadata and deterministic byte delivery.")
public class ResourceController {
    private static final String CONTRACT_IMAGE_PNG = "iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAYAAACddGYaAAAAF0lEQVR4nGJiQAJgjkPDgf8gGhAAAP//D2MChCRazqsAAAAASUVORK5CYII=";
    private static final String CONTRACT_VIDEO_MP4 = "AAAAHGZ0eXBpc29tAAACAGlzb21pc28ybXA0MQAAAAhmcmVlAAAAGm1kYXQAAAGzABAHAAABthBgsYKZt+8AAAMLbW9vdgAAAGxtdmhkAAAAAAAAAAAAAAAAAAAD6AAAACgAAQAAAQAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAlp0cmFrAAAAXHRraGQAAAADAAAAAAAAAAAAAAABAAAAAAAAACgAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAABAAAAAABAAAAAQAAAAAAAkZ0cwAAABxlbHN0AAAAAAAAAAEAAAAoAAAAAAABAAAAAAHSbWRpYQAAACBtZGhkAAAAAAAAAAAAAAAAAAAyAAAAAgBVxAAAAAAALWhkbHIAAAAAAAAAAHZpZGUAAAAAAAAAAAAAAABWaWRlb0hhbmRsZXIAAAABfW1pbmYAAAAUdm1oZAAAAAEAAAAAAAAAAAAAACRkaW5mAAAAHGRyZWYAAAAAAAAAAQAAAAx1cmwgAAAAAQAAAT1zdGJsAAAA2XN0c2QAAAAAAAAAAQAAAMltcDR2AAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAABAAEABIAAAASAAAAAAAAAABCkxhdmMgbXBlZzQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAGP//AAAAT2VzZHMAAAAAA4CAgD4AAQAEgICAMCARAAAAAAMNQAAADhAFgICAHgAAAbABAAABtYkTAAABAAAAASAAxI2IAM0AhAIUYwaAgIABAgAAABBwYXNwAAAAAQAAAAEAAAAUYnRydAAAAAAAAw1AAAAOEAAAABhzdHRzAAAAAAAAAAEAAAABAAACAAAAABxzdHNjAAAAAAAAAAEAAAABAAAAAQAAAAEAAAAUc3RzegAAAAAAAAASAAAAAQAAABRzdGNvAAAAAAAAAAEAAAAsAAAAPXVkdGEAAAA1bWV0YQAAAAAAAAAhaGRscgAAAAAAAAAAbWRpcmFwcGwAAAAAAAAAAAAAAAAIaWxzdA==";
    private final ReferenceService service;
    private final ReferenceStore store;
    private final Map<String, Map<Integer, byte[]>> activeUploadChunks = new ConcurrentHashMap<>();

    public ResourceController(ReferenceService service, ReferenceStore store) {
        this.service = service;
        this.store = store;
    }

    @PostMapping(value = "/resources", consumes = MediaType.MULTIPART_FORM_DATA_VALUE)
    @Operation(operationId = "reference.resource.create", summary = "Create a ready mock resource from multipart bytes")
    @ApiResponse(responseCode = "200", description = "Created resource", content = @Content(schema = @Schema(implementation = ResourceResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceResponse>> create(
            HttpServletRequest request,
            @RequestParam(defaultValue = "file") String kind,
            @RequestPart("file") MultipartFile file,
            @RequestHeader(name = "X-Idempotency-Key", required = false) String idempotencyKey) {
        String userId = service.requireUser(request);
        if (file == null || file.isEmpty() || file.getOriginalFilename() == null || file.getOriginalFilename().isBlank()) {
            throw new ApiException(400, 400, "bad_request", "file is required");
        }
        try {
            byte[] bytes = file.getBytes();
            String requestedKind = kind == null || kind.isBlank() ? "file" : kind;
            String mimeType = detectMime(file.getContentType(), file.getOriginalFilename(), bytes);
            String key = idempotencyKey == null ? "" : idempotencyKey.trim();
            ResourceDto createdResource = store.update(root -> {
                ObjectNode resourceUploads = root.with("ledger").with("resourceUploads");
                if (!key.isBlank()) {
                    JsonNode previous = resourceUploads.path(userId + "\u0000" + key);
                    if (previous.isObject()) {
                        JsonNode existing = root.with("resources").path(previous.path("resourceId").asText());
                        if (existing.isObject()
                                && bytes.length == existing.path("size").asLong()
                                && requestedKind.equals(existing.path("kind").asText())
                                && mimeType.equals(existing.path("mimeType").asText())) {
                            return toDto(existing);
                        }
                        throw new ApiException(409, 409, "resource_upload_conflict", "idempotent upload identity changed");
                    }
                }
                enforceQuota(root, userId, bytes.length);
                String id = store.newId("resource");
                ObjectNode remote = uploadToConfiguredObjectStore(id, bytes);
                enforceRate(root, userId, "upload");
                ObjectNode resourceNode = root.with("resources").putObject(id);
                resourceNode.put("id", id)
                        .put("ownerId", userId)
                        .put("fileName", file.getOriginalFilename())
                        .put("kind", requestedKind)
                        .put("mimeType", mimeType)
                        .put("size", bytes.length)
                        .put("width", 1)
                        .put("height", 1)
                        .put("durationMs", 0)
                        .put("status", "ready")
                        .put("revision", "1")
                        .put("provider", remote.path("provider").asText("local"))
                        .put("bytes", Base64.getEncoder().encodeToString(bytes))
                        .put("createdAt", Instant.now().toString())
                        .put("updatedAt", Instant.now().toString());
                if (remote.hasNonNull("remoteUrl")) {
                    resourceNode.put("remoteUrl", remote.path("remoteUrl").asText());
                }
                if (remote.hasNonNull("endpoint")) {
                    resourceNode.put("endpoint", remote.path("endpoint").asText());
                }
                if (remote.hasNonNull("bucket")) {
                    resourceNode.put("bucket", remote.path("bucket").asText());
                }
                if (remote.hasNonNull("objectKey")) {
                    resourceNode.put("objectKey", remote.path("objectKey").asText());
                }
                if (!key.isBlank()) {
                    resourceUploads.putObject(userId + "\u0000" + key)
                            .put("resourceId", id)
                            .put("size", bytes.length)
                            .put("kind", requestedKind)
                            .put("mimeType", mimeType);
                }
                return toDto(resourceNode);
            });
            return ResponseEntity.ok(ApiEnvelope.success(new ResourceResponse(createdResource)));
        } catch (ApiException exception) {
            throw exception;
        } catch (Exception exception) {
            throw new ApiException(400, 400, "bad_request", "resource bytes could not be read");
        }
    }

    @PostMapping("/resources/uploads")
    @Operation(operationId = "reference.resource.upload.create", summary = "Reserve a typed resumable resource upload")
    @ApiResponse(responseCode = "200", description = "Upload session", content = @Content(schema = @Schema(implementation = ResourceUploadSessionResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceUploadSessionDto>> createUpload(
            HttpServletRequest request,
            @Valid @RequestBody com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.UploadCreateRequest body) {
        String userId = service.requireUser(request);
        long size = body.size() == null ? 0 : body.size();
        if (size <= 0 || body.fileName() == null || body.fileName().isBlank() || body.fileName().length() > 255) {
            throw new ApiException(400, 400, "bad_request", "invalid upload declaration");
        }
        String kind = body.kind() == null || body.kind().isBlank() ? "file" : body.kind();
        String idempotencyKey = body.idempotencyKey() == null ? "" : body.idempotencyKey().trim();
        int chunkSize = 8 * 1024 * 1024;
        int chunkCount = Math.toIntExact((size + chunkSize - 1) / chunkSize);
        String uploadId = store.update(root -> {
            enforceAdmission(root, userId, size, "upload");
            String id = store.newId("upload");
            ObjectNode upload = root.with("uploads").putObject(id);
            upload.put("uploadId", id)
                    .put("ownerId", userId)
                    .put("fileName", body.fileName())
                    .put("kind", kind)
                    .put("size", size)
                    .put("chunkSize", chunkSize)
                    .put("chunkCount", chunkCount)
                    .put("status", "open")
                    .put("createdAt", Instant.now().toString())
                    .put("idempotencyKey", idempotencyKey);
            return id;
        });
        activeUploadChunks.put(uploadId, new ConcurrentHashMap<>());
        ResourceUploadSessionDto session = uploadSession(store.snapshot().path("uploads").path(uploadId));
        return ResponseEntity.ok(ApiEnvelope.success(session));
    }

    @PutMapping(value = "/resources/uploads/{uploadId}/chunks/{index}", consumes = MediaType.APPLICATION_OCTET_STREAM_VALUE)
    @Operation(operationId = "reference.resource.upload.chunk", summary = "Store one exact-sized upload chunk")
    @ApiResponse(responseCode = "200", description = "Chunk accepted", content = @Content(schema = @Schema(implementation = ResourceUploadChunkResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceUploadChunkResponse>> putChunk(
            HttpServletRequest request,
            @PathVariable String uploadId,
            @PathVariable int index,
            @RequestBody byte[] content) {
        String userId = service.requireUser(request);
        byte[] bytes = content == null ? new byte[0] : content;
        Map<Integer, byte[]> chunks = activeUploadChunks.get(uploadId);
        if (chunks == null) {
            throw new ApiException(404, 404, "not_found", "upload session not found");
        }
        ResourceUploadChunkResponse result = store.update(root -> {
            JsonNode upload = root.with("uploads").path(uploadId);
            if (!upload.isObject() || !userId.equals(ReferenceService.text(upload, "ownerId"))
                    || !("open".equals(ReferenceService.text(upload, "status"))
                    || "expired".equals(ReferenceService.text(upload, "status")))) {
                throw new ApiException(404, 404, "not_found", "upload session not found");
            }
            int chunkCount = upload.path("chunkCount").asInt();
            long declaredSize = upload.path("size").asLong();
            int chunkSize = upload.path("chunkSize").asInt();
            if (index < 0 || index >= chunkCount) {
                throw new ApiException(400, 400, "bad_request", "chunk index is outside the upload session");
            }
            long expected = Math.min(chunkSize, declaredSize - (long) index * chunkSize);
            if (bytes.length != expected) {
                chunks.remove(index);
                throw new ApiException(400, 400, "bad_request", "chunk byte count does not match the declared boundary");
            }
            chunks.put(index, bytes.clone());
            ((ObjectNode) upload).with("rejectedChunks").remove(Integer.toString(index));
            return new ResourceUploadChunkResponse(index, bytes.length);
        });
        return ResponseEntity.ok(ApiEnvelope.success(result));
    }

    @PostMapping("/resources/uploads/{uploadId}/complete")
    @Operation(operationId = "reference.resource.upload.complete", summary = "Complete a reserved upload and publish a typed resource")
    @ApiResponse(responseCode = "200", description = "Ready resource", content = @Content(schema = @Schema(implementation = ResourceResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceResponse>> completeUpload(HttpServletRequest request, @PathVariable String uploadId) {
        String userId = service.requireUser(request);
        JsonNode uploadSnapshot = store.snapshot().with("uploads").path(uploadId);
        if (!uploadSnapshot.isObject() || !userId.equals(ReferenceService.text(uploadSnapshot, "ownerId"))) {
            throw new ApiException(404, 404, "not_found", "upload session not found");
        }
        Map<Integer, byte[]> chunks = activeUploadChunks.get(uploadId);
        if (chunks == null) {
            throw new ApiException(404, 404, "not_found", "upload session not found");
        }
        if ("expired".equals(ReferenceService.text(uploadSnapshot, "status"))) {
            store.update(root -> {
                JsonNode upload = root.with("uploads").path(uploadId);
                if (upload.isObject()) ((ObjectNode) upload).put("status", "abandoned");
                return null;
            });
            activeUploadChunks.remove(uploadId);
            throw new ApiException(400, 400, "bad_request", "upload reservation has expired");
        }
        if (!"open".equals(ReferenceService.text(uploadSnapshot, "status"))) {
            throw new ApiException(404, 404, "not_found", "upload session not found");
        }
        ResourceDto resource = store.update(root -> {
            ObjectNode upload = (ObjectNode) root.with("uploads").path(uploadId);
            if (!upload.isObject() || !userId.equals(ReferenceService.text(upload, "ownerId"))
                    || !"open".equals(ReferenceService.text(upload, "status"))) {
                throw new ApiException(404, 404, "not_found", "upload session not found");
            }
            if (upload.path("rejectedChunks").isObject() && upload.path("rejectedChunks").size() > 0) {
                throw new ApiException(400, 400, "bad_request", "upload contains a rejected chunk");
            }
            int chunkCount = upload.path("chunkCount").asInt();
            ByteArrayOutputStream output = new ByteArrayOutputStream(Math.toIntExact(upload.path("size").asLong()));
            for (int index = 0; index < chunkCount; index++) {
                byte[] chunk = chunks.get(index);
                if (chunk == null) {
                    throw new ApiException(400, 400, "bad_request", "upload is missing one or more chunks");
                }
                try {
                    output.write(chunk);
                } catch (IOException exception) {
                    throw new ApiException(400, 400, "bad_request", "upload chunk could not be assembled");
                }
            }
            byte[] bytes = output.toByteArray();
            if (bytes.length != upload.path("size").asLong()) {
                throw new ApiException(400, 400, "bad_request", "assembled upload size does not match declaration");
            }
            String idempotencyKey = upload.path("idempotencyKey").asText("").trim();
            ObjectNode uploads = root.with("ledger").with("resourceUploads");
            if (!idempotencyKey.isBlank()) {
                JsonNode previous = uploads.path(userId + "\u0000" + idempotencyKey);
                if (previous.isObject()) {
                    JsonNode existing = root.with("resources").path(previous.path("resourceId").asText());
                    if (existing.isObject()
                            && bytes.length == existing.path("size").asLong()
                            && detectedKind(upload.path("kind").asText(), bytes).equals(existing.path("kind").asText())
                            && detectMime(null, upload.path("fileName").asText(), bytes).equals(existing.path("mimeType").asText())) {
                        upload.put("status", "completed").put("resourceId", existing.path("id").asText());
                        return toDto(existing);
                    }
                    throw new ApiException(409, 409, "resource_upload_conflict", "idempotent upload identity changed");
                }
            }
            String id = store.newId("resource");
            ObjectNode resourceNode = root.with("resources").putObject(id);
            String now = Instant.now().toString();
            resourceNode.put("id", id)
                    .put("ownerId", userId)
                    .put("fileName", upload.path("fileName").asText())
                    .put("kind", detectedKind(upload.path("kind").asText(), bytes))
                    .put("mimeType", detectMime(null, upload.path("fileName").asText(), bytes))
                    .put("size", bytes.length)
                    .put("width", imageWidth(bytes))
                    .put("height", imageHeight(bytes))
                    .put("durationMs", 0)
                    .put("status", "ready")
                    .put("revision", "1")
                    .put("provider", "local")
                    .put("bytes", Base64.getEncoder().encodeToString(bytes))
                    .put("createdAt", now)
                    .put("updatedAt", now);
            upload.put("status", "completed").put("resourceId", id);
            if (!idempotencyKey.isBlank()) {
                uploads.putObject(userId + "\u0000" + idempotencyKey)
                        .put("resourceId", id)
                        .put("size", bytes.length)
                        .put("kind", detectedKind(upload.path("kind").asText(), bytes))
                        .put("mimeType", detectMime(null, upload.path("fileName").asText(), bytes));
            }
            return toDto(resourceNode);
        });
        activeUploadChunks.remove(uploadId);
        return ResponseEntity.ok(ApiEnvelope.success(new ResourceResponse(resource)));
    }

    @PostMapping("/resources/import")
    @Operation(operationId = "reference.resource.import", summary = "Import a mock resource from a controlled HTTP URL")
    @ApiResponse(responseCode = "200", description = "Imported resource", content = @Content(schema = @Schema(implementation = ResourceImportResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceImportResponse>> importResource(
        HttpServletRequest request,
            @Valid @RequestBody com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ResourceImportRequest body,
            @RequestHeader(name = "X-Idempotency-Key", required = false) String idempotencyKey) {
        String userId = service.requireUser(request);
        URI source = validateImportUri(body.url());
        store.update(root -> {
            enforceRate(root, userId, "import");
            return null;
        });
        byte[] bytes = downloadImport(source);
        String mimeType = detectMime(null, source.getPath(), bytes);
        String kind = detectedKind(body.kind(), bytes);
        String key = idempotencyKey == null ? "" : idempotencyKey.trim();
        ResourceDto resource = store.update(root -> {
            enforceQuota(root, userId, bytes.length);
            ObjectNode imports = root.with("ledger").with("resourceImports");
            if (!key.isBlank() && imports.has(userId + "\u0000" + key)) {
                JsonNode previous = imports.path(userId + "\u0000" + key);
                if (previous.path("size").asLong() != bytes.length
                        || !mimeType.equals(previous.path("mimeType").asText())
                        || !kind.equals(previous.path("kind").asText())) {
                    throw new ApiException(409, 409, "resource_import_conflict", "idempotent import identity changed");
                }
                JsonNode replay = root.with("resources").path(previous.path("resourceId").asText());
                if (replay.isObject()) return toDto(replay);
            }
            String id = store.newId("resource");
            ObjectNode resourceNode = root.with("resources").putObject(id);
            String now = Instant.now().toString();
            resourceNode.put("id", id)
                    .put("ownerId", userId)
                    .put("fileName", source.getPath() == null || source.getPath().isBlank() ? "imported-resource" : source.getPath().substring(source.getPath().lastIndexOf('/') + 1))
                    .put("kind", kind)
                    .put("mimeType", mimeType)
                    .put("size", bytes.length)
                    .put("width", imageWidth(bytes))
                    .put("height", imageHeight(bytes))
                    .put("durationMs", 0)
                    .put("status", "ready")
                    .put("revision", "1")
                    .put("provider", "local")
                    .put("bytes", Base64.getEncoder().encodeToString(bytes))
                    .put("createdAt", now)
                    .put("updatedAt", now);
            if (!key.isBlank()) {
                imports.putObject(userId + "\u0000" + key)
                        .put("resourceId", id)
                        .put("size", bytes.length)
                        .put("mimeType", mimeType)
                        .put("kind", kind);
            }
            return toDto(resourceNode);
        });
        return ResponseEntity.ok(ApiEnvelope.success(new ResourceImportResponse(resource)));
    }

    @GetMapping("/resources")
    @Operation(operationId = "reference.resource.list", summary = "List typed resources owned by the current user")
    @ApiResponse(responseCode = "200", description = "Resource list", content = @Content(schema = @Schema(implementation = ResourceListResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceListResponse>> list(HttpServletRequest request) {
        String userId = service.requireUser(request);
        List<ResourceDto> resources = new ArrayList<>();
        for (JsonNode resource : store.snapshot().with("resources")) {
            if (userId.equals(ReferenceService.text(resource, "ownerId")) && !"deleted".equals(ReferenceService.text(resource, "status"))) {
                resources.add(toDto(resource));
            }
        }
        resources.sort(Comparator.comparing(ResourceDto::id));
        return ResponseEntity.ok(ApiEnvelope.success(new ResourceListResponse(resources)));
    }

    @GetMapping("/resources/storage-usage")
    @Operation(operationId = "reference.resource.usage", summary = "Read deterministic resource storage usage")
    @ApiResponse(responseCode = "200", description = "Storage usage", content = @Content(schema = @Schema(implementation = ResourceUsageResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceUsageResponse>> usage(HttpServletRequest request) {
        String userId = service.requireUser(request);
        long bytes = 0;
        int count = 0;
        for (JsonNode resource : store.snapshot().with("resources")) {
            if (userId.equals(ReferenceService.text(resource, "ownerId")) && "ready".equals(ReferenceService.text(resource, "status"))) {
                bytes += resource.path("size").asLong();
                count++;
            }
        }
        long totalBytes = policyBytes(store.snapshot(), "storage", "storedBytes", 20L * 1024 * 1024 * 1024);
        return ResponseEntity.ok(ApiEnvelope.success(new ResourceUsageResponse(new ResourceUsageDto(bytes, totalBytes, count))));
    }

    @GetMapping("/resources/{id}")
    @Operation(operationId = "reference.resource.read", summary = "Read typed resource metadata")
    @ApiResponse(responseCode = "200", description = "Resource metadata", content = @Content(schema = @Schema(implementation = ResourceResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceResponse>> read(HttpServletRequest request, @PathVariable String id) {
        String userId = service.requireUser(request);
        JsonNode resource = ownedResource(id, userId);
        return ResponseEntity.ok(ApiEnvelope.success(new ResourceResponse(toDto(resource))));
    }

    @GetMapping("/resources/{id}/file")
    @Operation(operationId = "reference.resource.file", summary = "Read resource bytes with HTTP Range support")
    public ResponseEntity<byte[]> file(
            HttpServletRequest request,
            @PathVariable String id,
            @RequestParam(name = "public", required = false, defaultValue = "false") boolean publicRequest) {
        String userId = publicRequest ? null : service.requireUser(request);
        JsonNode resource = publicRequest ? availableResource(id) : ownedResource(id, userId);
        return bytesResponse(request, resource, "no-store");
    }

    @GetMapping(value = "/resources/{id}/file", params = "access=1")
    @Operation(operationId = "reference.resource.file-description", summary = "Describe a local resource file access capability")
    @ApiResponse(responseCode = "200", description = "Typed access description", content = @Content(schema = @Schema(implementation = ResourceAccessSingleResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceAccessSingleResponse>> description(
            HttpServletRequest request,
            @PathVariable String id,
            @RequestParam(defaultValue = "original") String variant) {
        String userId = service.requireUser(request);
        JsonNode resource = ownedResource(id, userId);
        HttpHeaders headers = noStoreHeaders();
        return ResponseEntity.ok().headers(headers).body(ApiEnvelope.success(new ResourceAccessSingleResponse(buildAccess(id, variant, resource, null))));
    }

    @PostMapping("/resources/access")
    @Operation(operationId = "reference.resource.access", summary = "Issue typed deterministic local access capabilities")
    @ApiResponse(responseCode = "200", description = "Access capabilities", content = @Content(schema = @Schema(implementation = ResourceAccessResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceAccessResponse>> access(
            HttpServletRequest request,
            @RequestBody List<ResourceAccessRequest> body) {
        String userId = service.requireUser(request);
        List<ResourceAccessItemDto> items = new ArrayList<>();
        for (ResourceAccessRequest input : body == null ? List.<ResourceAccessRequest>of() : body) {
            JsonNode resource = ownedResource(input.resourceId(), userId);
            String variant = input.variant() == null || input.variant().isBlank() ? "original" : input.variant();
            ResourceAccessDto access = buildAccess(input.resourceId(), variant, resource, null);
            items.add(new ResourceAccessItemDto(input.resourceId(), access));
        }
        return ResponseEntity.ok(ApiEnvelope.success(new ResourceAccessResponse(items)));
    }

    @GetMapping({"/public/resources/{id}/file", "/public/resources/{id}/file/{ignored}"})
    @Operation(operationId = "reference.resource.public-file", summary = "Read a signed mock resource capability")
    @ApiResponse(responseCode = "200", description = "Binary resource bytes", content = @Content(schema = @Schema(type = "string", format = "binary")))
    @ApiResponse(responseCode = "403", description = "Invalid or expired signature", content = @Content(schema = @Schema(implementation = com.wubuku.canvas.contract.reference.api.common.ErrorResponseDto.class)))
    public ResponseEntity<byte[]> publicFile(HttpServletRequest request, @PathVariable String id,
            @PathVariable(name = "ignored", required = false) String ignored) {
        String expires = request.getParameter("expires");
        String variant = request.getParameter("variant");
        if (variant == null || variant.isBlank()) variant = "original";
        if (expires == null || !isFutureEpoch(expires) || !validSignature(id, expires, variant, request.getParameter("signature"))) {
            byte[] body = errorEnvelope(403, "forbidden", "forbidden");
            HttpHeaders headers = new HttpHeaders();
            headers.setContentType(MediaType.APPLICATION_JSON);
            headers.setContentLength(body.length);
            return ResponseEntity.status(HttpStatus.FORBIDDEN)
                    .headers(headers)
                    .body(body);
        }
        JsonNode resource = availableResource(id);
        return bytesResponse(request, resource, "public, max-age=0, must-revalidate");
    }

    @GetMapping(value = {"/public/resources/{id}/file", "/public/resources/{id}/file/{ignored}"}, params = "access=1")
    @Operation(operationId = "reference.resource.public-file.access-description", summary = "Describe a signed public resource capability")
    @ApiResponse(responseCode = "200", description = "Typed public access description", content = @Content(schema = @Schema(implementation = ResourceAccessSingleResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceAccessSingleResponse>> publicDescription(
            HttpServletRequest request,
            @PathVariable String id,
            @PathVariable(name = "ignored", required = false) String ignored,
            @RequestParam(defaultValue = "original") String variant) {
        JsonNode resource = availableResource(id);
        String expires = request.getParameter("expires");
        if (expires == null || !isFutureEpoch(expires) || !validSignature(id, expires, variant, request.getParameter("signature"))) {
            throw new ApiException(403, 403, "forbidden", "resource access is invalid or expired");
        }
        return ResponseEntity.ok().headers(noStoreHeaders()).body(ApiEnvelope.success(new ResourceAccessSingleResponse(
                new ResourceAccessDto(id, variant, "original", signedUrl(id, "original", Long.parseLong(expires)), "platform-local",
                        Instant.now().toString(), Instant.ofEpochSecond(Long.parseLong(expires)).toString(),
                        Instant.now().plusSeconds(30).toString(), resource.path("revision").asText("1")))));
    }

    @GetMapping(value = "/public/canvas-shares/{token}/resources/{resourceId}/file", params = "access=1")
    @Operation(operationId = "reference.canvas.share.resource-description", summary = "Describe access to a resource referenced by a public canvas share")
    @ApiResponse(responseCode = "200", description = "Typed share resource access", content = @Content(schema = @Schema(implementation = ResourceAccessSingleResponse.class)))
    public ResponseEntity<ApiEnvelope<ResourceAccessSingleResponse>> sharedDescription(
            @PathVariable String token,
            @PathVariable String resourceId,
            @RequestParam(defaultValue = "original") String variant) {
        JsonNode root = store.snapshot();
        JsonNode resource = sharedResource(root, token, resourceId);
        HttpHeaders headers = new HttpHeaders();
        headers.set("Referrer-Policy", "no-referrer");
        headers.setCacheControl("private, no-store");
        return ResponseEntity.ok().headers(headers).body(ApiEnvelope.success(new ResourceAccessSingleResponse(buildAccess(resourceId, variant, resource, sharedExpiry(root, token)))));
    }

    @GetMapping("/public/canvas-shares/{token}/resources/{resourceId}/file")
    @Operation(operationId = "reference.canvas.share.resource-read", summary = "Read a resource referenced by a public canvas share")
    public ResponseEntity<byte[]> sharedFile(
            HttpServletRequest request,
            @Parameter(description = "Opaque public canvas share token") @PathVariable String token,
            @Parameter(description = "Resource identity referenced by the live shared canvas") @PathVariable String resourceId) {
        JsonNode root = store.snapshot();
        JsonNode resource = root.with("resources").path(resourceId);
        if (!resource.isObject() || !"ready".equals(ReferenceService.text(resource, "status"))) {
            throw new ApiException(404, 404, "not_found", "resource not found");
        }
        boolean referenced = false;
        for (JsonNode canvas : root.with("canvases")) {
            JsonNode share = canvas.path("share");
            if (!ReferenceService.bool(canvas, "deleted")
                    && token.equals(ReferenceService.text(share, "token"))
                    && activeShare(share)
                    && canvas.path("project").toString().contains("resource:" + resourceId)) {
                referenced = true;
                break;
            }
        }
        if (!referenced) {
            throw new ApiException(404, 404, "not_found", "shared resource not found");
        }
        if ("s3".equals(ReferenceService.text(resource, "provider"))) {
            HttpHeaders headers = new HttpHeaders();
            headers.setLocation(URI.create(ReferenceService.text(resource, "remoteUrl")));
            headers.set("Referrer-Policy", "no-referrer");
            headers.set("X-Content-Type-Options", "nosniff");
            headers.setCacheControl("private, no-store");
            return ResponseEntity.status(HttpStatus.TEMPORARY_REDIRECT).headers(headers).build();
        }
        return bytesResponse(request, resource, "no-store");
    }

    @PutMapping("/assets/{id}")
    @Operation(operationId = "reference.legacy.asset.upsert", summary = "Store a mock legacy asset reference")
    public ResponseEntity<ApiEnvelope<LegacyAssetResponse>> asset(
            HttpServletRequest request,
            @PathVariable String id,
            @RequestBody ObjectNode body) {
        String userId = service.requireUser(request);
        JsonNode existing = store.snapshot().path("assets").path(id);
        if (existing.isObject() && !userId.equals(ReferenceService.text(existing, "ownerId"))) {
            throw new ApiException(500, 500, "internal", "asset owner mismatch");
        }
        validateAssetFolder(userId, body);
        validateAssetResourceReplacement(id, userId, body);
        JsonNode input = body != null && body.path("asset").isObject() ? body.path("asset") : body;
        String now = Instant.now().toString();
        store.update(root -> {
            ObjectNode asset = root.with("assets").putObject(id);
            asset.put("id", id)
                    .put("ownerId", userId)
                    .put("title", input == null ? "" : ReferenceService.nonBlank(ReferenceService.text(input, "title"), id))
                    .put("kind", input == null ? "file" : ReferenceService.nonBlank(ReferenceService.text(input, "kind"), "file"))
                    .put("createdAt", now)
                    .put("updatedAt", now)
                    .set("payload", body == null ? root.objectNode() : body.deepCopy());
            String folderId = input == null ? null : ReferenceService.text(input, "folderId");
            if (folderId != null && !folderId.isBlank()) asset.put("folderId", folderId);
            return null;
        });
        return ResponseEntity.ok(ApiEnvelope.success(new LegacyAssetResponse(
                new LegacyAssetDto(id,
                        ReferenceService.text(store.snapshot().path("assets").path(id), "kind"),
                        ReferenceService.text(store.snapshot().path("assets").path(id), "title"),
                        ReferenceService.text(store.snapshot().path("assets").path(id), "createdAt"),
                ReferenceService.text(store.snapshot().path("assets").path(id), "updatedAt")))));
    }

    @GetMapping("/assets")
    @Operation(operationId = "reference.legacy.asset.list", summary = "List mock legacy assets owned by the current user")
    public ResponseEntity<ApiEnvelope<ObjectNode>> listAssets(
            HttpServletRequest request,
            @RequestParam(required = false) String folderId) {
        String userId = service.requireUser(request);
        ObjectNode data = JsonNodeFactory.instance.objectNode();
        var assets = data.putArray("assets");
        List<JsonNode> owned = new ArrayList<>();
        for (JsonNode asset : store.snapshot().with("assets")) {
            if (!userId.equals(ReferenceService.text(asset, "ownerId"))) continue;
            if (folderId != null && !folderId.equals(ReferenceService.text(asset, "folderId"))) continue;
            owned.add(asset);
        }
        owned.sort(Comparator.comparing(node -> ReferenceService.text(node, "id")));
        owned.forEach(asset -> assets.add(assetPayload(asset)));
        data.put("page", 1).put("pageSize", 20).put("total", assets.size()).put("hasMore", false);
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @PostMapping("/assets/batch")
    @Operation(operationId = "reference.legacy.asset.batch", summary = "Read a batch of owned mock legacy assets")
    public ResponseEntity<ApiEnvelope<ObjectNode>> batchAssets(HttpServletRequest request, @RequestBody ObjectNode body) {
        String userId = service.requireUser(request);
        ObjectNode data = JsonNodeFactory.instance.objectNode();
        var assets = data.putArray("assets");
        JsonNode rawIds = body == null ? null : body.path("ids");
        if (rawIds != null && rawIds.isArray()) {
            for (JsonNode rawId : rawIds) {
                String id = rawId.asText();
                JsonNode asset = store.snapshot().path("assets").path(id);
                if (asset.isObject() && userId.equals(ReferenceService.text(asset, "ownerId"))) assets.add(assetPayload(asset));
            }
        }
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @PatchMapping("/assets/folder")
    @Operation(operationId = "reference.legacy.asset.folder.assign", summary = "Assign owned mock assets to an owned folder")
    public ResponseEntity<ApiEnvelope<ObjectNode>> assignFolder(HttpServletRequest request, @RequestBody ObjectNode body) {
        String userId = service.requireUser(request);
        String folderId = ReferenceService.text(body, "folderId");
        JsonNode folder = store.snapshot().path("folders").path(folderId == null ? "" : folderId);
        if (folderId == null || folderId.isBlank() || !folder.isObject() || !userId.equals(ReferenceService.text(folder, "ownerId"))) {
            throw new ApiException(400, 400, "bad_request", "folder is not owned by the current user");
        }
        List<String> ids = new ArrayList<>();
        JsonNode rawIds = body == null ? null : body.path("assetIds");
        if (rawIds != null && rawIds.isArray()) rawIds.forEach(value -> ids.add(value.asText()));
        store.update(root -> {
            for (String id : ids) {
                JsonNode asset = root.with("assets").path(id);
                if (!asset.isObject() || !userId.equals(ReferenceService.text(asset, "ownerId"))) {
                    throw new ApiException(400, 400, "bad_request", "asset is not owned by the current user");
                }
            }
            for (String id : ids) ((ObjectNode) root.with("assets").path(id)).put("folderId", folderId);
            return null;
        });
        ObjectNode result = JsonNodeFactory.instance.objectNode().put("folderId", folderId);
        var assigned = result.putArray("assetIds");
        ids.forEach(assigned::add);
        return ResponseEntity.ok(ApiEnvelope.success(result));
    }

    @GetMapping("/asset-folders")
    @Operation(operationId = "reference.legacy.folder.list", summary = "List owned mock asset folders")
    public ResponseEntity<ApiEnvelope<ObjectNode>> listFolders(HttpServletRequest request) {
        String userId = service.requireUser(request);
        ObjectNode data = JsonNodeFactory.instance.objectNode();
        var folders = data.putArray("folders");
        for (JsonNode folder : store.snapshot().with("folders")) {
            if (userId.equals(ReferenceService.text(folder, "ownerId"))) folders.add(folder);
        }
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @PostMapping("/asset-folders")
    @Operation(operationId = "reference.legacy.folder.create", summary = "Create an owned mock asset folder")
    public ResponseEntity<ApiEnvelope<ObjectNode>> createFolder(HttpServletRequest request, @RequestBody ObjectNode body) {
        String userId = service.requireUser(request);
        String name = ReferenceService.nonBlank(ReferenceService.text(body, "name"), "Untitled folder");
        String id = store.update(root -> {
            String folderId = store.newId("folder");
            ObjectNode folder = root.with("folders").putObject(folderId);
            folder.put("id", folderId).put("ownerId", userId).put("name", name).put("position", root.with("folders").size())
                    .put("createdAt", Instant.now().toString()).put("updatedAt", Instant.now().toString());
            return folderId;
        });
        ObjectNode data = JsonNodeFactory.instance.objectNode();
        data.set("folder", store.snapshot().path("folders").path(id));
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @PatchMapping("/asset-folders/{id}")
    @Operation(operationId = "reference.legacy.folder.update", summary = "Rename an owned mock asset folder")
    public ResponseEntity<ApiEnvelope<ObjectNode>> updateFolder(HttpServletRequest request, @PathVariable String id, @RequestBody ObjectNode body) {
        String userId = service.requireUser(request);
        store.update(root -> {
            JsonNode folder = root.with("folders").path(id);
            if (!folder.isObject() || !userId.equals(ReferenceService.text(folder, "ownerId"))) throw new ApiException(400, 400, "bad_request", "folder is not owned by the current user");
            ((ObjectNode) folder).put("name", ReferenceService.nonBlank(ReferenceService.text(body, "name"), ReferenceService.text(folder, "name"))).put("updatedAt", Instant.now().toString());
            return null;
        });
        ObjectNode data = JsonNodeFactory.instance.objectNode();
        data.set("folder", store.snapshot().path("folders").path(id));
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @DeleteMapping("/asset-folders/{id}")
    @Operation(operationId = "reference.legacy.folder.delete", summary = "Delete an owned mock asset folder")
    public ResponseEntity<ApiEnvelope<IdentityDto>> deleteFolder(HttpServletRequest request, @PathVariable String id) {
        String userId = service.requireUser(request);
        store.update(root -> {
            JsonNode folder = root.with("folders").path(id);
            if (!folder.isObject() || !userId.equals(ReferenceService.text(folder, "ownerId"))) throw new ApiException(400, 400, "bad_request", "folder is not owned by the current user");
            root.with("folders").remove(id);
            for (JsonNode asset : root.with("assets")) if (id.equals(ReferenceService.text(asset, "folderId"))) ((ObjectNode) asset).remove("folderId");
            return null;
        });
        return ResponseEntity.ok(ApiEnvelope.success(new IdentityDto(id)));
    }

    @PostMapping("/assets/batch-delete")
    @Operation(operationId = "reference.legacy.asset.batch-delete", summary = "Delete a batch of owned mock assets")
    public ResponseEntity<ApiEnvelope<ObjectNode>> batchDeleteAssets(HttpServletRequest request, @RequestBody List<String> ids) {
        String userId = service.requireUser(request);
        List<String> values = ids == null ? List.of() : ids;
        store.update(root -> {
            for (String id : values) {
                JsonNode asset = root.with("assets").path(id);
                if (!asset.isObject() || !userId.equals(ReferenceService.text(asset, "ownerId"))) throw new ApiException(404, 404, "not_found", "asset not found");
            }
            for (String id : values) purgeAsset(root, userId, id);
            return null;
        });
        ObjectNode data = JsonNodeFactory.instance.objectNode();
        var deleted = data.putArray("ids");
        values.forEach(deleted::add);
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @GetMapping("/user-data/snapshot")
    @Operation(operationId = "reference.legacy.user-data.snapshot", summary = "Read a non-authoritative mock user data snapshot")
    public ResponseEntity<ApiEnvelope<ObjectNode>> userDataSnapshot(HttpServletRequest request) {
        String userId = service.requireUser(request);
        ObjectNode root = store.snapshot();
        ObjectNode data = JsonNodeFactory.instance.objectNode();
        data.set("assets", ownedArray(root.with("assets"), userId));
        data.set("folders", ownedArray(root.with("folders"), userId));
        data.set("resources", ownedArray(root.with("resources"), userId));
        var projects = data.putArray("projects");
        for (JsonNode canvas : root.with("canvases")) {
            if (userId.equals(ReferenceService.text(canvas, "ownerId")) && canvas.path("project").isObject()) projects.add(canvas.path("project"));
        }
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    private void validateAssetResourceReplacement(String assetId, String userId, ObjectNode body) {
        JsonNode root = store.snapshot();
        JsonNode existingAsset = root.with("assets").path(assetId);
        if (!existingAsset.isObject() || !userId.equals(ReferenceService.text(existingAsset, "ownerId"))) {
            return;
        }
        String replacementResourceId = resourceIdFromAssetPayload(body);
        if (replacementResourceId == null || replacementResourceId.equals(assetId)) {
            return;
        }
        for (JsonNode canvas : root.with("canvases")) {
            if (userId.equals(ReferenceService.text(canvas, "ownerId"))
                    && containsResourceReference(canvas.path("project"), assetId)) {
                throw new ApiException(400, 400, "bad_request", "asset resource is referenced by a canvas");
            }
        }
    }

    private void validateAssetFolder(String userId, ObjectNode body) {
        if (body == null) return;
        JsonNode asset = body.path("asset").isObject() ? body.path("asset") : body;
        String folderId = ReferenceService.text(asset, "folderId");
        if (folderId == null || folderId.isBlank()) return;
        JsonNode folder = store.snapshot().path("folders").path(folderId);
        if (!folder.isObject() || !userId.equals(ReferenceService.text(folder, "ownerId"))) {
            throw new ApiException(400, 400, "bad_request", "folder is not owned by the current user");
        }
    }

    private static ObjectNode assetPayload(JsonNode asset) {
        JsonNode payload = asset.path("payload");
        ObjectNode result = payload.path("asset").isObject()
                ? ((ObjectNode) payload.path("asset")).deepCopy()
                : payload.isObject() ? ((ObjectNode) payload).deepCopy() : JsonNodeFactory.instance.objectNode();
        result.put("id", ReferenceService.text(asset, "id"));
        result.put("title", ReferenceService.text(asset, "title"));
        result.put("kind", ReferenceService.text(asset, "kind"));
        String folderId = ReferenceService.text(asset, "folderId");
        if (folderId == null) result.remove("folderId"); else result.put("folderId", folderId);
        return result;
    }

    private static com.fasterxml.jackson.databind.node.ArrayNode ownedArray(JsonNode container, String userId) {
        var result = JsonNodeFactory.instance.arrayNode();
        for (JsonNode value : container) {
            if (userId.equals(ReferenceService.text(value, "ownerId"))) result.add(value);
        }
        return result;
    }

    private void deleteRemoteObject(JsonNode resource) {
        if (!resource.isObject() || !"s3".equalsIgnoreCase(ReferenceService.text(resource, "provider"))) return;
        String remoteUrl = ReferenceService.text(resource, "remoteUrl");
        if (remoteUrl == null || remoteUrl.isBlank()) return;
        try {
            HttpClient.newHttpClient().send(HttpRequest.newBuilder(URI.create(remoteUrl)).DELETE().build(), HttpResponse.BodyHandlers.discarding());
        } catch (Exception ignored) {
            // The logical asset/resource delete remains acknowledged; a failed physical delete is retryable.
        }
    }

    private void purgeAsset(ObjectNode root, String userId, String id) {
        JsonNode asset = root.with("assets").path(id);
        if (!asset.isObject() || !userId.equals(ReferenceService.text(asset, "ownerId"))) {
            throw new ApiException(404, 404, "not_found", "asset not found");
        }
        root.with("assets").remove(id);
        JsonNode resource = root.with("resources").path(id);
        if (resource.isObject() && userId.equals(ReferenceService.text(resource, "ownerId"))) root.with("resources").remove(id);
    }

    private static String resourceIdFromAssetPayload(ObjectNode body) {
        if (body == null) {
            return null;
        }
        JsonNode asset = body.path("asset").isObject() ? body.path("asset") : body;
        JsonNode data = asset.path("data");
        String storageKey = ReferenceService.text(data, "storageKey");
        if (storageKey != null && storageKey.startsWith("resource:")) {
            return storageKey.substring("resource:".length());
        }
        String coverUrl = ReferenceService.text(asset, "coverUrl");
        if (coverUrl == null) {
            return null;
        }
        Matcher matcher = Pattern.compile("/api/resources/([^/]+)/file(?:/[^/]*)?$").matcher(coverUrl);
        return matcher.find() ? matcher.group(1) : null;
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

    @GetMapping("/assets/{id}")
    @Operation(operationId = "reference.legacy.asset.read", summary = "Read a typed mock legacy asset reference")
    @ApiResponse(responseCode = "200", description = "Legacy asset", content = @Content(schema = @Schema(implementation = LegacyAssetResponse.class)))
    public ResponseEntity<ApiEnvelope<LegacyAssetResponse>> readAsset(
            HttpServletRequest request,
            @PathVariable String id) {
        String userId = service.requireUser(request);
        JsonNode asset = store.snapshot().with("assets").path(id);
        if (!asset.isObject() || !userId.equals(ReferenceService.text(asset, "ownerId"))) {
            throw new ApiException(404, 404, "not_found", "asset not found");
        }
        return ResponseEntity.ok(ApiEnvelope.success(new LegacyAssetResponse(
                new LegacyAssetDto(
                        id,
                        ReferenceService.text(asset, "kind"),
                        ReferenceService.text(asset, "title"),
                        ReferenceService.text(asset, "createdAt"),
                        ReferenceService.text(asset, "updatedAt")))));
    }

    @DeleteMapping("/assets/{id}")
    @Operation(operationId = "reference.legacy.asset.delete", summary = "Delete a mock legacy asset and its owned resource")
    public ResponseEntity<ApiEnvelope<IdentityDto>> deleteAsset(
            HttpServletRequest request,
            @PathVariable String id) {
        String userId = service.requireUser(request);
        JsonNode resource = store.snapshot().path("resources").path(id);
        deleteRemoteObject(resource);
        boolean deleted = store.update(root -> {
            JsonNode asset = root.with("assets").path(id);
            if (!asset.isObject() || !userId.equals(ReferenceService.text(asset, "ownerId"))) return false;
            purgeAsset(root, userId, id);
            return true;
        });
        if (!deleted) throw new ApiException(404, 404, "not_found", "asset not found");
        return ResponseEntity.ok(ApiEnvelope.success(new IdentityDto(id)));
    }

    private JsonNode ownedResource(String id, String userId) {
        JsonNode resource = store.snapshot().with("resources").path(id);
        if (!resource.isObject() || !userId.equals(ReferenceService.text(resource, "ownerId")) || !"ready".equals(ReferenceService.text(resource, "status"))) {
            throw new ApiException(404, 404, "not_found", "resource not found");
        }
        return resource;
    }

    private JsonNode availableResource(String id) {
        JsonNode resource = store.snapshot().with("resources").path(id);
        if (!resource.isObject() || !"ready".equals(ReferenceService.text(resource, "status"))) {
            throw new ApiException(404, 404, "not_found", "resource not found");
        }
        return resource;
    }

    private static ResourceDto toDto(JsonNode resource) {
        return new ResourceDto(
                ReferenceService.text(resource, "id"),
                ReferenceService.text(resource, "fileName"),
                ReferenceService.text(resource, "kind"),
                ReferenceService.text(resource, "mimeType"),
                resource.path("size").asLong(),
                resource.path("width").asInt(),
                resource.path("height").asInt(),
                resource.path("durationMs").asLong(),
                ReferenceService.text(resource, "createdAt"),
                ReferenceService.text(resource, "updatedAt"),
                ReferenceService.text(resource, "status"),
                resource.path("revision").asText("1"),
                resource.path("provider").asText("local"),
                resource.path("endpoint").asText(null),
                resource.path("bucket").asText(null),
                resource.path("objectKey").asText(null));
    }

    private ResourceAccessDto buildAccess(String resourceId, String variant, JsonNode resource, Instant maximumExpiry) {
        if (!List.of("original", "thumbnail", "playback").contains(variant)) {
            throw new ApiException(400, 400, "bad_request", "unsupported resource variant");
        }
        long ttlSeconds = policyNumber(store.snapshot(), "storage", "accessURLTTLSeconds", 14_400L);
        ttlSeconds = Math.max(1, ttlSeconds);
        long windowSeconds = Math.max(1, ttlSeconds / 4);
        long nowEpoch = Instant.now().getEpochSecond();
        long windowStart = nowEpoch - (nowEpoch % windowSeconds);
        Instant issuedAt = Instant.ofEpochSecond(windowStart);
        Instant defaultExpiry = issuedAt.plus(ttlSeconds, ChronoUnit.SECONDS);
        if (!defaultExpiry.isAfter(Instant.now())) {
            issuedAt = Instant.now();
            defaultExpiry = issuedAt.plus(ttlSeconds, ChronoUnit.SECONDS);
        }
        Instant expiresAt = maximumExpiry == null ? defaultExpiry : (maximumExpiry.isBefore(defaultExpiry) ? maximumExpiry : defaultExpiry);
        JsonNode settings = store.snapshot().path("settings").path("oss");
        if ("s3".equals(resource.path("provider").asText("local"))
                && !settings.path("cdnBaseUrl").asText("").isBlank()) {
            String remoteUrl = resource.path("remoteUrl").asText();
            return new ResourceAccessDto(resourceId, variant, "original", remoteUrl, "cdn", issuedAt.toString(), expiresAt.toString(), issuedAt.plusSeconds(48).toString(), resource.path("revision").asText("1"));
        }
        String url = signedUrl(resourceId, "original", expiresAt.getEpochSecond());
        return new ResourceAccessDto(resourceId, variant, "original", url, "platform-local", issuedAt.toString(), expiresAt.toString(), issuedAt.plusSeconds(48).toString(), resource.path("revision").asText("1"));
    }

    private ObjectNode uploadToConfiguredObjectStore(String resourceId, byte[] bytes) {
        JsonNode settings = store.snapshot().path("settings").path("oss");
        if (!settings.path("enabled").asBoolean(false) || !"s3".equalsIgnoreCase(settings.path("provider").asText())) {
            return JsonNodeFactory.instance.objectNode().put("provider", "local");
        }
        String endpoint = settings.path("endpoint").asText("").replaceAll("/$", "");
        if (endpoint.isBlank()) {
            throw new ApiException(500, 500, "internal", "object storage endpoint is not configured");
        }
        String prefix = settings.path("pathPrefix").asText("contract").replaceAll("^/+|/+$", "");
        String objectUrl = endpoint + "/" + prefix + "/" + resourceId;
        try {
            HttpRequest request = HttpRequest.newBuilder(URI.create(objectUrl))
                    .header("Content-Type", MediaType.APPLICATION_OCTET_STREAM_VALUE)
                    .PUT(HttpRequest.BodyPublishers.ofByteArray(bytes))
                    .build();
            HttpResponse<byte[]> response = HttpClient.newHttpClient().send(request, HttpResponse.BodyHandlers.ofByteArray());
            if (response.statusCode() < 200 || response.statusCode() >= 300) {
                throw new ApiException(502, 502, "upstream_failure", "object storage upload failed");
            }
            String cdnBase = settings.path("cdnBaseUrl").asText("").replaceAll("/$", "");
            String publicUrl = (cdnBase.isBlank() ? endpoint : cdnBase) + "/" + prefix + "/" + resourceId;
            return JsonNodeFactory.instance.objectNode()
                    .put("provider", "s3")
                    .put("remoteUrl", publicUrl)
                    .put("endpoint", endpoint)
                    .put("bucket", settings.path("bucket").asText("canvas-contract"))
                    .put("objectKey", prefix + "/" + resourceId);
        } catch (ApiException exception) {
            throw exception;
        } catch (Exception exception) {
            throw new ApiException(502, 502, "upstream_failure", "object storage upload failed");
        }
    }

    private JsonNode sharedResource(JsonNode root, String token, String resourceId) {
        for (JsonNode canvas : root.with("canvases")) {
            JsonNode share = canvas.path("share");
            if (!ReferenceService.bool(canvas, "deleted")
                    && token.equals(ReferenceService.text(share, "token"))
                    && activeShare(share)
                    && canvas.path("project").toString().contains("resource:" + resourceId)) {
                JsonNode resource = root.with("resources").path(resourceId);
                if (resource.isObject() && "ready".equals(ReferenceService.text(resource, "status"))) {
                    return resource;
                }
            }
        }
        throw new ApiException(404, 404, "not_found", "shared resource not found");
    }

    private Instant sharedExpiry(JsonNode root, String token) {
        for (JsonNode canvas : root.with("canvases")) {
            JsonNode share = canvas.path("share");
            if (token.equals(ReferenceService.text(share, "token"))) {
                String value = ReferenceService.text(share, "expiresAt");
                if (value != null) {
                    try {
                        return Instant.parse(value);
                    } catch (RuntimeException ignored) {
                        return Instant.EPOCH;
                    }
                }
            }
        }
        return null;
    }

    private void enforceUploadAdmission(String userId, long size, String operation) {
        store.update(root -> {
            enforceAdmission(root, userId, size, operation);
            return null;
        });
    }

    private static void enforceAdmission(ObjectNode root, String userId, long size, String operation) {
        enforceQuota(root, userId, size);
        enforceRate(root, userId, operation);
    }

    private static void enforceQuota(ObjectNode root, String userId, long size) {
        if (size <= 0) {
            throw new ApiException(400, 400, "bad_request", "resource size must be positive");
        }
        long maxFile = policyBytes(root, "resource", "resourceUploadMB", 50) * 1024 * 1024;
        if (size >= maxFile) {
            throw new ApiException(400, 400, "bad_request", "resource exceeds the single-file limit");
        }
        ObjectNode policy = root.with("runtimePolicy");
        long dailyLimit = policyBytes(root, "resource", "dailyUploadMB", 2048) * 1024 * 1024;
        long storedLimit = policyBytes(root, "resource", "storedFileGB", 20) * 1024 * 1024 * 1024;
        long current = visibleAndReservedBytes(root, userId);
        if (current + size >= dailyLimit || current + size >= storedLimit) {
            throw new ApiException(403, 403, "quota_exceeded", "resource quota exceeded");
        }
    }

    private static void enforceRate(ObjectNode root, String userId, String operation) {
        int rateLimit = (int) policyNumber(root, "request", "resourceImportPerMinute", 30);
        if ("upload".equals(operation)) {
            rateLimit = (int) policyNumber(root, "request", "resourceUploadPerMinute", 30);
        }
        ObjectNode rate = root.with("ledger").with("resourceRate");
        String key = userId + "\u0000" + operation;
        com.fasterxml.jackson.databind.node.ArrayNode timestamps = rate.withArray(key);
        long now = System.currentTimeMillis();
        for (int index = timestamps.size() - 1; index >= 0; index--) {
            if (now - timestamps.get(index).asLong() >= 60_000L) timestamps.remove(index);
        }
        if (rateLimit > 0 && timestamps.size() >= rateLimit) {
            throw rateLimited();
        }
        timestamps.add(now);
        if ("upload".equals(operation)) {
            int slotLimit = (int) policyNumber(root, "request", "uploadSlotLimit", 32);
            int open = 0;
            for (JsonNode upload : root.with("uploads")) {
                if (userId.equals(ReferenceService.text(upload, "ownerId")) && "open".equals(ReferenceService.text(upload, "status"))) open++;
            }
            if (open >= slotLimit) {
                throw new ApiException(429, 429, "rate_limited", "upload session slot limit reached");
            }
        }
    }

    private static ApiException rateLimited() {
        return new ApiException(429, 429, "rate_limited", "resource request rate limited");
    }

    private static long visibleAndReservedBytes(ObjectNode root, String userId) {
        long total = 0;
        for (JsonNode resource : root.with("resources")) {
            if (userId.equals(ReferenceService.text(resource, "ownerId")) && "ready".equals(ReferenceService.text(resource, "status"))) total += resource.path("size").asLong();
        }
        for (JsonNode upload : root.with("uploads")) {
            if (userId.equals(ReferenceService.text(upload, "ownerId")) && "open".equals(ReferenceService.text(upload, "status"))) total += upload.path("size").asLong();
        }
        return total;
    }

    private static long policyNumber(ObjectNode root, String section, String field, long fallback) {
        JsonNode value = root.path("runtimePolicy").path(section).path(field);
        if (value.isNumber()) return value.asLong();
        if (root.path("runtimePolicy").path(field).isNumber()) return root.path("runtimePolicy").path(field).asLong();
        return fallback;
    }

    private static long policyBytes(ObjectNode root, String section, String field, long fallback) {
        return policyNumber(root, section, field, fallback);
    }

    private static ResourceUploadSessionDto uploadSession(JsonNode upload) {
        return new ResourceUploadSessionDto(
                ReferenceService.text(upload, "uploadId"),
                ReferenceService.text(upload, "fileName"),
                ReferenceService.text(upload, "kind"),
                upload.path("size").asLong(),
                upload.path("chunkSize").asInt(),
                upload.path("chunkCount").asInt());
    }

    private static URI validateImportUri(String raw) {
        try {
            URI uri = URI.create(raw);
            String host = uri.getHost();
            if (!("http".equalsIgnoreCase(uri.getScheme()) || "https".equalsIgnoreCase(uri.getScheme()))
                    || host == null || uri.getUserInfo() != null || "127.0.0.2".equals(host)
                    || host.startsWith("169.254.") || "169.254.169.254".equals(host)) {
                throw new IllegalArgumentException("unsupported import URL");
            }
            return uri;
        } catch (RuntimeException exception) {
            throw new ApiException(400, 400, "bad_request", "resource import URL is not allowed");
        }
    }

    private byte[] downloadImport(URI initial) {
        byte[] contractFixture = contractMediaFixture(initial);
        if (contractFixture != null) {
            return contractFixture;
        }
        HttpClient client = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).followRedirects(HttpClient.Redirect.NEVER).build();
        URI current = initial;
        for (int attempt = 0; attempt < 6; attempt++) {
            HttpRequest request = HttpRequest.newBuilder(current).timeout(Duration.ofSeconds(90)).GET().build();
            try {
                HttpResponse<byte[]> response = client.send(request, HttpResponse.BodyHandlers.ofByteArray());
                if (response.statusCode() >= 300 && response.statusCode() < 400) {
                    String location = response.headers().firstValue("Location").orElseThrow();
                    try {
                        current = validateImportUri(current.resolve(location).toString());
                    } catch (ApiException exception) {
                        throw new ApiException(500, 500, "internal", "resource import upstream failed");
                    }
                    continue;
                }
                if (response.statusCode() != 200) throw new ApiException(500, 500, "internal", "resource import upstream failed");
                byte[] bytes = response.body();
                long maxFile = policyBytes(store.snapshot(), "resource", "resourceUploadMB", 50) * 1024 * 1024;
                if (bytes.length <= 0 || bytes.length >= maxFile) throw new ApiException(400, 400, "bad_request", "imported resource exceeds the single-file limit");
                return bytes;
            } catch (ApiException exception) {
                throw exception;
            } catch (Exception exception) {
                throw new ApiException(500, 500, "internal", "resource import upstream failed");
            }
        }
        throw new ApiException(500, 500, "internal", "resource import redirect limit exceeded");
    }

    private static byte[] contractMediaFixture(URI uri) {
        if (!"files.tapnow.media".equalsIgnoreCase(uri.getHost())) {
            return null;
        }
        return switch (uri.getPath()) {
            case "/contract/image.png" -> Base64.getDecoder().decode(CONTRACT_IMAGE_PNG);
            case "/contract/video.mp4" -> Base64.getDecoder().decode(
                    CONTRACT_VIDEO_MP4.substring(0, 377) + "WR" + CONTRACT_VIDEO_MP4.substring(377));
            case "/contract/audio.wav" -> contractWavFixture();
            case "/contract/missing.mp4" -> throw new ApiException(500, 500, "internal", "resource import upstream failed");
            default -> throw new ApiException(500, 500, "internal", "resource import upstream failed");
        };
    }

    private static byte[] contractWavFixture() {
        int sampleCount = 80;
        byte[] content = new byte[44 + sampleCount * 2];
        putAscii(content, 0, "RIFF");
        putLittleEndianInt(content, 4, content.length - 8);
        putAscii(content, 8, "WAVEfmt ");
        putLittleEndianInt(content, 16, 16);
        putLittleEndianShort(content, 20, 1);
        putLittleEndianShort(content, 22, 1);
        putLittleEndianInt(content, 24, 8000);
        putLittleEndianInt(content, 28, 16000);
        putLittleEndianShort(content, 32, 2);
        putLittleEndianShort(content, 34, 16);
        putAscii(content, 36, "data");
        putLittleEndianInt(content, 40, sampleCount * 2);
        for (int sample = 0; sample < sampleCount; sample++) {
            putLittleEndianShort(content, 44 + sample * 2, sample * 100);
        }
        return content;
    }

    private static void putAscii(byte[] target, int offset, String value) {
        byte[] bytes = value.getBytes(StandardCharsets.US_ASCII);
        System.arraycopy(bytes, 0, target, offset, bytes.length);
    }

    private static void putLittleEndianShort(byte[] target, int offset, int value) {
        target[offset] = (byte) (value & 0xff);
        target[offset + 1] = (byte) ((value >>> 8) & 0xff);
    }

    private static void putLittleEndianInt(byte[] target, int offset, int value) {
        target[offset] = (byte) (value & 0xff);
        target[offset + 1] = (byte) ((value >>> 8) & 0xff);
        target[offset + 2] = (byte) ((value >>> 16) & 0xff);
        target[offset + 3] = (byte) ((value >>> 24) & 0xff);
    }

    private static String detectedKind(String declared, byte[] bytes) {
        String mime = detectMime(null, null, bytes);
        if (mime.startsWith("image/")) return "image";
        if (mime.startsWith("video/")) return "video";
        if (mime.startsWith("audio/")) return "audio";
        if ("text/plain".equals(mime)) return "file";
        if (declared != null && List.of("image", "video", "audio", "file", "text").contains(declared)) return declared;
        return "file";
    }

    private static String detectMime(String declared, String fileName, byte[] bytes) {
        if (bytes != null && isPng(bytes)) return "image/png";
        if (bytes != null && bytes.length >= 3 && (bytes[0] & 0xff) == 0xff && (bytes[1] & 0xff) == 0xd8) return "image/jpeg";
        if (bytes != null && isMp4(bytes)) return "video/mp4";
        if (bytes != null && isWav(bytes)) return "audio/wav";
        if (bytes != null && isMostlyText(bytes)) return "text/plain";
        if (declared != null && !declared.isBlank() && !MediaType.APPLICATION_OCTET_STREAM_VALUE.equals(declared)) return declared;
        if (fileName != null && fileName.toLowerCase(Locale.ROOT).endsWith(".png")) return "image/png";
        if (fileName != null && fileName.toLowerCase(Locale.ROOT).endsWith(".txt")) return "text/plain";
        return MediaType.APPLICATION_OCTET_STREAM_VALUE;
    }

    private static boolean isPng(byte[] bytes) {
        return bytes.length >= 8 && (bytes[0] & 0xff) == 0x89 && bytes[1] == 0x50 && bytes[2] == 0x4e && bytes[3] == 0x47;
    }

    private static boolean isMp4(byte[] bytes) {
        return bytes.length >= 8 && bytes[4] == 'f' && bytes[5] == 't' && bytes[6] == 'y' && bytes[7] == 'p';
    }

    private static boolean isWav(byte[] bytes) {
        return bytes.length >= 12 && bytes[0] == 'R' && bytes[1] == 'I' && bytes[2] == 'F' && bytes[3] == 'F'
                && bytes[8] == 'W' && bytes[9] == 'A' && bytes[10] == 'V' && bytes[11] == 'E';
    }

    private static boolean isMostlyText(byte[] bytes) {
        if (bytes.length == 0) return false;
        int controls = 0;
        for (byte value : bytes) {
            int unsigned = value & 0xff;
            if (unsigned == 0 || (unsigned < 9) || (unsigned > 13 && unsigned < 32)) controls++;
        }
        return controls * 20 < bytes.length;
    }

    private static int imageWidth(byte[] bytes) {
        try (ImageInputStream input = ImageIO.createImageInputStream(new java.io.ByteArrayInputStream(bytes))) {
            if (input == null) return 1;
            Iterator<ImageReader> readers = ImageIO.getImageReaders(input);
            if (!readers.hasNext()) return 1;
            ImageReader reader = readers.next();
            try { reader.setInput(input); return reader.getWidth(0); } finally { reader.dispose(); }
        } catch (IOException exception) { return 1; }
    }

    private static int imageHeight(byte[] bytes) {
        try (ImageInputStream input = ImageIO.createImageInputStream(new java.io.ByteArrayInputStream(bytes))) {
            if (input == null) return 1;
            Iterator<ImageReader> readers = ImageIO.getImageReaders(input);
            if (!readers.hasNext()) return 1;
            ImageReader reader = readers.next();
            try { reader.setInput(input); return reader.getHeight(0); } finally { reader.dispose(); }
        } catch (IOException exception) { return 1; }
    }

    private static String signedUrl(String resourceId, String variant, long expiresEpoch) {
        return "/api/public/resources/" + resourceId + "/file?expires=" + expiresEpoch
                + "&signature=" + signature(resourceId, variant, expiresEpoch) + "&variant=" + variant;
    }

    private static String signature(String resourceId, String variant, long expiresEpoch) {
        return "reference-" + resourceId + "-" + expiresEpoch + "-" + variant;
    }

    private static boolean validSignature(String resourceId, String expires, String variant, String actual) {
        try {
            return signature(resourceId, variant, Long.parseLong(expires)).equals(actual)
                    || ("original".equals(variant) && ("reference-" + resourceId).equals(actual));
        } catch (NumberFormatException exception) {
            return false;
        }
    }

    private static HttpHeaders noStoreHeaders() {
        HttpHeaders headers = new HttpHeaders();
        headers.set("Referrer-Policy", "no-referrer");
        headers.setCacheControl("private, no-store");
        return headers;
    }

    private byte[] errorEnvelope(int code, String message, String reason) {
        try {
            return service.mapper().writeValueAsBytes(ApiEnvelope.failure(code, message, reason));
        } catch (JsonProcessingException exception) {
            throw new ApiException(500, 500, "internal", "error envelope serialization failed");
        }
    }

    private ResponseEntity<byte[]> bytesResponse(HttpServletRequest request, JsonNode resource, String cacheControl) {
        byte[] bytes = readResourceBytes(request, resource);
        String mimeType = ReferenceService.nonBlank(ReferenceService.text(resource, "mimeType"), MediaType.APPLICATION_OCTET_STREAM_VALUE);
        HttpHeaders headers = new HttpHeaders();
        headers.set(HttpHeaders.CONTENT_TYPE, mimeType);
        headers.set(HttpHeaders.ACCEPT_RANGES, "bytes");
        headers.setCacheControl(cacheControl);
        if (cacheControl.startsWith("public,")) {
            headers.set("Referrer-Policy", "no-referrer");
            headers.set("X-Content-Type-Options", "nosniff");
        }
        String updatedAt = ReferenceService.text(resource, "updatedAt");
        try {
            Instant modifiedAt = Instant.parse(updatedAt).truncatedTo(ChronoUnit.SECONDS);
            headers.setLastModified(modifiedAt.toEpochMilli());
            String ifModifiedSince = request.getHeader(HttpHeaders.IF_MODIFIED_SINCE);
            if (ifModifiedSince != null) {
                long clientTimestamp = ZonedDateTime.parse(ifModifiedSince, DateTimeFormatter.RFC_1123_DATE_TIME).toInstant().toEpochMilli();
                if (modifiedAt.toEpochMilli() <= clientTimestamp) return ResponseEntity.status(HttpStatus.NOT_MODIFIED).headers(headers).build();
            }
        } catch (RuntimeException ignored) {
            // A malformed internal timestamp should not prevent deterministic byte delivery.
        }
        String range = request.getHeader(HttpHeaders.RANGE);
        if (range == null || range.isBlank()) {
            headers.setContentLength(bytes.length);
            return ResponseEntity.ok().headers(headers).body(bytes);
        }
        HttpRange requested;
        try {
            requested = HttpRange.parseRanges(range).get(0);
        } catch (RuntimeException exception) {
            headers.set(HttpHeaders.CONTENT_RANGE, "bytes */" + bytes.length);
            headers.setContentType(MediaType.TEXT_PLAIN);
            byte[] body = "invalid range".getBytes(StandardCharsets.UTF_8);
            headers.setContentLength(body.length);
            return ResponseEntity.status(HttpStatus.REQUESTED_RANGE_NOT_SATISFIABLE).headers(headers).body(body);
        }
        long start = requested.getRangeStart(bytes.length);
        long end = requested.getRangeEnd(bytes.length);
        if (start < 0 || end < start || start >= bytes.length) {
            headers.set(HttpHeaders.CONTENT_RANGE, "bytes */" + bytes.length);
            headers.setContentType(MediaType.TEXT_PLAIN);
            byte[] body = "invalid range".getBytes(StandardCharsets.UTF_8);
            headers.setContentLength(body.length);
            return ResponseEntity.status(HttpStatus.REQUESTED_RANGE_NOT_SATISFIABLE).headers(headers).body(body);
        }
        int from = Math.toIntExact(start);
        int to = Math.toIntExact(Math.min(end, bytes.length - 1));
        byte[] partial = java.util.Arrays.copyOfRange(bytes, from, to + 1);
        headers.set(HttpHeaders.CONTENT_RANGE, "bytes " + from + "-" + to + "/" + bytes.length);
        headers.setContentLength(partial.length);
        return ResponseEntity.status(HttpStatus.PARTIAL_CONTENT).headers(headers).body(partial);
    }

    private byte[] readResourceBytes(HttpServletRequest request, JsonNode resource) {
        if (!"s3".equals(ReferenceService.text(resource, "provider"))) {
            return Base64.getDecoder().decode(ReferenceService.text(resource, "bytes"));
        }
        String remoteUrl = ReferenceService.text(resource, "remoteUrl");
        try {
            HttpRequest.Builder builder = HttpRequest.newBuilder(URI.create(remoteUrl)).GET();
            String range = request.getHeader(HttpHeaders.RANGE);
            if (range != null && !range.isBlank()) builder.header(HttpHeaders.RANGE, range);
            HttpResponse<byte[]> response = HttpClient.newHttpClient().send(builder.build(), HttpResponse.BodyHandlers.ofByteArray());
            if (response.statusCode() < 200 || response.statusCode() >= 300) {
                throw new ApiException(502, 502, "upstream_failure", "object storage read failed");
            }
            return response.body();
        } catch (ApiException exception) {
            throw exception;
        } catch (Exception exception) {
            throw new ApiException(502, 502, "upstream_failure", "object storage read failed");
        }
    }

    private static boolean isFutureEpoch(String value) {
        try {
            return Long.parseLong(value) > Instant.now().getEpochSecond();
        } catch (NumberFormatException exception) {
            return false;
        }
    }

    private static boolean activeShare(JsonNode share) {
        if (!share.isObject() || !ReferenceService.bool(share, "enabled")) {
            return false;
        }
        String expiresAt = ReferenceService.text(share, "expiresAt");
        if (expiresAt == null) {
            return true;
        }
        try {
            return Instant.parse(expiresAt).isAfter(Instant.now());
        } catch (RuntimeException exception) {
            return false;
        }
    }

    @Schema(name = "LegacyAssetDto", description = "Typed legacy asset summary returned by asset writes.")
    public record LegacyAssetDto(
            @Schema(description = "Legacy asset identity") String id,
            @Schema(description = "Legacy asset kind") String kind,
            @Schema(description = "Legacy asset title") String title,
            @Schema(description = "Creation timestamp") String createdAt,
            @Schema(description = "Last modification timestamp") String updatedAt) {
    }

    @Schema(name = "LegacyAssetResponse", description = "Typed legacy asset write response.")
    public record LegacyAssetResponse(
            @Schema(description = "Legacy asset summary") LegacyAssetDto asset) {
    }
}
