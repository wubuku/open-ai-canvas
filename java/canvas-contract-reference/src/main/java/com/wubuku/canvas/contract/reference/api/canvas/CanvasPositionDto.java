package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "CanvasPosition", description = "Canvas node position.")
public record CanvasPositionDto(
        @Schema(description = "Horizontal coordinate") Double x,
        @Schema(description = "Vertical coordinate") Double y) {
}
