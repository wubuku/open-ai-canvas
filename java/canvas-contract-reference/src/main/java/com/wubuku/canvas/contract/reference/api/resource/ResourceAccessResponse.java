package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;
import java.util.List;

@Schema(name = "ResourceAccessResponse", description = "Typed batch resource access response.")
public record ResourceAccessResponse(
        @Schema(description = "One result per requested resource") List<ResourceAccessItemDto> items) {
}
