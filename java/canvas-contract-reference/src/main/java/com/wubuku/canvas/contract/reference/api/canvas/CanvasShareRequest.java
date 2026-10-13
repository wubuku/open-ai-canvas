package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;

@Schema(name = "CanvasShareRequest", description = "Create or rotate a public canvas share.")
public record CanvasShareRequest(
        @Min(0) @Max(365)
        @Schema(description = "Lifetime in whole days. Zero creates a non-expiring share.", example = "7")
        Integer expiresDays,
        @Schema(description = "When true, publish a new token; when false, reuse the active token")
        Boolean rotate) {
}
