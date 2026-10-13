package com.wubuku.canvas.contract.reference.api.canvas;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.wubuku.canvas.contract.reference.api.common.OpaqueJsonValue;
import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;

@Schema(name = "CanvasNode", description = "A strongly typed canvas node with an explicit metadata extension boundary.")
@JsonIgnoreProperties(ignoreUnknown = true)
public record CanvasNodeDto(
        @NotBlank @Schema(description = "Stable node identity", example = "node-1") String id,
        @Schema(description = "Node kind", example = "image") String type,
        @Schema(description = "Node title") String title,
        @Valid @Schema(description = "Node position") CanvasPositionDto position,
        @Schema(description = "Legacy wire x coordinate") Double x,
        @Schema(description = "Legacy wire y coordinate") Double y,
        @Schema(description = "Node width") Integer width,
        @Schema(description = "Node height") Integer height,
        @Schema(ref = "#/components/schemas/OpaqueJsonValue", description = "Client-owned node metadata extension") OpaqueJsonValue metadata,
        @Schema(description = "Optional node content") String content) {
}
