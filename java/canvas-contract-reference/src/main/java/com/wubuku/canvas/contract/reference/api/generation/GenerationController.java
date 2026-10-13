package com.wubuku.canvas.contract.reference.api.generation;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ApiEnvelope;
import com.wubuku.canvas.contract.reference.contract.ApiException;
import com.wubuku.canvas.contract.reference.contract.ReferenceService;
import com.wubuku.canvas.contract.reference.contract.ReferenceStore;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.media.Content;
import io.swagger.v3.oas.annotations.media.Schema;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.tags.Tag;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.nio.charset.StandardCharsets;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api")
@Tag(name = "Reference generation boundaries", description = "Provider-free mock task, creation-run, Agent and text replay boundaries.")
public class GenerationController {
    private final ReferenceService service;
    private final ReferenceStore store;

    public GenerationController(ReferenceService service, ReferenceStore store) {
        this.service = service;
        this.store = store;
    }

    @PostMapping("/tasks")
    @Operation(operationId = "reference.generation.task.create", summary = "Create a deterministic provider-free mock task")
    @ApiResponse(responseCode = "200", description = "Mock task", content = @Content(schema = @Schema(type = "object")))
    public ResponseEntity<ApiEnvelope<ObjectNode>> createTask(HttpServletRequest request, @RequestBody ObjectNode body) {
        String userId = service.requireUser(request);
        String id = store.update(root -> {
            String taskId = store.newId("task");
            ObjectNode task = root.with("tasks").putObject(taskId);
            task.put("id", taskId).put("ownerId", userId).put("type", text(body, "type", "canvas_text"))
                    .put("operation", text(body, "operation", "text_generation"))
                    .put("prompt", text(body, "prompt", "reference replay"))
                    .put("status", "text_replay").put("createdAt", Instant.now().toString());
            task.set("input", body == null ? root.objectNode() : body.path("input").deepCopy());
            task.set("deltas", root.arrayNode());
            task.set("events", root.arrayNode());
            task.set("logs", root.arrayNode());
            return taskId;
        });
        return ResponseEntity.ok(ApiEnvelope.success(store.snapshot().with("tasks").path(id).deepCopy()));
    }

    @GetMapping("/tasks")
    @Operation(operationId = "reference.generation.task.list", summary = "List mock tasks owned by the current user")
    public ResponseEntity<ApiEnvelope<ArrayNode>> listTasks(HttpServletRequest request) {
        String userId = service.requireUser(request);
        ArrayNode result = JsonNodeFactory.instance.arrayNode();
        for (JsonNode task : store.snapshot().with("tasks")) {
            if (userId.equals(ReferenceService.text(task, "ownerId"))) result.add(task);
        }
        return ResponseEntity.ok(ApiEnvelope.success(result));
    }

    @GetMapping("/tasks/{id}")
    @Operation(operationId = "reference.generation.task.read", summary = "Read one owned mock task")
    public ResponseEntity<ApiEnvelope<ObjectNode>> readTask(HttpServletRequest request, @PathVariable String id) {
        String userId = service.requireUser(request);
        ObjectNode task = ownedTask(id, userId);
        return ResponseEntity.ok(ApiEnvelope.success(task.deepCopy()));
    }

    @GetMapping("/tasks/{id}/logs")
    @Operation(operationId = "reference.generation.task.logs", summary = "Read owned mock task logs")
    public ResponseEntity<ApiEnvelope<ArrayNode>> taskLogs(HttpServletRequest request, @PathVariable String id) {
        String userId = service.requireUser(request);
        JsonNode task = store.snapshot().with("tasks").path(id);
        if (!task.isObject() || !userId.equals(ReferenceService.text(task, "ownerId"))) {
            return ResponseEntity.ok(ApiEnvelope.success(JsonNodeFactory.instance.arrayNode()));
        }
        return ResponseEntity.ok(ApiEnvelope.success(task.path("logs").deepCopy()));
    }

