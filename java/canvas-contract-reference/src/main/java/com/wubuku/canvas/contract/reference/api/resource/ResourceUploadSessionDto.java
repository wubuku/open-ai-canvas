package com.wubuku.canvas.contract.reference.api.resource;

import io.swagger.v3.oas.annotations.media.Schema;

@Schema(name = "ResourceUploadSession", description = "A reserved, resumable mock resource upload session.")
public record ResourceUploadSessionDto(
        @Schema(description = "Upload session identity", requiredMode = Schema.RequiredMode.REQUIRED) String uploadId,
        @Schema(description = "Original file name", requiredMode = Schema.RequiredMode.REQUIRED) String fileName,
        @Schema(description = "Declared resource kind", requiredMode = Schema.RequiredMode.REQUIRED) String kind,
        @Schema(description = "Declared total byte count", requiredMode = Schema.RequiredMode.REQUIRED) long size,
        @Schema(description = "Fixed chunk size in bytes", requiredMode = Schema.RequiredMode.REQUIRED) int chunkSize,
        @Schema(description = "Number of chunks expected", requiredMode = Schema.RequiredMode.REQUIRED) int chunkCount) {
}
