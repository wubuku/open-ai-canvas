package com.wubuku.canvas.contract.reference.api.common;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "ErrorResponse", description = "Stable machine-readable error data. The reference stub currently returns null data and reason on the envelope.")
public record ErrorResponseDto(String reason, String message) {
}
