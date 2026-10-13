package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "ResourceUploadChunkResponse", description = "Acknowledgement for an accepted upload chunk.")
public record ResourceUploadChunkResponse(
        @Schema(description = "Accepted zero-based chunk index", requiredMode = Schema.RequiredMode.REQUIRED) int index,
        @Schema(description = "Accepted chunk byte count", requiredMode = Schema.RequiredMode.REQUIRED) long size) {
}
