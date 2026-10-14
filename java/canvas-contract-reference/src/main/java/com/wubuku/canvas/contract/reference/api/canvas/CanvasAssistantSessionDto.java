package com.wubuku.canvas.contract.reference.api.canvas;

import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import java.util.List;

@Schema(name = "CanvasAssistantSession", description = "Typed assistant conversation state persisted with a canvas document.")
public record CanvasAssistantSessionDto(
        @NotBlank @Schema(description = "Stable conversation identity") String id,
        @NotBlank @Schema(description = "Conversation title") String title,
        @NotNull @Valid @Schema(description = "Ordered conversation messages") List<CanvasAssistantMessageDto> messages,
        @Valid @Schema(description = "An in-progress backend conversation reference") CanvasAssistantPendingBackendSessionDto pendingBackendSession,
        @Schema(description = "Generation effect keys associated with this conversation") List<String> generationEffectKeys,
        @NotBlank @Schema(description = "Conversation creation timestamp") String createdAt,
        @NotBlank @Schema(description = "Conversation update timestamp") String updatedAt) {
}
