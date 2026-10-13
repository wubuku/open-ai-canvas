package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "CanvasRestoreRequest", description = "Compare-and-swap history restore request.")
public record CanvasRestoreRequest(
        @Schema(description = "Current canvas revision expected by the restore") Integer revision) {
}
