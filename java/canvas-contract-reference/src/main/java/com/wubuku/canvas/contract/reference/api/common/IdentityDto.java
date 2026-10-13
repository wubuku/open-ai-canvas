package com.wubuku.canvas.contract.reference.api.common;

import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.constraints.NotBlank;

@Schema(name = "Identity", description = "Stable resource identity response.")
public record IdentityDto(@NotBlank @Schema(description = "Stable resource identity") String id) {
}
