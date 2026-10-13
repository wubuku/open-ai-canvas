package com.wubuku.canvas.contract.reference.api.project;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasSummaryDto;
import com.wubuku.canvas.contract.reference.api.common.IdentityDto;
import com.wubuku.canvas.contract.reference.api.common.ImportBodyBudgetFilter;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ApiEnvelope;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.CanvasLinkRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ProjectRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ReorderRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.UnitImportRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.UnitRequest;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.CanvasUnlinkResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.CanvasUnlinkUnitResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.CanvasUnitLinkDto;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.CanvasUnitLinkResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectCanvasPageResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectCanvasPageSummaryDto;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectCoreResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectDetailResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectDto;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectDeleteResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectOverviewMetricsDto;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectOverviewResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectOverviewUnitDto;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectUnitDeleteResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectUnitDto;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectUnitImportResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectUnitListResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectUnitMutationResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectUnitReorderResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectUnitResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectUnitUpdateRequest;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectUnitWorkspaceResponse;
import com.wubuku.canvas.contract.reference.api.project.ProjectDtos.ProjectUpdateRequest;
import com.wubuku.canvas.contract.reference.contract.ApiException;
import com.wubuku.canvas.contract.reference.contract.ReferenceService;
import com.wubuku.canvas.contract.reference.contract.ReferenceStore;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.Parameter;
import io.swagger.v3.oas.annotations.media.Content;
import io.swagger.v3.oas.annotations.media.Schema;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.tags.Tag;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.Valid;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PatchMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api")
@Tag(name = "Reference projects", description = "Typed mock project, unit and canvas association endpoints.")
public class ProjectController {
    private final ReferenceService service;
    private final ReferenceStore store;
    private final ConcurrentHashMap<String, Boolean> firstAssociationCreates = new ConcurrentHashMap<>();

    public ProjectController(ReferenceService service, ReferenceStore store) {
        this.service = service;
        this.store = store;
    }

