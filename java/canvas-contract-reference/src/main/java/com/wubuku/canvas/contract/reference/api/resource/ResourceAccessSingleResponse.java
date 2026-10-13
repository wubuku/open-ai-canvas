package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "ResourceAccessSingleResponse", description = "Typed response for one resource access description.")
public record ResourceAccessSingleResponse(
        @Schema(description = "Short-lived resource access capability") ResourceAccessDto access) {
}
