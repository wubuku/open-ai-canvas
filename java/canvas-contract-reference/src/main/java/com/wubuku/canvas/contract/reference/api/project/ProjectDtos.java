package com.wubuku.canvas.contract.reference.api.project;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasSummaryDto;
import com.wubuku.canvas.contract.reference.api.common.OpaqueJsonValue;
import io.swagger.v3.oas.annotations.media.Schema;
import java.util.List;

public final class ProjectDtos {
    private ProjectDtos() {
    }

    @Schema(name = "Project", description = "Typed project aggregate reference used by the contract stub.")
    @JsonIgnoreProperties(ignoreUnknown = true)
    public record ProjectDto(
            String id,
            String userId,
            String name,
            String type,
            String aspectRatio,
            String sourceType,
            String description,
            String status,
            long revision,
            String createdAt,
            String updatedAt) {
    }

    @Schema(name = "ProjectSummary", description = "Typed project list summary.")
    public record ProjectSummaryDto(
            ProjectDto project,
            int canvasCount,
            long assetCount,
            int unitCount,
            int completedUnitCount) {
    }

    @Schema(name = "ProjectListResponse", description = "Paginated project summaries.")
    public record ProjectListResponse(
            List<ProjectSummaryDto> projects,
            Integer page,
            Integer pageSize,
            Long total,
            Boolean hasMore) {
    }

    @Schema(name = "ProjectResponse", description = "Project create/read response.")
    public record ProjectResponse(ProjectDto project) {
    }

    @Schema(name = "ProjectUpdateRequest", description = "Typed mutable project fields used by the workbench.")
    public record ProjectUpdateRequest(
            String name,
            String type,
            String aspectRatio,
            String sourceType,
            String description,
            String status) {
    }

    @Schema(name = "ProjectOverviewMetrics", description = "Typed project overview counters.")
    public record ProjectOverviewMetricsDto(
            int unitCount,
            int completedUnitCount,
            int totalWordCount,
            int canvasCount,
            int assetCount,
            int shotCount) {
    }

    @Schema(name = "ProjectOverviewUnit", description = "Typed unit wrapper in the project overview.")
    public record ProjectOverviewUnitDto(ProjectUnitDto unit) {
    }

    @Schema(name = "ProjectOverviewResponse", description = "Typed project overview projection.")
    public record ProjectOverviewResponse(
            ProjectOverviewMetricsDto metrics,
            List<ProjectOverviewUnitDto> units) {
    }

    @Schema(name = "ProjectCoreResponse", description = "Minimal typed project core response.")
    public record ProjectCoreResponse(ProjectDto project) {
    }

    @Schema(name = "ProjectUnit", description = "Typed project chapter/unit.")
    public record ProjectUnitDto(
            String id,
            String projectId,
            String parentId,
            String kind,
            String title,
            String sourceText,
            int wordCount,
            String status,
            int position,
            String createdAt,
            String updatedAt) {
    }

    @Schema(name = "ProjectUnitResponse", description = "Single project unit response.")
    public record ProjectUnitResponse(ProjectUnitDto unit) {
    }

    @Schema(name = "ProjectUnitListResponse", description = "Project unit list response.")
    public record ProjectUnitListResponse(List<ProjectUnitDto> units) {
    }

    @Schema(name = "ProjectUnitWorkspaceResponse", description = "Typed unit workspace projection with named extension collections.")
    public record ProjectUnitWorkspaceResponse(
            ProjectUnitDto unit,
            List<OpaqueJsonValue> workflows,
            List<OpaqueJsonValue> shots,
            List<OpaqueJsonValue> assets,
            List<OpaqueJsonValue> tasks) {
    }

    @Schema(name = "ProjectUnitImportResponse", description = "Batch-created typed project unit summaries.")
    public record ProjectUnitImportResponse(List<ProjectUnitDto> units) {
    }

    @Schema(name = "ProjectUnitReorderResponse", description = "The committed typed unit ordering.")
    public record ProjectUnitReorderResponse(List<String> unitIds) {
    }

    @Schema(name = "ProjectUnitUpdateRequest", description = "Typed mutable project unit fields.")
    public record ProjectUnitUpdateRequest(
            String kind,
            String title,
            String sourceText,
            String status) {
    }

    @Schema(name = "ProjectUnitMutationResponse", description = "Single typed project unit mutation response.")
    public record ProjectUnitMutationResponse(ProjectUnitDto unit) {
    }

    @Schema(name = "ProjectDeleteResponse", description = "Project deletion acknowledgement.")
    public record ProjectDeleteResponse(String projectId) {
    }

    @Schema(name = "ProjectUnitDeleteResponse", description = "Project unit deletion acknowledgement.")
    public record ProjectUnitDeleteResponse(String projectId, String unitId) {
    }

    @Schema(name = "CanvasUnitLink", description = "Typed association between one project unit and one canvas.")
    public record CanvasUnitLinkDto(
            String id,
            String projectId,
            String canvasId,
            String unitId,
            String role,
            String createdAt) {
    }

    @Schema(name = "CanvasUnitLinkResponse", description = "Canvas association acknowledgement.")
    public record CanvasUnitLinkResponse(CanvasUnitLinkDto link) {
    }

    @Schema(name = "CanvasUnlinkUnitResponse", description = "Unit-level canvas unlink acknowledgement.")
    public record CanvasUnlinkUnitResponse(String canvasId, String unitId) {
    }

    @Schema(name = "CanvasUnlinkResponse", description = "Project-level canvas unlink acknowledgement.")
    public record CanvasUnlinkResponse(String canvasId) {
    }

    @Schema(name = "ProjectCanvasPageResponse", description = "Typed project canvas page with unit associations.")
    public record ProjectCanvasPageResponse(
            List<ProjectCanvasPageSummaryDto> canvases,
            List<CanvasUnitLinkDto> canvasUnitLinks,
            int page,
            int pageSize,
            long total,
            boolean hasMore) {
    }

    @Schema(name = "ProjectCanvasPageSummary", description = "The observed project page canvas projection; document revision is intentionally not authoritative here.")
    public record ProjectCanvasPageSummaryDto(
            String id,
            String title,
            long revision,
            String createdAt,
            String updatedAt) {
    }

    @Schema(name = "ProjectDetailResponse", description = "Typed project detail projection used by association tests.")
    public record ProjectDetailResponse(
            ProjectDto project,
            List<ProjectUnitDto> units,
            List<CanvasSummaryDto> canvases,
            List<CanvasUnitLinkDto> canvasUnitLinks,
            List<OpaqueJsonValue> assets,
            List<OpaqueJsonValue> assetFolders,
            List<OpaqueJsonValue> workflows,
            List<OpaqueJsonValue> shots,
            List<OpaqueJsonValue> shotRevisions,
            List<OpaqueJsonValue> shotArtifacts,
            List<OpaqueJsonValue> shotReferences,
            List<OpaqueJsonValue> assetCandidates,
            List<OpaqueJsonValue> tasks) {
    }
}