    @PostMapping("/tasks/{id}/text-deltas")
    @Operation(operationId = "reference.generation.text.delta.append", summary = "Append one deterministic text replay delta")
    public ResponseEntity<ApiEnvelope<ObjectNode>> appendDelta(HttpServletRequest request, @PathVariable String id, @RequestBody ObjectNode body) {
        String userId = service.requireUser(request);
        ObjectNode task = ownedTaskForReplay(id, userId);
        ObjectNode result = store.update(root -> {
            ObjectNode current = ownedTaskForReplay(root, id, userId);
            if ("succeeded".equals(ReferenceService.text(current, "status"))) {
                throw new ApiException(400, 400, "bad_request", "text replay is already complete");
            }
            String content = text(body, "content", "");
            if (content.isBlank()) throw new ApiException(400, 400, "bad_request", "text delta content is required");
            ArrayNode deltas = current.withArray("deltas");
            long sequence = deltas.size() + 1L;
            ObjectNode delta = JsonNodeFactory.instance.objectNode()
                    .put("id", store.newId("delta"))
                    .put("taskId", id)
                    .put("sequence", sequence)
                    .put("content", content)
                    .put("byteCount", content.getBytes(StandardCharsets.UTF_8).length);
            deltas.add(delta);
            current.withArray("events").addObject().put("id", sequence).put("type", "delta").set("data", delta.deepCopy());
            return delta;
        });
        return ResponseEntity.ok(ApiEnvelope.success(result));
    }

    @GetMapping("/tasks/{id}/text-deltas")
    @Operation(operationId = "reference.generation.text.delta.replay", summary = "Replay owned text deltas after a cursor")
    public ResponseEntity<ApiEnvelope<ObjectNode>> replayDeltas(HttpServletRequest request, @PathVariable String id,
            @RequestParam(defaultValue = "0") long after) {
        String userId = service.requireUser(request);
        ObjectNode task = ownedTaskForReplay(id, userId);
        return ResponseEntity.ok(ApiEnvelope.success(replay(task, after)));
    }

    @PostMapping("/tasks/{id}/text-replay-complete")
    @Operation(operationId = "reference.generation.text.complete", summary = "Complete one deterministic text replay")
    public ResponseEntity<ApiEnvelope<ObjectNode>> completeReplay(HttpServletRequest request, @PathVariable String id, @RequestBody ObjectNode body) {
        String userId = service.requireUser(request);
        ObjectNode result = store.update(root -> {
            JsonNode storedTask = root.with("tasks").path(id);
            if (!storedTask.isObject() || !userId.equals(ReferenceService.text(storedTask, "ownerId"))) {
                throw new ApiException(400, 400, "bad_request", "task replay is unavailable");
            }
            ObjectNode task = (ObjectNode) storedTask;
            if ("succeeded".equals(ReferenceService.text(task, "status"))) {
                throw new ApiException(400, 400, "bad_request", "text replay is already complete");
            }
            task.put("status", "succeeded").put("finalText", text(body, "text", ""));
            task.withArray("events").addObject().put("id", task.withArray("deltas").size() + 1L).put("type", "terminal")
                    .set("data", JsonNodeFactory.instance.objectNode().put("status", "succeeded"));
            return task.deepCopy();
        });
        return ResponseEntity.ok(ApiEnvelope.success(result));
    }

    @GetMapping("/tasks/{id}/text-events")
    @Operation(operationId = "reference.generation.text.events", summary = "Replay owned text events as deterministic SSE")
    public ResponseEntity<String> textEvents(HttpServletRequest request, @PathVariable String id,
            @RequestParam(defaultValue = "0") long after) {
        String userId = service.requireUser(request);
        ObjectNode task = ownedTaskForReplay(id, userId);
        StringBuilder body = new StringBuilder();
        if (after < 1) body.append("event: progress\ndata: {\"status\":\"text_replay\"}\n\n");
        for (JsonNode event : task.withArray("events")) {
            long eventId = event.path("id").asLong();
            if (eventId <= after) continue;
            body.append("id: ").append(eventId).append('\n');
            String type = event.path("type").asText();
            body.append("event: ").append(type).append('\n');
            body.append("data: ").append(event.path("data").toString()).append("\n\n");
        }
        return ResponseEntity.ok().contentType(MediaType.TEXT_EVENT_STREAM).header(HttpHeaders.CACHE_CONTROL, "no-cache")
                .body(body.toString());
    }

