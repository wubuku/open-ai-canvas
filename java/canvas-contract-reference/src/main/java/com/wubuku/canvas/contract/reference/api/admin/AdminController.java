package com.wubuku.canvas.contract.reference.api.admin;

import com.fasterxml.jackson.databind.node.ObjectNode;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.AdminUserRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ApiEnvelope;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ChannelRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.FeatureSettingsRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ModelRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ReferenceObjectDto;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.RuntimePolicyRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.WireData;
import com.wubuku.canvas.contract.reference.contract.ReferenceService;
import com.wubuku.canvas.contract.reference.contract.ReferenceStore;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.tags.Tag;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.Valid;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PatchMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/admin")
@Tag(name = "Reference administration", description = "Small deterministic setup surface used by the black-box suite.")
public class AdminController {
    private final ReferenceService service;
    private final ReferenceStore store;

    public AdminController(ReferenceService service, ReferenceStore store) {
        this.service = service;
        this.store = store;
    }

    @PostMapping("/users")
    @Operation(operationId = "reference.admin.users.create", summary = "Create a mock user")
    public ResponseEntity<ApiEnvelope<WireData>> createUser(HttpServletRequest request, @Valid @RequestBody AdminUserRequest body) {
        service.requireAdmin(request);
        String id = service.register(body.username(), body.password(), body.displayName(), body.role() == null ? "user" : body.role());
        store.update(root -> {
            root.with("users").with(id).put("status", body.status() == null ? "active" : body.status());
            return null;
        });
        WireData data = new WireData();
        data.user = service.wire(service.user(id));
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @GetMapping("/settings/features")
    @Operation(operationId = "reference.admin.settings.features.read", summary = "Read mock feature flags")
    public ResponseEntity<ApiEnvelope<WireData>> features(HttpServletRequest request) {
        service.requireAdmin(request);
        WireData data = new WireData();
        data.features = service.wire(store.snapshot().path("features"));
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @PatchMapping("/settings/features")
    @Operation(operationId = "reference.admin.settings.features.update", summary = "Update mock feature flags")
    public ResponseEntity<ApiEnvelope<WireData>> updateFeatures(HttpServletRequest request, @RequestBody FeatureSettingsRequest body) {
        service.requireAdmin(request);
        store.update(root -> {
            ObjectNode features = root.with("features");
            if (body.shortDramaEnabled() != null) features.put("shortDramaEnabled", body.shortDramaEnabled());
            if (body.canvasEnabled() != null) features.put("canvasEnabled", body.canvasEnabled());
            return null;
        });
        return features(request);
    }

    @GetMapping("/settings/runtime-policy")
    @Operation(operationId = "reference.admin.settings.runtime-policy.read", summary = "Read mock runtime limits")
    public ResponseEntity<ApiEnvelope<WireData>> runtimePolicy(HttpServletRequest request) {
        service.requireAdmin(request);
        WireData data = new WireData();
        data.setting = service.wire(store.snapshot().path("runtimePolicy"));
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @PutMapping("/settings/runtime-policy")
    @Operation(operationId = "reference.admin.settings.runtime-policy.update", summary = "Update mock runtime limits")
    public ResponseEntity<ApiEnvelope<WireData>> updateRuntimePolicy(HttpServletRequest request, @RequestBody ObjectNode body) {
        service.requireAdmin(request);
        store.update(root -> {
            root.set("runtimePolicy", body == null ? root.objectNode() : body.deepCopy());
            return null;
        });
        return runtimePolicy(request);
    }

    @PatchMapping("/settings/libtv")
    @Operation(operationId = "reference.admin.settings.libtv.update", summary = "Configure deterministic import fixture")
    public ResponseEntity<ApiEnvelope<WireData>> libtv(HttpServletRequest request, @RequestBody ObjectNode body) {
        service.requireAdmin(request);
        store.update(root -> {
            root.with("settings").set("libtv", body.deepCopy());
            return null;
        });
        return ResponseEntity.ok(ApiEnvelope.success(new WireData()));
    }

    @PatchMapping("/settings/oss")
    @Operation(operationId = "reference.admin.settings.oss.update", summary = "Configure mock object storage")
    public ResponseEntity<ApiEnvelope<WireData>> oss(HttpServletRequest request, @RequestBody ObjectNode body) {
        service.requireAdmin(request);
        store.update(root -> {
            root.with("settings").set("oss", body.deepCopy());
            return null;
        });
        WireData data = new WireData();
        data.settings = service.wire(store.snapshot().path("settings").path("oss"));
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @PostMapping("/settings/oss/test")
    @Operation(operationId = "reference.admin.settings.oss.test", summary = "Run the mock object storage check")
    public ResponseEntity<ApiEnvelope<WireData>> ossTest(HttpServletRequest request, @RequestBody ObjectNode body) {
        service.requireAdmin(request);
        WireData data = new WireData();
        ObjectNode result = body.deepCopy();
        result.put("ok", true);
        data.settings = service.wire(result);
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @PostMapping("/channels")
    @Operation(operationId = "reference.admin.channels.create", summary = "Create a mock model channel")
    public ResponseEntity<ApiEnvelope<WireData>> channel(HttpServletRequest request, @RequestBody ChannelRequest body) {
        service.requireAdmin(request);
        String id = store.update(root -> {
            String channelId = store.newId("channel");
            ObjectNode channel = root.with("channels").putObject(channelId);
            channel.put("id", channelId).put("name", ReferenceService.nonBlank(body.name(), "reference"))
                    .put("provider", ReferenceService.nonBlank(body.provider(), "mock"))
                    .put("baseUrl", ReferenceService.nonBlank(body.baseUrl(), "http://127.0.0.1/mock"));
            return channelId;
        });
        WireData data = new WireData();
        data.channel = service.wire(store.snapshot().path("channels").path(id));
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }

    @PostMapping("/channels/{id}/models")
    @Operation(operationId = "reference.admin.channels.models.create", summary = "Create a mock channel model")
    public ResponseEntity<ApiEnvelope<WireData>> model(HttpServletRequest request, @PathVariable String id, @RequestBody ModelRequest body) {
        service.requireAdmin(request);
        if (!store.snapshot().path("channels").has(id)) {
            throw new com.wubuku.canvas.contract.reference.contract.ApiException(404, 404, "not_found", "channel not found");
        }
        String modelId = store.update(root -> {
            String value = store.newId("model");
            ObjectNode model = root.with("models").putObject(value);
            model.put("id", value).put("channelId", id).put("model", ReferenceService.nonBlank(body.model(), "mock-model"))
                    .put("type", ReferenceService.nonBlank(body.type(), "text"));
            return value;
        });
        WireData data = new WireData();
        data.model = service.wire(store.snapshot().path("models").path(modelId));
        return ResponseEntity.ok(ApiEnvelope.success(data));
    }
}
