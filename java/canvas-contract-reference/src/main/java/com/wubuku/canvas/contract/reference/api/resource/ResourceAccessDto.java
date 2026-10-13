package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "ResourceAccess", description = "Short-lived local signed-access reference.")
public record ResourceAccessDto(
        @Schema(description = "Requested resource identity") String resourceId,
        @Schema(description = "Requested variant") String requestedVariant,
        @Schema(description = "Variant actually served") String actualVariant,
        @Schema(description = "Local public resource path") String url,
        @Schema(description = "Delivery mode") String delivery,
        @Schema(description = "Capability issue timestamp") String issuedAt,
        @Schema(description = "Capability expiry timestamp") String expiresAt,
        @Schema(description = "Recommended refresh timestamp") String refreshAt,
        @Schema(description = "Resource revision bound to the capability") String revision) {
}
