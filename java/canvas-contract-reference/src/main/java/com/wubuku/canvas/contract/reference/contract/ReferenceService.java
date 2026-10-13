package com.wubuku.canvas.contract.reference.contract;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ReferenceObjectDto;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasDocumentDto;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasHistorySnapshotDto;
import com.wubuku.canvas.contract.reference.api.canvas.CanvasSummaryDto;
import jakarta.servlet.http.Cookie;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.function.Function;
import org.springframework.stereotype.Service;

@Service
public class ReferenceService {
    public static final String SESSION_COOKIE = "canvas_session";

    private final ObjectMapper mapper;
    private final ReferenceStore store;

    public ReferenceService(ObjectMapper mapper, ReferenceStore store) {
        this.mapper = mapper;
        this.store = store;
    }

    public ObjectMapper mapper() {
        return mapper;
    }

    public ReferenceObjectDto wire(JsonNode node) {
        return new ReferenceObjectDto(node == null ? null : node.deepCopy());
    }

    public CanvasDocumentDto canvasDocument(JsonNode node) {
        return convert(node, CanvasDocumentDto.class);
    }

    public CanvasSummaryDto canvasSummary(JsonNode node) {
        return convert(node, CanvasSummaryDto.class);
    }

    public CanvasHistorySnapshotDto canvasHistorySnapshot(JsonNode node) {
        return convert(node, CanvasHistorySnapshotDto.class);
    }

    private <T> T convert(JsonNode node, Class<T> type) {
        try {
            return mapper.treeToValue(node, type);
        } catch (Exception exception) {
            throw new IllegalStateException("reference wire conversion failed for " + type.getSimpleName(), exception);
        }
    }

    public String currentUserId(HttpServletRequest request) {
        String sessionId = cookie(request, SESSION_COOKIE);
        return userIdForSession(sessionId);
    }

    public String userIdForSession(String sessionId) {
        if (sessionId == null) {
            return null;
        }
        JsonNode session = store.snapshot().path("sessions").path(sessionId);
        return session.isObject() ? text(session, "userId") : null;
    }

    public String requireUser(HttpServletRequest request) {
        String userId = currentUserId(request);
        if (userId == null) {
            throw new ApiException(401, 401, "unauthorized", "authentication required");
        }
        return userId;
    }

    public String requireAdmin(HttpServletRequest request) {
        String userId = requireUser(request);
        JsonNode user = store.snapshot().path("users").path(userId);
        if (!"admin".equals(text(user, "role"))) {
            throw new ApiException(403, 403, "forbidden", "administrator permission required");
        }
        return userId;
    }

    public String register(String username, String password, String displayName, String role) {
        return store.update(root -> {
            ObjectNode users = root.with("users");
            for (JsonNode user : users) {
                if (username.equals(text(user, "username"))) {
                    throw new ApiException(409, 409, "username_conflict", "username already exists");
                }
            }
            String id = store.newId("user");
            ObjectNode user = users.putObject(id);
            user.put("id", id).put("username", username).put("password", password)
                    .put("displayName", displayName == null ? username : displayName)
                    .put("role", role == null ? (users.size() == 1 ? "admin" : "user") : role)
                    .put("status", "active");
            return id;
        });
    }

    public String login(String username, String password) {
        return store.update(root -> {
            String id = null;
            for (Map.Entry<String, JsonNode> entry : root.with("users").properties()) {
                JsonNode user = entry.getValue();
                if (username.equals(text(user, "username")) && password.equals(text(user, "password")) && "active".equals(text(user, "status"))) {
                    id = entry.getKey();
                    break;
                }
            }
            if (id == null) {
                throw new ApiException(401, 401, "invalid_credentials", "invalid credentials");
            }
            String sessionId = store.newId("session");
            root.with("sessions").putObject(sessionId).put("userId", id).put("createdAt", Instant.now().toString());
            return sessionId;
        });
    }

    public void logout(HttpServletRequest request) {
        String session = cookie(request, SESSION_COOKIE);
        if (session != null) {
            store.update(root -> {
                root.with("sessions").remove(session);
                return null;
            });
        }
    }

    public ObjectNode user(String userId) {
        JsonNode node = store.snapshot().path("users").path(userId);
        if (!node.isObject()) {
            throw new ApiException(404, 404, "not_found", "user not found");
        }
        return (ObjectNode) node;
    }

    public ObjectNode requireCanvas(ObjectNode root, String canvasId, String userId) {
        JsonNode canvas = root.with("canvases").path(canvasId);
        if (!canvas.isObject() || !userId.equals(text(canvas, "ownerId")) || bool(canvas, "deleted")) {
            throw new ApiException(404, 404, "not_found", "canvas not found");
        }
        return (ObjectNode) canvas;
    }

    public ObjectNode requireProject(ObjectNode root, String projectId, String userId) {
        JsonNode project = root.with("projects").path(projectId);
        if (!project.isObject() || !userId.equals(text(project, "ownerId")) || bool(project, "deleted")) {
            throw new ApiException(404, 404, "not_found", "project not found");
        }
        return (ObjectNode) project;
    }

    public static String cookie(HttpServletRequest request, String name) {
        Cookie[] cookies = request.getCookies();
        if (cookies == null) {
            return null;
        }
        for (Cookie cookie : cookies) {
            if (name.equals(cookie.getName())) {
                return cookie.getValue();
            }
        }
        return null;
    }

    public static String text(JsonNode node, String field) {
        JsonNode value = node == null ? null : node.get(field);
        return value == null || value.isNull() ? null : value.asText();
    }

    public static long number(JsonNode node, String field, long fallback) {
        JsonNode value = node == null ? null : node.get(field);
        return value == null || !value.isNumber() ? fallback : value.asLong();
    }

    public static boolean bool(JsonNode node, String field) {
        JsonNode value = node == null ? null : node.get(field);
        return value != null && value.asBoolean(false);
    }

    public static ObjectNode copyObject(JsonNode node) {
        return node != null && node.isObject() ? (ObjectNode) node.deepCopy() : JsonNodeFactory.instance.objectNode();
    }

    public static ArrayNode copyArray(JsonNode node) {
        return node != null && node.isArray() ? (ArrayNode) node.deepCopy() : JsonNodeFactory.instance.arrayNode();
    }

    public static String nonBlank(String value, String fallback) {
        return value == null || value.isBlank() ? fallback : value;
    }

    public static List<String> strings(JsonNode array) {
        List<String> values = new ArrayList<>();
        if (array != null && array.isArray()) {
            array.forEach(value -> values.add(value.asText()));
        }
        return values;
    }
}
