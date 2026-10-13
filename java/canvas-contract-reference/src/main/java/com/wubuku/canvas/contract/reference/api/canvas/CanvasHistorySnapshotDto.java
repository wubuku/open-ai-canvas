package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "CanvasHistorySnapshot", description = "Typed retained canvas history metadata.")
public record CanvasHistorySnapshotDto(
        @Schema(description = "Stable retained snapshot identity", requiredMode = Schema.RequiredMode.REQUIRED) String id,
        @Schema(description = "Canvas identity owning the snapshot", requiredMode = Schema.RequiredMode.REQUIRED) String canvasId,
        @Schema(description = "Canvas revision represented by this snapshot", requiredMode = Schema.RequiredMode.REQUIRED) long revision,
        @Schema(description = "Canvas title at snapshot time", requiredMode = Schema.RequiredMode.REQUIRED) String title,
        @Schema(description = "Number of nodes in the snapshot", requiredMode = Schema.RequiredMode.REQUIRED) int nodeCount,
        @Schema(description = "Number of connections in the snapshot", requiredMode = Schema.RequiredMode.REQUIRED) int connectionCount,
        @Schema(description = "Serialized payload size in bytes", requiredMode = Schema.RequiredMode.REQUIRED) int payloadBytes,
        @Schema(description = "Reference-only reason for retaining the snapshot", requiredMode = Schema.RequiredMode.REQUIRED) String reason,
        @Schema(description = "Content modification time represented by the snapshot", requiredMode = Schema.RequiredMode.REQUIRED) String contentUpdatedAt,
        @Schema(description = "Snapshot creation time", requiredMode = Schema.RequiredMode.REQUIRED) String createdAt) {
}
