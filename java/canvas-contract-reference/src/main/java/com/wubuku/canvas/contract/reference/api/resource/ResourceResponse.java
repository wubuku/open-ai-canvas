package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "ResourceResponse", description = "Typed resource response envelope payload.")
public record ResourceResponse(
        @Schema(description = "Resource metadata") ResourceDto resource) {
}
