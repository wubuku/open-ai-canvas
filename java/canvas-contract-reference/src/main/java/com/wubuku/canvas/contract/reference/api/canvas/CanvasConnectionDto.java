package com.wubuku.canvas.contract.reference.api.canvas;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.wubuku.canvas.contract.reference.api.common.OpaqueJsonValue;
import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.constraints.NotBlank;

@Schema(name = "CanvasConnection", description = "A typed directed connection between two canvas nodes.")
@JsonIgnoreProperties(ignoreUnknown = true)
public record CanvasConnectionDto(
        @NotBlank @Schema(description = "Stable connection identity") String id,
        @NotBlank @Schema(description = "Source node identity") String fromNodeId,
        @NotBlank @Schema(description = "Target node identity") String toNodeId,
        @Schema(description = "Optional source handle identity") String fromHandleId,
        @Schema(description = "Optional target handle identity") String toHandleId,
        @Schema(ref = "#/components/schemas/OpaqueJsonValue", description = "Client-owned connection extension") OpaqueJsonValue metadata) {
}
