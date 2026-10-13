package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "CanvasHistoryReadResponse", description = "Typed history snapshot and its document payload.")
public record CanvasHistoryReadResponse(
        @Schema(description = "Typed retained snapshot metadata", requiredMode = Schema.RequiredMode.REQUIRED)
        CanvasHistorySnapshotDto snapshot,
        @Schema(description = "Typed canvas document captured by the snapshot", requiredMode = Schema.RequiredMode.REQUIRED)
        CanvasDocumentDto project) {
}
