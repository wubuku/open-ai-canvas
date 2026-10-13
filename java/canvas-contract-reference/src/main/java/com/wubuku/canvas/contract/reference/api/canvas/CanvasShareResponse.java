package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "CanvasShareResponse", description = "Typed canvas share management response.")
public record CanvasShareResponse(
        @Schema(description = "Current share capability") CanvasShareDto share) {
}
