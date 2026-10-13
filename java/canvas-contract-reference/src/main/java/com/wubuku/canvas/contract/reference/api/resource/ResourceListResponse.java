package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;
import java.util.List;

@Schema(name = "ResourceListResponse", description = "Typed list of resources owned by the current user.")
public record ResourceListResponse(
        @Schema(description = "Owned resources") List<ResourceDto> resources) {
}
