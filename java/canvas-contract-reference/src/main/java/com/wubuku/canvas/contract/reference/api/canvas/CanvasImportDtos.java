package com.wubuku.canvas.contract.reference.api.canvas;

import com.wubuku.canvas.contract.reference.api.common.OpaqueJsonValue;
import io.swagger.v3.oas.annotations.media.Schema;
import java.util.List;

public final class CanvasImportDtos {
    private CanvasImportDtos() {
    }

    @Schema(name = "CanvasLibTvImportRequest", description = "Request to convert one controlled LibTV project into canvas nodes.")
    public record LibTvImportRequest(
            @Schema(description = "LibTV project UUID") String uuid) {
    }

    @Schema(name = "CanvasTapNowImportRequest", description = "Request to convert one controlled TapNow share into canvas nodes.")
    public record TapNowImportRequest(
            @Schema(description = "TapNow share identity") String shareId) {
    }

    @Schema(name = "CanvasImportNode", description = "Typed, non-persisted node produced by a canvas import conversion.")
    public record ImportNodeDto(
            @Schema(description = "New conversion node identity") String id,
            @Schema(description = "Normalized canvas node type") String type,
            @Schema(description = "Displayed node title") String title,
            @Schema(description = "Normalized node content or media URL") String content,
            @Schema(description = "Horizontal position") double x,
            @Schema(description = "Vertical position") double y,
            @Schema(description = "Node width") double width,
            @Schema(description = "Node height") double height,
            @Schema(description = "Normalized generation prompt") String prompt,
            @Schema(description = "Normalized model") String model,
            @Schema(description = "Normalized image size") String size,
            @Schema(description = "Normalized image quality") String quality,
            @Schema(description = "Normalized duration string") String seconds,
            @Schema(description = "Normalized video quality") String vquality,
            @Schema(description = "Normalized audio generation flag") String generateAudio,
            @Schema(description = "Detected natural media width") int naturalWidth,
            @Schema(description = "Detected natural media height") int naturalHeight,
            @Schema(description = "Detected media duration in milliseconds") long durationMs,
            @Schema(description = "Detected media MIME type") String mimeType,
            @Schema(description = "Conversion status") String status,
            @Schema(description = "Historical task failure details") String errorDetails,
            @Schema(ref = "#/components/schemas/OpaqueJsonValue", description = "Explicit import provenance extension") OpaqueJsonValue metadata) {
    }

    @Schema(name = "CanvasImportConnection", description = "Typed connection between converted import nodes.")
    public record ImportConnectionDto(
            @Schema(description = "New conversion connection identity") String id,
            @Schema(description = "Converted source node identity") String fromNodeId,
            @Schema(description = "Converted target node identity") String toNodeId,
            @Schema(description = "Source handle when supplied by the provider") String fromHandleId,
            @Schema(description = "Target handle when supplied by the provider") String toHandleId) {
    }

    @Schema(name = "CanvasImportIssue", description = "Typed reason why one provider node or connection was skipped.")
    public record ImportIssueDto(
            @Schema(description = "Provider identity when available") String id,
            @Schema(description = "Provider display name when available") String name,
            @Schema(description = "Stable human-readable skip reason") String reason) {
    }

    @Schema(name = "CanvasImportWarning", description = "Typed non-fatal conversion warning.")
    public record ImportWarningDto(
            @Schema(description = "Provider identity when available") String id,
            @Schema(description = "Stable human-readable warning") String message) {
    }

    @Schema(name = "CanvasImportResponse", description = "Typed, non-persisted canvas import conversion result.")
    public record ImportResponseDto(
            @Schema(description = "Unique conversion batch identity") String batchId,
            @Schema(description = "Conversion creation timestamp") String batchCreatedAt,
            @Schema(description = "Source LibTV project UUID") String projectUuid,
            @Schema(description = "Source TapNow share identity") String shareId,
            @Schema(description = "Source project display name") String projectName,
            @Schema(description = "Converted nodes") List<ImportNodeDto> nodes,
            @Schema(description = "Converted connections") List<ImportConnectionDto> connections,
            @Schema(description = "Number of converted nodes") int importedNodeCount,
            @Schema(description = "Number of converted connections") int importedConnectionCount,
            @Schema(description = "Skipped provider nodes") List<ImportIssueDto> skippedNodes,
            @Schema(description = "Skipped provider connections") List<ImportIssueDto> skippedConnections,
            @Schema(description = "Non-fatal conversion warnings") List<ImportWarningDto> warnings,
            @Schema(description = "Nodes containing multiple provider results") int multiResultNodeCount,
            @Schema(description = "Nodes reusing a historical failed result") int reusedFailedNodeCount,
            @Schema(description = "Placeholder nodes created for failed tasks") int placeholderNodeCount,
            @Schema(description = "Provider-specific special conversions") int convertedSpecialCount,
            @Schema(description = "Stale source nodes retained as historical results") int staleNodeCount) {
    }
}
