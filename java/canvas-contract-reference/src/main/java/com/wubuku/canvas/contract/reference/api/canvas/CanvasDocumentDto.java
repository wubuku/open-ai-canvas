package com.wubuku.canvas.contract.reference.api.canvas;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.wubuku.canvas.contract.reference.api.common.OpaqueJsonValue;
import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.Valid;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import java.util.List;

@Schema(name = "CanvasDocument", description = "Strongly typed canvas document contract. Client-owned node metadata is isolated behind named extension fields.")
@JsonIgnoreProperties(ignoreUnknown = true)
public record CanvasDocumentDto(
        @NotBlank @Schema(description = "Stable canvas identity", example = "canvas-123") String id,
        @NotBlank @Schema(description = "Human-readable canvas title", example = "Storyboard") String title,
        @Min(0) @Schema(description = "Client base revision for writes; server responses contain a positive revision", example = "1") Long revision,
        @Schema(description = "Optional workbench project identity") String projectId,
        @NotNull @Valid @Schema(description = "Typed canvas nodes") List<CanvasNodeDto> nodes,
        @NotNull @Valid @Schema(description = "Typed canvas connections") List<CanvasConnectionDto> connections,
        @Valid @Schema(description = "Canvas assistant conversation sessions; omitted legacy values are normalized to an empty list") List<CanvasAssistantSessionDto> chatSessions,
        @Schema(description = "Currently selected assistant conversation session") String activeChatId,
        @Schema(description = "Viewport state owned by the canvas client") CanvasViewportDto viewport,
        @Schema(ref = "#/components/schemas/OpaqueJsonValue", description = "Client-owned document metadata extension") OpaqueJsonValue metadata,
        @Schema(ref = "#/components/schemas/OpaqueJsonValue", description = "Client-owned extension payload") OpaqueJsonValue customExtension,
        @Schema(description = "Client-computed content hash") String remoteContentHash,
        @Schema(description = "Server creation timestamp") String createdAt,
        @Schema(description = "Server modification timestamp") String updatedAt,
        @Schema(description = "Number of nodes in this document") Integer nodeCount,
        @Schema(description = "Small typed preview used by list responses") List<CanvasNodeDto> previewNodes) {

    public CanvasDocumentDto {
        if (chatSessions == null) {
            chatSessions = List.of();
        }
    }
}
