package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotNull;

@Schema(name = "CanvasWriteRequest", description = "Compare-and-swap canvas write request.")
public record CanvasWriteRequest(
        @NotNull @Valid @Schema(description = "Typed canvas document payload") CanvasDocumentDto project,
        @Schema(description = "Reference-only repair mode for missing media references") Boolean repairMissingResources) {
}