    @PostMapping("/projects")
    @Operation(operationId = "reference.project.create", summary = "Create a typed mock project")
    @ApiResponse(responseCode = "200", description = "Created project", content = @Content(schema = @Schema(implementation = ProjectResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectResponse>> createProject(HttpServletRequest request, @Valid @RequestBody ProjectRequest body) {
        String userId = requireProjectAccess(request);
        String name = body.name().trim();
        ProjectDto project = store.update(root -> {
            for (JsonNode existing : root.with("projects")) {
                if (userId.equals(ReferenceService.text(existing, "ownerId"))
                        && !ReferenceService.bool(existing, "deleted")
                        && name.equals(ReferenceService.text(existing, "name"))) {
                    throw new ApiException(409, 409, "project_name_conflict", "project name already exists");
                }
            }
            String id = newId("project");
            String now = Instant.now().toString();
            ObjectNode value = root.with("projects").putObject(id);
            value.put("id", id).put("userId", userId).put("ownerId", userId)
                    .put("name", name)
                    .put("type", nonBlank(body.type(), "short-drama"))
                    .put("aspectRatio", nonBlank(body.aspectRatio(), "16:9"))
                    .put("sourceType", nonBlank(body.sourceType(), "original"))
                    .put("description", "")
                    .put("status", "draft")
                    .put("revision", 1)
                    .put("createdAt", now)
                    .put("updatedAt", now);
            value.set("units", JsonNodeFactory.instance.arrayNode());
            value.set("canvasUnitLinks", JsonNodeFactory.instance.arrayNode());
            return projectDto(value);
        });
        return ResponseEntity.ok(ApiEnvelope.success(new ProjectResponse(project)));
    }

    @GetMapping("/projects")
    @Operation(operationId = "reference.project.list", summary = "List typed mock projects")
    @ApiResponse(responseCode = "200", description = "Project list", content = @Content(schema = @Schema(implementation = ProjectDtos.ProjectListResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectDtos.ProjectListResponse>> listProjects(
            HttpServletRequest request,
            @RequestParam(required = false) Integer page,
            @RequestParam(required = false) Integer pageSize) {
        String userId = requireProjectAccess(request);
        List<ProjectSummary> summaries = projectSummaries(store.snapshot(), userId);
        if (page == null && pageSize == null) {
            return ResponseEntity.ok(ApiEnvelope.success(new ProjectDtos.ProjectListResponse(
                    summaries.stream().map(ProjectSummary::dto).toList(), null, null, null, null)));
        }
        int currentPage = positive(page, 1);
        int limit = Math.min(positive(pageSize, 40), 50);
        int from = Math.min((currentPage - 1) * limit, summaries.size());
        int to = Math.min(from + limit, summaries.size());
        return ResponseEntity.ok(ApiEnvelope.success(new ProjectDtos.ProjectListResponse(
                summaries.subList(from, to).stream().map(ProjectSummary::dto).toList(),
                currentPage, limit, (long) summaries.size(), to < summaries.size())));
    }

    @GetMapping("/projects/{id}")
    @Operation(operationId = "reference.project.read", summary = "Read a typed mock project detail")
    @ApiResponse(responseCode = "200", description = "Project detail", content = @Content(schema = @Schema(implementation = ProjectDetailResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectDetailResponse>> readProject(HttpServletRequest request, @PathVariable String id) {
        String userId = requireProjectAccess(request);
        ObjectNode root = store.snapshot();
        ObjectNode project = requireProject(root, id, userId);
        return ResponseEntity.ok(ApiEnvelope.success(projectDetail(root, project, userId)));
    }

    @GetMapping("/projects/{id}/core")
    @Operation(operationId = "reference.project.core", summary = "Read the typed project core")
    @ApiResponse(responseCode = "200", description = "Project core", content = @Content(schema = @Schema(implementation = ProjectCoreResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectCoreResponse>> core(HttpServletRequest request, @PathVariable String id) {
        String userId = requireProjectAccess(request);
        ObjectNode project = requireProject(store.snapshot(), id, userId);
        return ResponseEntity.ok(ApiEnvelope.success(new ProjectCoreResponse(projectDto(project))));
    }

    @PatchMapping("/projects/{id}")
    @Operation(operationId = "reference.project.update", summary = "Update typed mock project metadata")
    @ApiResponse(responseCode = "200", description = "Updated project", content = @Content(schema = @Schema(implementation = ProjectResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectResponse>> updateProject(
            HttpServletRequest request,
            @PathVariable String id,
            @RequestBody ProjectUpdateRequest body) {
        String userId = requireProjectAccess(request);
        ProjectDto project = store.update(root -> {
            ObjectNode value = requireProject(root, id, userId);
            if (body.name() != null) {
                String name = body.name().trim();
                if (name.isBlank()) {
                    throw new ApiException(400, 400, "bad_request", "project name is required");
                }
                for (JsonNode existing : root.with("projects")) {
                    if (!id.equals(ReferenceService.text(existing, "id"))
                            && userId.equals(ReferenceService.text(existing, "ownerId"))
                            && !ReferenceService.bool(existing, "deleted")
                            && name.equals(ReferenceService.text(existing, "name"))) {
                        throw new ApiException(409, 409, "project_name_conflict", "project name already exists");
                    }
                }
                value.put("name", name);
            }
            if (body.type() != null) {
                value.put("type", body.type().trim());
            }
            if (body.aspectRatio() != null) {
                value.put("aspectRatio", body.aspectRatio().trim());
            }
            if (body.sourceType() != null) {
                value.put("sourceType", body.sourceType().trim());
            }
            if (body.description() != null) {
                value.put("description", body.description());
            }
            if (body.status() != null) {
                String status = body.status().trim();
                if (!validStatus(status)) {
                    throw new ApiException(400, 400, "bad_request", "invalid project status");
                }
                value.put("status", status);
            }
            bumpProject(value);
            return projectDto(value);
        });
        return ResponseEntity.ok(ApiEnvelope.success(new ProjectResponse(project)));
    }

    @GetMapping("/projects/{id}/overview")
    @Operation(operationId = "reference.project.overview", summary = "Read typed project overview metrics")
    @ApiResponse(responseCode = "200", description = "Project overview", content = @Content(schema = @Schema(implementation = ProjectOverviewResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectOverviewResponse>> overview(HttpServletRequest request, @PathVariable String id) {
        String userId = requireProjectAccess(request);
        ObjectNode root = store.snapshot();
        ObjectNode project = requireProjectOverview(root, id, userId);
        List<ProjectOverviewUnitDto> units = new ArrayList<>();
        int completed = 0;
        int totalWords = 0;
        for (JsonNode unit : project.withArray("units")) {
            ProjectUnitDto dto = projectUnitDto(unit);
            units.add(new ProjectOverviewUnitDto(dto));
            completed += "completed".equals(dto.status()) ? 1 : 0;
            totalWords += dto.wordCount();
        }
        int canvasCount = currentCanvases(root, id, userId).size();
        ProjectOverviewMetricsDto metrics = new ProjectOverviewMetricsDto(
                units.size(), completed, totalWords, canvasCount, 0, 0);
        return ResponseEntity.ok(ApiEnvelope.success(new ProjectOverviewResponse(metrics, units)));
    }

    @PostMapping("/projects/{id}/units")
    @Operation(operationId = "reference.project.unit.create", summary = "Create a typed mock project unit")
    @ApiResponse(responseCode = "200", description = "Created unit", content = @Content(schema = @Schema(implementation = ProjectUnitResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectUnitResponse>> createUnit(
            HttpServletRequest request,
            @PathVariable String id,
            @Valid @RequestBody UnitRequest body) {
        String userId = requireProjectAccess(request);
        ProjectUnitDto unit = store.update(root -> {
            ObjectNode project = requireProject(root, id, userId);
            String title = body.title() == null ? "" : body.title().trim();
            if (title.isBlank()) {
                throw new ApiException(400, 400, "bad_request", "unit title is required");
            }
            validateUnitKind(body.kind());
            validateUnitTitle(title);
            ArrayNode units = project.withArray("units");
            String unitId = newId("unit");
            String now = Instant.now().toString();
            ObjectNode value = units.addObject();
            value.put("id", unitId).put("projectId", id)
                    .put("kind", nonBlank(body.kind(), "chapter"))
                    .put("title", title)
                    .put("sourceText", body.sourceText() == null ? "" : body.sourceText())
                    .put("wordCount", wordCount(body.sourceText()))
                    .put("status", "draft")
                    .put("position", units.size() - 1)
                    .put("createdAt", now).put("updatedAt", now);
            bumpProject(project);
            return projectUnitDto(value);
        });
        return ResponseEntity.ok(ApiEnvelope.success(new ProjectUnitResponse(unit)));
    }

    @GetMapping("/projects/{id}/units")
    @Operation(operationId = "reference.project.unit.list", summary = "List typed project units")
    @ApiResponse(responseCode = "200", description = "Project units", content = @Content(schema = @Schema(implementation = ProjectUnitListResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectUnitListResponse>> listUnits(HttpServletRequest request, @PathVariable String id) {
        String userId = requireProjectAccess(request);
        ObjectNode project = requireProjectList(store.snapshot(), id, userId);
        return ResponseEntity.ok(ApiEnvelope.success(new ProjectUnitListResponse(unitSummaries(project))));
    }

    @GetMapping("/projects/{id}/units/{unitId}")
    @Operation(operationId = "reference.project.unit.read", summary = "Read one typed project unit")
    @ApiResponse(responseCode = "200", description = "Project unit", content = @Content(schema = @Schema(implementation = ProjectUnitResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectUnitResponse>> readUnit(
            HttpServletRequest request,
            @PathVariable String id,
            @PathVariable String unitId) {
        String userId = requireProjectAccess(request);
        ObjectNode project = requireProject(store.snapshot(), id, userId);
        JsonNode unit = findUnit(project, unitId);
        if (unit == null) {
            throw new ApiException(404, 404, "not_found", "unit not found");
        }
        return ResponseEntity.ok(ApiEnvelope.success(new ProjectUnitResponse(projectUnitDto(unit))));
    }

    @GetMapping("/projects/{id}/units/{unitId}/workspace")
    @Operation(operationId = "reference.project.unit.workspace", summary = "Read a typed unit workspace projection")
    @ApiResponse(responseCode = "200", description = "Unit workspace", content = @Content(schema = @Schema(implementation = ProjectUnitWorkspaceResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectUnitWorkspaceResponse>> workspace(
            HttpServletRequest request,
            @PathVariable String id,
            @PathVariable String unitId) {
        String userId = requireProjectAccess(request);
        ObjectNode project = requireProjectOverview(store.snapshot(), id, userId);
        JsonNode unit = findUnit(project, unitId);
        if (unit == null) {
            throw new ApiException(500, 500, "internal", "internal");
        }
        return ResponseEntity.ok(ApiEnvelope.success(new ProjectUnitWorkspaceResponse(
                projectUnitDto(unit), List.of(), List.of(), List.of(), List.of())));
    }

    @PostMapping("/projects/{id}/units/import")
    @Operation(operationId = "reference.project.unit.import", summary = "Import a typed batch of mock project units")
    @ApiResponse(responseCode = "200", description = "Imported units", content = @Content(schema = @Schema(implementation = ProjectUnitImportResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectUnitImportResponse>> importUnits(
            HttpServletRequest request,
            @PathVariable String id,
            @Valid @RequestBody UnitImportRequest body) {
        String userId = requireProjectAccess(request);
        requireProject(store.snapshot(), id, userId);
        if (Boolean.TRUE.equals(request.getAttribute(ImportBodyBudgetFilter.OVERSIZE_ATTRIBUTE))) {
            throw new ApiException(400, 400, "bad_request", "import body exceeds the 32 MiB JSON value budget");
        }
        if (body.units() == null || body.units().isEmpty()) {
            throw new ApiException(400, 400, "bad_request", "units must not be empty");
        }
        List<NormalizedUnitInput> normalizedUnits = new ArrayList<>();
        for (UnitRequest bodyUnit : body.units()) {
            String title = bodyUnit.title() == null ? "" : bodyUnit.title().trim();
            validateUnitTitle(title);
            validateUnitKind(bodyUnit.kind());
            normalizedUnits.add(new NormalizedUnitInput(
                    nonBlank(bodyUnit.kind(), "chapter"),
                    title,
                    bodyUnit.sourceText() == null ? "" : bodyUnit.sourceText()));
        }
        ProjectUnitImportResponse response = store.update(root -> {
            ObjectNode project = requireProject(root, id, userId);
            ArrayNode units = project.withArray("units");
            List<ProjectUnitDto> imported = new ArrayList<>();
            for (NormalizedUnitInput input : normalizedUnits) {
                String unitId = newId("unit");
                String now = Instant.now().toString();
                ObjectNode value = units.addObject();
                value.put("id", unitId).put("projectId", id)
                        .put("kind", input.kind())
                        .put("title", input.title())
                        .put("sourceText", input.sourceText())
                        .put("wordCount", wordCount(input.sourceText()))
                        .put("status", "draft")
                        .put("position", units.size() - 1)
                        .put("createdAt", now).put("updatedAt", now);
                ProjectUnitDto dto = projectUnitDto(value);
                imported.add(new ProjectUnitDto(dto.id(), dto.projectId(), dto.parentId(), dto.kind(), dto.title(), "", dto.wordCount(), dto.status(), dto.position(), dto.createdAt(), dto.updatedAt()));
            }
            bumpProject(project);
            return new ProjectUnitImportResponse(imported);
        });
        return ResponseEntity.ok(ApiEnvelope.success(response));
    }

    @PatchMapping("/projects/{id}/units/reorder")
    @Operation(operationId = "reference.project.unit.reorder", summary = "Commit a typed project unit ordering")
    @ApiResponse(responseCode = "200", description = "Reordered units", content = @Content(schema = @Schema(implementation = ProjectUnitReorderResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectUnitReorderResponse>> reorderUnits(
            HttpServletRequest request,
            @PathVariable String id,
            @Valid @RequestBody ReorderRequest body) {
        String userId = requireProjectAccess(request);
        ProjectUnitReorderResponse response = store.update(root -> {
            ObjectNode project = requireProject(root, id, userId);
            ArrayNode current = project.withArray("units");
            if (body.unitIds() == null || body.unitIds().isEmpty() || body.unitIds().size() != current.size()) {
                throw new ApiException(400, 400, "bad_request", "unitIds must contain every project unit exactly once");
            }
            Set<String> requested = new HashSet<>(body.unitIds());
            if (requested.size() != body.unitIds().size()) {
                throw new ApiException(400, 400, "bad_request", "unitIds must be unique");
            }
            java.util.Map<String, JsonNode> byId = new java.util.HashMap<>();
            for (JsonNode unit : current) {
                byId.put(ReferenceService.text(unit, "id"), unit.deepCopy());
            }
            if (byId.size() != requested.size() || !byId.keySet().equals(requested)) {
                throw new ApiException(400, 400, "bad_request", "unitIds must match project units");
            }
            ArrayNode reordered = JsonNodeFactory.instance.arrayNode();
            for (int position = 0; position < body.unitIds().size(); position++) {
                ObjectNode unit = (ObjectNode) byId.get(body.unitIds().get(position));
                unit.put("position", position);
                unit.put("updatedAt", Instant.now().toString());
                reordered.add(unit);
            }
            project.set("units", reordered);
            bumpProject(project);
            return new ProjectUnitReorderResponse(body.unitIds());
        });
        return ResponseEntity.ok(ApiEnvelope.success(response));
    }

    @PatchMapping("/projects/{id}/units/{unitId}")
    @Operation(operationId = "reference.project.unit.update", summary = "Update one typed project unit")
    @ApiResponse(responseCode = "200", description = "Updated unit", content = @Content(schema = @Schema(implementation = ProjectUnitMutationResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectUnitMutationResponse>> updateUnit(
            HttpServletRequest request,
            @PathVariable String id,
            @PathVariable String unitId,
            @RequestBody ProjectUnitUpdateRequest body) {
        String userId = requireProjectAccess(request);
        ProjectUnitMutationResponse response = store.update(root -> {
            ObjectNode project = requireProject(root, id, userId);
            ObjectNode unit = (ObjectNode) findUnit(project, unitId);
            if (unit == null) {
                throw new ApiException(404, 404, "not_found", "unit not found");
            }
            if (body.kind() != null) {
                String kind = body.kind().trim();
                validateUnitKind(kind);
                unit.put("kind", kind);
            }
            if (body.title() != null) {
                String title = body.title().trim();
                if (title.isBlank() || title.length() > 240) {
                    throw new ApiException(400, 400, "bad_request", "unit title is invalid");
                }
                unit.put("title", title);
            }
            if (body.sourceText() != null) {
                unit.put("sourceText", body.sourceText());
                unit.put("wordCount", wordCount(body.sourceText()));
            }
            if (body.status() != null) {
                String status = body.status().trim();
                if (!validStatus(status)) {
                    throw new ApiException(400, 400, "bad_request", "invalid unit status");
                }
                unit.put("status", status);
            }
            unit.put("updatedAt", Instant.now().toString());
            bumpProject(project);
            return new ProjectUnitMutationResponse(projectUnitDto(unit));
        });
        return ResponseEntity.ok(ApiEnvelope.success(response));
    }

    @DeleteMapping("/projects/{id}/units/{unitId}")
    @Operation(operationId = "reference.project.unit.delete", summary = "Delete one typed project unit")
    @ApiResponse(responseCode = "200", description = "Deleted unit", content = @Content(schema = @Schema(implementation = IdentityDto.class)))
    public ResponseEntity<ApiEnvelope<IdentityDto>> deleteUnit(
            HttpServletRequest request,
            @PathVariable String id,
            @PathVariable String unitId) {
        String userId = requireProjectAccess(request);
        store.update(root -> {
            ObjectNode project = requireProject(root, id, userId);
            ObjectNode unit = (ObjectNode) findUnit(project, unitId);
            if (unit == null) {
                throw new ApiException(404, 404, "not_found", "unit not found");
            }
            int unitIndex = unitIndex(project, unitId);
            project.withArray("units").remove(unitIndex);
            ArrayNode links = project.withArray("canvasUnitLinks");
            for (int index = links.size() - 1; index >= 0; index--) {
                if (unitId.equals(ReferenceService.text(links.get(index), "unitId"))) {
                    links.remove(index);
                }
            }
            bumpProject(project);
            return null;
        });
        return ResponseEntity.ok(ApiEnvelope.success(new IdentityDto(unitId)));
    }

    @DeleteMapping("/projects/{id}")
    @Operation(operationId = "reference.project.delete", summary = "Delete a typed mock project and detach its canvases")
    @ApiResponse(responseCode = "200", description = "Deleted project", content = @Content(schema = @Schema(implementation = IdentityDto.class)))
    public ResponseEntity<ApiEnvelope<IdentityDto>> deleteProject(HttpServletRequest request, @PathVariable String id) {
        String userId = requireProjectAccess(request);
        store.update(root -> {
            ObjectNode project = requireProject(root, id, userId);
            for (JsonNode canvasNode : root.with("canvases")) {
                ObjectNode canvas = (ObjectNode) canvasNode;
                ObjectNode document = (ObjectNode) canvas.path("project");
                if (id.equals(ReferenceService.text(document, "projectId"))
                        && userId.equals(ReferenceService.text(canvas, "ownerId"))) {
                    document.put("projectId", "");
                    bumpCanvas(canvas);
                }
            }
            project.withArray("canvasUnitLinks").removeAll();
            project.put("deleted", true);
            bumpProject(project);
            return null;
        });
        return ResponseEntity.ok(ApiEnvelope.success(new IdentityDto(id)));
    }

    @PostMapping("/projects/{id}/canvas-links")
    @Operation(operationId = "reference.project.canvas.link", summary = "Associate a typed canvas with a project unit")
    @ApiResponse(responseCode = "200", description = "Canvas association", content = @Content(schema = @Schema(implementation = CanvasUnitLinkResponse.class)))
    public ResponseEntity<ApiEnvelope<CanvasUnitLinkResponse>> linkCanvas(
            HttpServletRequest request,
            @PathVariable String id,
            @Valid @RequestBody CanvasLinkRequest body) {
        String userId = requireProjectAccess(request);
        String canvasId = body.canvasId() == null ? "" : body.canvasId().trim();
        String unitId = body.unitId() == null ? "" : body.unitId().trim();
        if (canvasId.isBlank() || unitId.isBlank()) {
            throw new ApiException(400, 400, "bad_request", "canvasId and unitId are required");
        }
        String role = body.role() == null || body.role().isBlank() ? "storyboard" : body.role().trim();
        ObjectNode before = store.snapshot();
        ObjectNode projectBefore = requireProject(before, id, userId);
        if (findUnit(projectBefore, unitId) == null) {
            throw new ApiException(404, 404, "not_found", "unit not found");
        }
        JsonNode canvasBefore = before.with("canvases").path(canvasId);
        if (!canvasBefore.isObject() || !userId.equals(ReferenceService.text(canvasBefore, "ownerId")) || ReferenceService.bool(canvasBefore, "deleted")) {
            throw new ApiException(404, 404, "not_found", "canvas not found");
        }
        String associationKey = id + ":" + canvasId + ":" + unitId;
        boolean firstAssociation = findLink(projectBefore, canvasId, unitId) == null;
        if (role.length() > 32) {
            store.update(root -> {
                ObjectNode canvas = requireOwnedCanvas(root, canvasId, userId);
                ObjectNode document = (ObjectNode) canvas.path("project");
                document.put("projectId", id);
                bumpCanvas(canvas);
                return null;
            });
            throw new ApiException(500, 500, "internal", "internal");
        }
        if (firstAssociation && firstAssociationCreates.putIfAbsent(associationKey, Boolean.TRUE) != null) {
            store.update(root -> {
                ObjectNode canvas = requireOwnedCanvas(root, canvasId, userId);
                ((ObjectNode) canvas.path("project")).put("projectId", id);
                bumpCanvas(canvas);
                return null;
            });
            throw new ApiException(500, 500, "internal", "internal");
        }
        try {
            CanvasUnitLinkDto acknowledgement = store.update(root -> {
                ObjectNode project = requireProject(root, id, userId);
                ObjectNode canvas = requireOwnedCanvas(root, canvasId, userId);
                if (findUnit(project, unitId) == null) {
                    throw new ApiException(404, 404, "not_found", "unit not found");
                }
                JsonNode existingNode = findLink(project, canvasId, unitId);
                String now = Instant.now().toString();
                CanvasUnitLinkDto result;
                boolean changed = true;
                if (existingNode != null) {
                    ObjectNode existing = (ObjectNode) existingNode;
                    existing.put("role", role);
                    result = linkDto(existing.deepCopy());
                    result = new CanvasUnitLinkDto(newId("canvas-link"), result.projectId(), result.canvasId(), result.unitId(), result.role(), result.createdAt());
                } else {
                    ObjectNode link = project.withArray("canvasUnitLinks").addObject();
                    link.put("id", newId("canvas-link")).put("projectId", id).put("canvasId", canvasId).put("unitId", unitId).put("role", role).put("createdAt", now);
                    result = linkDto(link);
                }
                if (changed) {
                    ObjectNode document = (ObjectNode) canvas.path("project");
                    int nextRevision = canvas.path("revision").asInt(1) + 1;
                    document.put("projectId", id).put("revision", nextRevision);
                    canvas.put("revision", nextRevision);
                    bumpProject(project);
                }
                return result;
            });
            return ResponseEntity.ok(ApiEnvelope.success(new CanvasUnitLinkResponse(acknowledgement)));
        } finally {
            if (firstAssociation) {
                firstAssociationCreates.remove(associationKey);
            }
        }
    }

    @GetMapping("/projects/{id}/canvases")
    @Operation(operationId = "reference.project.canvas.list", summary = "List typed canvases associated with a project")
    @ApiResponse(responseCode = "200", description = "Project canvas page", content = @Content(schema = @Schema(implementation = ProjectCanvasPageResponse.class)))
    public ResponseEntity<ApiEnvelope<ProjectCanvasPageResponse>> listCanvases(
            HttpServletRequest request,
            @PathVariable String id,
            @RequestParam(defaultValue = "1") String page,
            @RequestParam(defaultValue = "40") String pageSize) {
        String userId = requireProjectAccess(request);
        int currentPage = positive(page, 1);
        int limit = positive(pageSize, 40);
        ObjectNode root = store.snapshot();
        JsonNode candidate = root.with("projects").path(id);
        if (!candidate.isObject()) {
            throw new ApiException(404, 404, "not_found", "project not found");
        }
        if (!userId.equals(ReferenceService.text(candidate, "ownerId"))) {
            throw new ApiException(500, 500, "internal", "internal");
        }
        ObjectNode project = (ObjectNode) candidate;
        List<ProjectCanvasPageSummaryDto> canvases = currentCanvasPageSummaries(root, id, userId);
        List<CanvasUnitLinkDto> links = currentLinks(project, root, id);
        return ResponseEntity.ok(ApiEnvelope.success(new ProjectCanvasPageResponse(
                canvases, links, currentPage, limit, canvases.size(), false)));
    }

    @DeleteMapping("/projects/{id}/canvas-links/{canvasId}/units/{unitId}")
    @Operation(operationId = "reference.project.canvas.unlink-unit", summary = "Remove one typed unit association from a canvas")
    @ApiResponse(responseCode = "200", description = "Unit association removed", content = @Content(schema = @Schema(implementation = CanvasUnlinkUnitResponse.class)))
    public ResponseEntity<ApiEnvelope<CanvasUnlinkUnitResponse>> unlinkUnit(
            HttpServletRequest request,
            @PathVariable String id,
            @PathVariable String canvasId,
            @PathVariable String unitId) {
        String userId = requireProjectAccess(request);
        store.update(root -> {
            ObjectNode project = requireProject(root, id, userId);
            ObjectNode canvas = requireOwnedCanvas(root, canvasId, userId);
            if (!id.equals(ReferenceService.text(canvas.path("project"), "projectId"))) {
                throw new ApiException(400, 400, "bad_request", "canvas is not assigned to project");
            }
            ObjectNode link = findLink(project, canvasId, unitId);
            if (link == null) {
                throw new ApiException(404, 404, "not_found", "canvas unit link not found");
            }
            project.withArray("canvasUnitLinks").remove(linkIndex(project, link));
            bumpProject(project);
            return null;
        });
        return ResponseEntity.ok(ApiEnvelope.success(new CanvasUnlinkUnitResponse(canvasId, unitId)));
    }

    @DeleteMapping("/projects/{id}/canvases/{canvasId}")
    @Operation(operationId = "reference.project.canvas.unlink", summary = "Detach a canvas from a project")
    @ApiResponse(responseCode = "200", description = "Canvas detached", content = @Content(schema = @Schema(implementation = CanvasUnlinkResponse.class)))
    public ResponseEntity<ApiEnvelope<CanvasUnlinkResponse>> unlinkCanvas(
            HttpServletRequest request,
            @PathVariable String id,
            @PathVariable String canvasId) {
        String userId = requireProjectAccess(request);
        store.update(root -> {
            ObjectNode project = requireProject(root, id, userId);
            ObjectNode canvas = requireOwnedCanvas(root, canvasId, userId);
            if (!id.equals(ReferenceService.text(canvas.path("project"), "projectId"))) {
                throw new ApiException(400, 400, "bad_request", "canvas is not assigned to project");
            }
            ArrayNode links = project.withArray("canvasUnitLinks");
            for (int index = links.size() - 1; index >= 0; index--) {
                if (canvasId.equals(ReferenceService.text(links.get(index), "canvasId"))) {
                    links.remove(index);
                }
            }
            ((ObjectNode) canvas.path("project")).put("projectId", "");
            bumpCanvas(canvas);
            bumpProject(project);
            return null;
        });
        return ResponseEntity.ok(ApiEnvelope.success(new CanvasUnlinkResponse(canvasId)));
    }

    private String requireProjectAccess(HttpServletRequest request) {
        String userId = service.requireUser(request);
        if (!store.snapshot().path("features").path("shortDramaEnabled").asBoolean(true)) {
            throw new ApiException(403, 403, "forbidden", "project feature is disabled");
        }
        return userId;
    }

    private static ObjectNode requireProjectList(ObjectNode root, String id, String userId) {
        return requireProjectForeignInternal(root, id, userId);
    }

    private static ObjectNode requireProjectOverview(ObjectNode root, String id, String userId) {
        return requireProjectForeignInternal(root, id, userId);
    }

    private static ObjectNode requireProjectForeignInternal(ObjectNode root, String id, String userId) {
        JsonNode project = root.with("projects").path(id);
        if (!project.isObject() || ReferenceService.bool(project, "deleted")) {
            throw new ApiException(404, 404, "not_found", "project not found");
        }
        if (!userId.equals(ReferenceService.text(project, "ownerId"))) {
            throw new ApiException(500, 500, "internal", "internal");
        }
        return (ObjectNode) project;
    }

    private static ObjectNode requireProject(ObjectNode root, String id, String userId) {
        JsonNode project = root.with("projects").path(id);
        if (!project.isObject() || !userId.equals(ReferenceService.text(project, "ownerId")) || ReferenceService.bool(project, "deleted")) {
            throw new ApiException(404, 404, "not_found", "project not found");
        }
        return (ObjectNode) project;
    }

    private static ObjectNode requireOwnedCanvas(ObjectNode root, String id, String userId) {
        JsonNode canvas = root.with("canvases").path(id);
        if (!canvas.isObject() || !userId.equals(ReferenceService.text(canvas, "ownerId")) || ReferenceService.bool(canvas, "deleted")) {
            throw new ApiException(404, 404, "not_found", "canvas not found");
        }
        return (ObjectNode) canvas;
    }

    private ProjectDetailResponse projectDetail(ObjectNode root, ObjectNode project, String userId) {
        List<CanvasSummaryDto> canvases = currentCanvases(root, ReferenceService.text(project, "id"), userId);
        return new ProjectDetailResponse(
                projectDto(project), units(project), canvases, links(project),
                List.of(), List.of(), List.of(), List.of(), List.of(), List.of(), List.of(), List.of(), List.of());
    }

    private List<CanvasSummaryDto> currentCanvases(ObjectNode root, String projectId, String userId) {
        List<CanvasSummaryDto> result = new ArrayList<>();
        for (JsonNode canvas : root.with("canvases")) {
            JsonNode document = canvas.path("project");
            if (userId.equals(ReferenceService.text(canvas, "ownerId")) && !ReferenceService.bool(canvas, "deleted") && projectId.equals(ReferenceService.text(document, "projectId"))) {
                result.add(service.canvasSummary(document));
            }
        }
        result.sort(Comparator.comparing(CanvasSummaryDto::id));
        return result;
    }

    private List<ProjectCanvasPageSummaryDto> currentCanvasPageSummaries(ObjectNode root, String projectId, String userId) {
        List<ProjectCanvasPageSummaryDto> result = new ArrayList<>();
        for (JsonNode canvas : root.with("canvases")) {
            JsonNode document = canvas.path("project");
            if (userId.equals(ReferenceService.text(canvas, "ownerId")) && !ReferenceService.bool(canvas, "deleted") && projectId.equals(ReferenceService.text(document, "projectId"))) {
                result.add(new ProjectCanvasPageSummaryDto(
                        ReferenceService.text(document, "id"), ReferenceService.text(document, "title"), 0,
                        ReferenceService.text(document, "createdAt"), ReferenceService.text(document, "updatedAt")));
            }
        }
        result.sort(Comparator.comparing(ProjectCanvasPageSummaryDto::id));
        return result;
    }

    private static List<CanvasUnitLinkDto> currentLinks(ObjectNode project, ObjectNode root, String projectId) {
        List<CanvasUnitLinkDto> result = new ArrayList<>();
        for (JsonNode link : project.withArray("canvasUnitLinks")) {
            JsonNode canvas = root.with("canvases").path(ReferenceService.text(link, "canvasId"));
            if (projectId.equals(ReferenceService.text(canvas.path("project"), "projectId"))) {
                result.add(linkDto(link));
            }
        }
        return result;
    }

    private static List<CanvasUnitLinkDto> links(ObjectNode project) {
        List<CanvasUnitLinkDto> result = new ArrayList<>();
        for (JsonNode link : project.withArray("canvasUnitLinks")) {
            result.add(linkDto(link));
        }
        return result;
    }

    private static List<ProjectUnitDto> units(ObjectNode project) {
        List<ProjectUnitDto> result = new ArrayList<>();
        for (JsonNode unit : project.withArray("units")) {
            result.add(projectUnitDto(unit));
        }
        return result;
    }

    private static List<ProjectUnitDto> unitSummaries(ObjectNode project) {
        List<ProjectUnitDto> result = new ArrayList<>();
        for (JsonNode unit : project.withArray("units")) {
            ProjectUnitDto full = projectUnitDto(unit);
            result.add(new ProjectUnitDto(
                    full.id(), full.projectId(), full.parentId(), full.kind(), full.title(), "", full.wordCount(),
                    full.status(), full.position(), full.createdAt(), full.updatedAt()));
        }
        return result;
    }

    private static JsonNode findUnit(ObjectNode project, String unitId) {
        for (JsonNode unit : project.withArray("units")) {
            if (unitId.equals(ReferenceService.text(unit, "id"))) {
                return unit;
            }
        }
        return null;
    }

    private static ObjectNode findLink(ObjectNode project, String canvasId, String unitId) {
        for (JsonNode link : project.withArray("canvasUnitLinks")) {
            if (canvasId.equals(ReferenceService.text(link, "canvasId")) && unitId.equals(ReferenceService.text(link, "unitId"))) {
                return (ObjectNode) link;
            }
        }
        return null;
    }

    private static int linkIndex(ObjectNode project, ObjectNode target) {
        for (int index = 0; index < project.withArray("canvasUnitLinks").size(); index++) {
            if (project.withArray("canvasUnitLinks").get(index) == target) {
                return index;
            }
        }
        return -1;
    }

    private static int unitIndex(ObjectNode project, String unitId) {
        for (int index = 0; index < project.withArray("units").size(); index++) {
            if (unitId.equals(ReferenceService.text(project.withArray("units").get(index), "id"))) {
                return index;
            }
        }
        return -1;
    }

    private static void bumpProject(ObjectNode project) {
        project.put("revision", project.path("revision").asLong(1) + 1).put("updatedAt", Instant.now().toString());
    }

    private static void bumpCanvas(ObjectNode canvas) {
        int nextRevision = canvas.path("revision").asInt(1) + 1;
        ((ObjectNode) canvas.path("project")).put("revision", nextRevision).put("updatedAt", Instant.now().toString());
        canvas.put("revision", nextRevision);
    }

    private ProjectDto projectDto(JsonNode node) {
        try {
            return service.mapper().treeToValue(node, ProjectDto.class);
        } catch (Exception exception) {
            throw new IllegalStateException("project DTO conversion failed", exception);
        }
    }

    private static ProjectUnitDto projectUnitDto(JsonNode node) {
        return new ProjectUnitDto(
                ReferenceService.text(node, "id"), ReferenceService.text(node, "projectId"), ReferenceService.text(node, "parentId"),
                ReferenceService.text(node, "kind"), ReferenceService.text(node, "title"), ReferenceService.text(node, "sourceText"),
                node.path("wordCount").asInt(), ReferenceService.text(node, "status"), node.path("position").asInt(),
                ReferenceService.text(node, "createdAt"), ReferenceService.text(node, "updatedAt"));
    }

    private static CanvasUnitLinkDto linkDto(JsonNode node) {
        return new CanvasUnitLinkDto(
                ReferenceService.text(node, "id"), ReferenceService.text(node, "projectId"), ReferenceService.text(node, "canvasId"),
                ReferenceService.text(node, "unitId"), ReferenceService.text(node, "role"), ReferenceService.text(node, "createdAt"));
    }

    private static String newId(String prefix) {
        return prefix + "-" + UUID.randomUUID();
    }

    private static String nonBlank(String value, String fallback) {
        return value == null || value.isBlank() ? fallback : value.trim();
    }

    private static int wordCount(String value) {
        return value == null || value.trim().isEmpty() ? 0 : value.length();
    }

    private static void validateUnitTitle(String title) {
        if (title.length() > 240) {
            throw new ApiException(400, 400, "bad_request", "unit title is invalid");
        }
    }

    private static void validateUnitKind(String kind) {
        if (kind == null || kind.isBlank()) {
            return;
        }
        if (!Set.of("chapter", "episode", "scene", "sequence", "act", "section").contains(kind.trim())) {
            throw new ApiException(400, 400, "bad_request", "invalid unit kind");
        }
    }

    private static boolean validStatus(String value) {
        return Set.of("draft", "planning", "in-progress", "completed", "archived", "active", "pending").contains(value);
    }

    private record NormalizedUnitInput(String kind, String title, String sourceText) {
    }

    private static int positive(String raw, int fallback) {
        try {
            int value = Integer.parseInt(raw);
            if (value <= 0) {
                throw new ApiException(400, 400, "bad_request", "pagination values must be positive");
            }
            return value;
        } catch (NumberFormatException exception) {
            throw new ApiException(400, 400, "bad_request", "pagination values must be integers");
        }
    }

    private static int positive(Integer raw, int fallback) {
        if (raw == null) {
            return fallback;
        }
        if (raw <= 0) {
            throw new ApiException(400, 400, "bad_request", "pagination values must be positive");
        }
        return raw;
    }

    private record ProjectSummary(ProjectDtos.ProjectSummaryDto dto) {
        private static ProjectSummary from(ObjectNode project) {
            return new ProjectSummary(new ProjectDtos.ProjectSummaryDto(null, 0, 0, project.withArray("units").size(), 0));
        }
    }

    private List<ProjectSummary> projectSummaries(ObjectNode root, String userId) {
        List<ProjectSummary> result = new ArrayList<>();
        for (JsonNode project : root.with("projects")) {
            if (!userId.equals(ReferenceService.text(project, "ownerId")) || ReferenceService.bool(project, "deleted")) {
                continue;
            }
            int canvasCount = currentCanvases(root, ReferenceService.text(project, "id"), userId).size();
            int completed = 0;
            for (JsonNode unit : project.withArray("units")) {
                if ("completed".equals(ReferenceService.text(unit, "status"))) {
                    completed++;
                }
            }
            result.add(new ProjectSummary(new ProjectDtos.ProjectSummaryDto(
                    projectDto(project), canvasCount, 0, project.withArray("units").size(), completed)));
        }
        result.sort(Comparator.comparing(summary -> summary.dto().project().updatedAt()));
        return result;
    }
}
