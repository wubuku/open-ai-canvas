package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.constraints.NotBlank;

@Schema(name = "CanvasAssistantReference", description = "Typed reference to a canvas entity included in an assistant message.")
public record CanvasAssistantReferenceDto(
        @NotBlank @Schema(description = "Referenced node identity") String id,
        @NotBlank @Schema(description = "Referenced canvas node kind") String type,
        @NotBlank @Schema(description = "Referenced node title") String title,
        @Schema(description = "Optional inline image data") String dataUrl,
        @Schema(description = "Optional durable resource key") String storageKey,
        @Schema(description = "Optional referenced text") String text) {
}
