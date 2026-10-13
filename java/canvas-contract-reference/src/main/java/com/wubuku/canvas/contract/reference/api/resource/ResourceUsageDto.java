package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "ResourceUsage", description = "Deterministic mock storage usage.")
public record ResourceUsageDto(
        @Schema(description = "Bytes used by ready resources", requiredMode = Schema.RequiredMode.REQUIRED) long usedBytes,
        @Schema(description = "Configured storage limit", requiredMode = Schema.RequiredMode.REQUIRED) long totalBytes,
        @Schema(description = "Number of ready resources", requiredMode = Schema.RequiredMode.REQUIRED) int resourceCount) {
}
