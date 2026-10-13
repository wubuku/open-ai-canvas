package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "CanvasViewport", description = "Canvas viewport state.")
public record CanvasViewportDto(
        @Schema(description = "Horizontal viewport offset") Double x,
        @Schema(description = "Vertical viewport offset") Double y,
        @Schema(description = "Zoom scale") Double k) {
}
