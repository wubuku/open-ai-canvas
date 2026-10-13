package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "ResourceUsageResponse", description = "Typed resource usage response payload.")
public record ResourceUsageResponse(
        @Schema(description = "Storage usage") ResourceUsageDto usage) {
}
