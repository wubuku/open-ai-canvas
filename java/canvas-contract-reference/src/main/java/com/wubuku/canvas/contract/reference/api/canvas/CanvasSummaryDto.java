package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "CanvasSummary", description = "Typed canvas summary used by create/write and library responses.")
public record CanvasSummaryDto(
        @Schema(requiredMode = Schema.RequiredMode.REQUIRED) String id,
        @Schema(requiredMode = Schema.RequiredMode.REQUIRED) String title,
        @Schema(requiredMode = Schema.RequiredMode.REQUIRED) long revision,
        @Schema(requiredMode = Schema.RequiredMode.REQUIRED) String createdAt,
        @Schema(requiredMode = Schema.RequiredMode.REQUIRED) String updatedAt,
        @Schema(description = "Number of nodes") int nodeCount,
        @Schema(description = "Typed preview nodes") java.util.List<CanvasNodeDto> previewNodes) {
}
