package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.constraints.NotBlank;

@Schema(name = "ResourceAccessRequest", description = "Request a deterministic mock access capability for one resource.")
public record ResourceAccessRequest(
        @NotBlank @Schema(description = "Resource identity") String resourceId,
        @Schema(description = "Access purpose") String purpose,
        @Schema(description = "Requested variant", example = "original") String variant) {
}
