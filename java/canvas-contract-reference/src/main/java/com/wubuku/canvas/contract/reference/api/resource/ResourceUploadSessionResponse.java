package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "ResourceUploadSessionResponse", description = "Upload session response payload.")
public record ResourceUploadSessionResponse(
        @Schema(description = "Reserved upload session") ResourceUploadSessionDto upload) {
}
