package com.wubuku.canvas.contract.reference.api.common;

import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonValue;
import com.fasterxml.jackson.databind.annotation.JsonDeserialize;
import com.fasterxml.jackson.databind.JsonNode;
import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import jakarta.validation.Valid;
import java.util.List;

public final class ReferenceDtos {
    private ReferenceDtos() {
    }

    @Schema(name = "ApiEnvelope", description = "The stable canvas response envelope.")
    @JsonInclude(JsonInclude.Include.ALWAYS)
    public record ApiEnvelope<T>(int code, T data, String msg, String reason) {
        public static <T> ApiEnvelope<T> success(T data) {
            return new ApiEnvelope<>(0, data, "ok", "");
        }

        public static <T> ApiEnvelope<T> failure(int code, String msg, String reason) {
            return new ApiEnvelope<>(code, null, msg, reason);
        }
    }

    @Schema(name = "ReferenceObject", description = "A named, schema-owned JSON value used only for wire fields whose current contract is extensible.")
    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record ReferenceObjectDto(JsonNode value) {
        @JsonCreator(mode = JsonCreator.Mode.DELEGATING)
        public ReferenceObjectDto {
        }

        @JsonValue
        public JsonNode asJson() {
            return value;
        }
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static final class WireData {
        @Schema(description = "Canvas or workbench project representation.")
        public ReferenceObjectDto project;
        public List<ReferenceObjectDto> projects;
        public ReferenceObjectDto snapshot;
        public List<ReferenceObjectDto> snapshots;
        public Integer currentRevision;
        public ReferenceObjectDto share;
        public ReferenceObjectDto resource;
        public List<ReferenceObjectDto> resources;
        public ReferenceObjectDto usage;
        public ReferenceObjectDto access;
        public ReferenceObjectDto upload;
        public ReferenceObjectDto unit;
        public List<ReferenceObjectDto> units;
        public ReferenceObjectDto workspace;
        public ReferenceObjectDto canvasLink;
        public List<ReferenceObjectDto> canvasLinks;
        public ReferenceObjectDto task;
        public List<ReferenceObjectDto> tasks;
        public ReferenceObjectDto delta;
        public List<ReferenceObjectDto> deltas;
        public List<ReferenceObjectDto> events;
        public ReferenceObjectDto creationRun;
        public List<ReferenceObjectDto> creationRuns;
        public ReferenceObjectDto user;
        public ReferenceObjectDto session;
        public ReferenceObjectDto features;
        public ReferenceObjectDto runtimePolicy;
        public ReferenceObjectDto setting;
        public ReferenceObjectDto channel;
        public ReferenceObjectDto model;
        public ReferenceObjectDto imported;
        public ReferenceObjectDto fileDescriptor;
        public ReferenceObjectDto settings;
        public String cursor;
        public String message;
    }

    public record RegisterRequest(
            @NotBlank @Size(max = 120) String username,
            @NotBlank @Size(max = 200) String password,
            @Size(max = 200) String displayName,
            Boolean acceptedTerms) {
    }

    public record LoginRequest(@NotBlank String username, @NotBlank String password) {
    }

    public record AdminUserRequest(
            @NotBlank @Size(max = 120) String username,
            @NotBlank @Size(max = 200) String password,
            @Size(max = 200) String displayName,
            String role,
            String status) {
    }

    public record FeatureSettingsRequest(Boolean shortDramaEnabled, Boolean canvasEnabled) {
    }

    public record RuntimePolicyRequest(
            Long singleFileBytes,
            Long dailyBytes,
            Long storedBytes,
            Integer accessTtlSeconds,
            Integer uploadSlotLimit) {
    }

    public record ChannelRequest(String name, String provider, String baseUrl, List<String> models) {
    }

    public record ModelRequest(String model, String type, String capability, String channelId) {
    }

    public record ShareRequest(Integer expiresDays, Boolean rotate) {
    }

    public record ProjectRequest(
            @NotBlank @Size(max = 240) String name,
            String type,
            String aspectRatio,
            String sourceType) {
    }

    public record UnitRequest(
            String kind,
            @NotBlank String title,
            String sourceText,
            Integer position) {
    }

    public record UnitImportRequest(@NotNull @Size(max = 2500) List<@Valid UnitRequest> units) {
    }

    public record ReorderRequest(@NotNull List<String> unitIds) {
    }

    public record CanvasLinkRequest(
            @NotBlank String canvasId,
            @NotBlank String unitId,
            @JsonDeserialize(using = StrictStringDeserializer.class) String role) {
    }

    public record ResourceAccessRequest(
            @NotBlank String resourceId,
            String purpose,
            String variant) {
    }

    public record ResourceImportRequest(@NotBlank String url, String kind) {
    }

    public record UploadCreateRequest(
            @NotBlank @Size(max = 255) String fileName,
            @Size(max = 64) String kind,
            @NotNull Long size,
            String idempotencyKey) {
    }

    public record TextDeltaRequest(@NotBlank String text, String requestId) {
    }

    public record TextReplayCompleteRequest(String status) {
    }

    public record SessionDto(String userId, String username, String displayName, String role) {
    }

    public record ErrorDetailDto(String field, String message) {
    }

    @Schema(name = "CanvasTimestampFixtureRequest", description = "Private deterministic fixture input for the contract suite.")
    public record CanvasTimestampFixtureRequest(@NotNull List<@NotBlank String> projectIds, @NotBlank String updatedAt) {
    }

    @Schema(name = "UploadExpiryFixtureRequest", description = "Private deterministic upload fixture input for the contract suite.")
    public record UploadExpiryFixtureRequest(@NotBlank String uploadId) {
    }

    @Schema(name = "ShareExpiryFixtureRequest", description = "Private deterministic share fixture input for the contract suite.")
    public record ShareExpiryFixtureRequest(@NotBlank String projectId) {
    }

    public record CanvasTimestampFixtureResponse(int updated, String updatedAt) {
    }

    public record UploadExpiryFixtureResponse(boolean expired, String uploadId) {
    }

    public record ShareExpiryFixtureResponse(boolean expired, String projectId) {
    }
}