    @PostMapping("/creation-runs")
    @Operation(operationId = "reference.generation.creation-run.create", summary = "Create a mock creation run")
    public ResponseEntity<ApiEnvelope<ObjectNode>> createRun(HttpServletRequest request, @RequestBody ObjectNode body) {
        String userId = service.requireUser(request);
        String id = store.update(root -> {
            String runId = store.newId("creation-run");
            ObjectNode run = root.with("creationRuns").putObject(runId);
            run.put("id", runId).put("ownerId", userId).put("clientKey", text(body, "clientKey", runId))
                    .put("status", "created").set("state", body == null ? root.objectNode() : body.path("state").deepCopy());
            return runId;
        });
        ObjectNode data = JsonNodeFactory.instance.objectNode();
        data.set("run", store.snapshot().with("creationRuns").path(id).deepCopy());
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @GetMapping("/creation-runs")
    @Operation(operationId = "reference.generation.creation-run.list", summary = "List owned mock creation runs")
    public ResponseEntity<ApiEnvelope<ArrayNode>> listRuns(HttpServletRequest request) {
        String userId = service.requireUser(request);
        ArrayNode result = JsonNodeFactory.instance.arrayNode();
        for (JsonNode run : store.snapshot().with("creationRuns")) if (userId.equals(ReferenceService.text(run, "ownerId"))) result.add(run);
        return ResponseEntity.ok(ApiEnvelope.success(result));
    }

    @GetMapping("/creation-runs/{id}")
    @Operation(operationId = "reference.generation.creation-run.read", summary = "Read one owned mock creation run")
    public ResponseEntity<ApiEnvelope<ObjectNode>> readRun(HttpServletRequest request, @PathVariable String id) {
        String userId = service.requireUser(request);
        JsonNode run = store.snapshot().with("creationRuns").path(id);
        if (!run.isObject() || !userId.equals(ReferenceService.text(run, "ownerId"))) throw notFound("creation run not found");
        return ResponseEntity.ok(ApiEnvelope.success(run.deepCopy()));
    }

    @GetMapping("/agent/capabilities")
    @Operation(operationId = "reference.agent.capabilities", summary = "Describe deterministic Agent capability boundaries")
    public ResponseEntity<ApiEnvelope<ObjectNode>> capabilities(HttpServletRequest request) {
        service.requireUser(request);
        ObjectNode result = JsonNodeFactory.instance.objectNode();
        result.putArray("capabilities").add("canvas");
        return ResponseEntity.ok(ApiEnvelope.success(result));
    }

    @GetMapping("/agent/skills/usage")
    @Operation(operationId = "reference.agent.skills.usage", summary = "Read mock Agent skill usage")
    public ResponseEntity<ApiEnvelope<ObjectNode>> skillUsage(HttpServletRequest request) {
        service.requireUser(request);
        ObjectNode result = JsonNodeFactory.instance.objectNode();
        result.putArray("skills");
        return ResponseEntity.ok(ApiEnvelope.success(result));
    }

    @GetMapping("/agent/profile")
    @Operation(operationId = "reference.agent.profile", summary = "Read mock Agent profile")
    public ResponseEntity<ApiEnvelope<ObjectNode>> profile(HttpServletRequest request) {
        service.requireUser(request);
        return ResponseEntity.ok(ApiEnvelope.success(JsonNodeFactory.instance.objectNode().put("mode", "reference")));
    }

    @GetMapping("/agent/runs/{id}")
    @Operation(operationId = "reference.agent.run.read", summary = "Read one mock Agent run boundary")
    public ResponseEntity<ApiEnvelope<ObjectNode>> readAgentRun(HttpServletRequest request, @PathVariable String id) {
        service.requireUser(request);
        throw notFound("agent run not found");
    }

    private ObjectNode ownedTask(String id, String userId) {
        JsonNode task = store.snapshot().with("tasks").path(id);
        if (!task.isObject() || !userId.equals(ReferenceService.text(task, "ownerId"))) throw notFound("task not found");
        return task.deepCopy();
    }

    private ObjectNode ownedTaskForReplay(String id, String userId) {
        JsonNode task = store.snapshot().with("tasks").path(id);
        if (!task.isObject() || !userId.equals(ReferenceService.text(task, "ownerId"))) {
            throw new ApiException(500, 500, "internal", "task replay is unavailable");
        }
        return task.deepCopy();
    }

    private static ObjectNode ownedTaskForReplay(ObjectNode root, String id, String userId) {
        JsonNode task = root.with("tasks").path(id);
        if (!task.isObject() || !userId.equals(ReferenceService.text(task, "ownerId"))) {
            throw new ApiException(500, 500, "internal", "task replay is unavailable");
        }
        return (ObjectNode) task;
    }

    private static ObjectNode replay(ObjectNode task, long after) {
        ObjectNode result = JsonNodeFactory.instance.objectNode();
        ArrayNode deltas = result.putArray("deltas");
        for (JsonNode delta : task.withArray("deltas")) if (delta.path("sequence").asLong() > after) deltas.add(delta);
        result.put("complete", "succeeded".equals(ReferenceService.text(task, "status")))
                .put("status", ReferenceService.text(task, "status"));
        if (task.has("finalText")) result.put("finalText", task.path("finalText").asText());
        return result;
    }

    private static String text(JsonNode node, String field, String fallback) {
        String value = ReferenceService.text(node, field);
        return value == null || value.isBlank() ? fallback : value;
    }

    private static ApiException notFound(String message) {
        return new ApiException(404, 404, "not_found", message);
    }
}
