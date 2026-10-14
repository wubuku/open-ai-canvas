package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.constraints.NotBlank;

@Schema(name = "CanvasAssistantPendingBackendSession", description = "A typed pending backend conversation marker.")
public record CanvasAssistantPendingBackendSessionDto(
        @NotBlank @Schema(description = "Backend conversation identity") String id,
        @NotBlank @Schema(description = "Backend conversation kind", allowableValues = {"cinematic"}) String kind,
        @NotBlank @Schema(description = "Message that initiated the backend conversation") String messageId,
        @NotBlank @Schema(description = "Pending operation state", allowableValues = {"pending"}) String status,
        @NotBlank @Schema(description = "Pending operation start timestamp") String startedAt) {
}
