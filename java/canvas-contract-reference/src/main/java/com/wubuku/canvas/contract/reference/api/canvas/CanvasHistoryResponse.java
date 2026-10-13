package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;
import java.util.List;

@Schema(name = "CanvasHistoryResponse", description = "Typed list of retained canvas snapshots.")
public record CanvasHistoryResponse(
        @Schema(description = "Retained snapshots ordered newest first", requiredMode = Schema.RequiredMode.REQUIRED)
        List<CanvasHistorySnapshotDto> snapshots,
        @Schema(description = "Current live canvas revision", requiredMode = Schema.RequiredMode.REQUIRED)
        int currentRevision) {
}
