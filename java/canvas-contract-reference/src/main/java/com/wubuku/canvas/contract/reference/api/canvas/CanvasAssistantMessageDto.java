package com.wubuku.canvas.contract.reference.api.canvas;

import com.wubuku.canvas.contract.reference.api.common.OpaqueJsonValue;
import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import java.util.List;

@Schema(name = "CanvasAssistantMessage", description = "Typed assistant conversation message stored in a canvas document.")
public record CanvasAssistantMessageDto(
        @NotBlank @Schema(description = "Stable message identity") String id,
        @NotBlank @Schema(description = "Message role", allowableValues = {"user", "assistant", "system", "tool", "error"}) String role,
        @Schema(description = "Optional message title") String title,
        @Schema(description = "Message text") String text,
        @Schema(description = "Optional plain-text message metadata") String meta,
        @Schema(ref = "#/components/schemas/OpaqueJsonValue", description = "Client-owned structured message detail") OpaqueJsonValue detail,
        @Valid @Schema(description = "Canvas entities referenced by this message") List<CanvasAssistantReferenceDto> references) {
}
