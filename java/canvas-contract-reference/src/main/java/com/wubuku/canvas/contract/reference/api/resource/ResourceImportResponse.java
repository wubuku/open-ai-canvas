package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "ResourceImportResponse", description = "Typed URL-import resource response payload.")
public record ResourceImportResponse(
        @Schema(description = "Imported or replayed ready resource") ResourceDto resource) {
}
