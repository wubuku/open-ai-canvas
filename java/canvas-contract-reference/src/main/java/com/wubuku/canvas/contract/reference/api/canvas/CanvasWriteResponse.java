package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "CanvasWriteResponse", description = "Typed response for a canvas create or write.")
public record CanvasWriteResponse(
        @Schema(description = "Typed canvas summary after the write") CanvasSummaryDto project) {
}
