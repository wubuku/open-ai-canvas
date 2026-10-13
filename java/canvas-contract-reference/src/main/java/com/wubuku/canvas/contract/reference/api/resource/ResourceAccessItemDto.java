package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "ResourceAccessItem", description = "One resource access result.")
public record ResourceAccessItemDto(
        @Schema(description = "Requested resource identity") String resourceId,
        @Schema(description = "Access capability; null when the resource is not available") ResourceAccessDto access) {
}
