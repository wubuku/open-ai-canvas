package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "Resource", description = "Typed mock resource metadata stored by the reference stub.")
public record ResourceDto(
        @Schema(description = "Stable resource identity", requiredMode = Schema.RequiredMode.REQUIRED) String id,
        @Schema(description = "Original client filename", requiredMode = Schema.RequiredMode.REQUIRED) String fileName,
        @Schema(description = "Resource kind such as image, video, or file", requiredMode = Schema.RequiredMode.REQUIRED) String kind,
        @Schema(description = "Detected or supplied media type", requiredMode = Schema.RequiredMode.REQUIRED) String mimeType,
        @Schema(description = "Stored byte count", requiredMode = Schema.RequiredMode.REQUIRED) long size,
        @Schema(description = "Detected image width when available") int width,
        @Schema(description = "Detected image height when available") int height,
        @Schema(description = "Media duration in milliseconds when available", requiredMode = Schema.RequiredMode.REQUIRED) long durationMs,
        @Schema(description = "Creation timestamp", requiredMode = Schema.RequiredMode.REQUIRED) String createdAt,
        @Schema(description = "Last modification timestamp", requiredMode = Schema.RequiredMode.REQUIRED) String updatedAt,
        @Schema(description = "Reference lifecycle state", requiredMode = Schema.RequiredMode.REQUIRED) String status,
        @Schema(description = "Resource revision used by access capabilities", requiredMode = Schema.RequiredMode.REQUIRED) String revision,
        @Schema(description = "Storage provider used by the reference stub") String provider,
        @Schema(description = "Configured object-storage endpoint when the provider is remote") String endpoint,
        @Schema(description = "Configured object-storage bucket when the provider is remote") String bucket,
        @Schema(description = "Stable object key when the provider is remote") String objectKey) {
}
