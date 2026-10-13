package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;
import java.util.List;

@Schema(name = "CanvasLibraryResponse", description = "Typed canvas library response with optional pagination metadata.")
public record CanvasLibraryResponse(
        @Schema(description = "One-based page number; null when pagination is not requested") Integer page,
        @Schema(description = "Applied page size; null when pagination is not requested") Integer pageSize,
        @Schema(description = "Total visible canvas summaries") Integer total,
        @Schema(description = "Whether another page is available") Boolean hasMore,
        @Schema(description = "Typed canvas summaries") List<CanvasSummaryDto> projects) {
}
