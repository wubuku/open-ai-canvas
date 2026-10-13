package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "CanvasReadResponse", description = "Typed response for a canvas document read.")
public record CanvasReadResponse(
        @Schema(description = "Typed canvas document") CanvasDocumentDto project) {
}
