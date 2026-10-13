package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "CanvasShare", description = "Public share capability attached to one canvas.")
public record CanvasShareDto(
        @Schema(description = "Whether the share is currently usable", requiredMode = Schema.RequiredMode.REQUIRED)
        boolean enabled,
        @Schema(description = "Opaque public share token; empty when disabled", requiredMode = Schema.RequiredMode.REQUIRED)
        String token,
        @Schema(description = "UTC expiry timestamp; null means no expiry")
        String expiresAt,
        @Schema(description = "Share creation timestamp; null when disabled")
        String createdAt) {
}
