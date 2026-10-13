package com.wubuku.canvas.contract.reference.api.fixture;

import com.fasterxml.jackson.databind.JsonNode;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.CanvasTimestampFixtureRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.CanvasTimestampFixtureResponse;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ShareExpiryFixtureRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ShareExpiryFixtureResponse;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.UploadExpiryFixtureRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.UploadExpiryFixtureResponse;
import com.wubuku.canvas.contract.reference.contract.ApiException;
import com.wubuku.canvas.contract.reference.contract.ReferenceService;
import com.wubuku.canvas.contract.reference.contract.ReferenceStore;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.Hidden;
import io.swagger.v3.oas.annotations.tags.Tag;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.Valid;
import java.util.HashSet;
import java.util.Set;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@Hidden
@RestController
@RequestMapping("/")
@Tag(name = "Private contract fixtures", description = "Non-product endpoints used only to reset deterministic reference state.")
public class ReferenceFixtureController {
    private final ReferenceStore store;
    private final String fixtureToken;

    public ReferenceFixtureController(ReferenceStore store, @Value("${reference.contract.fixture-token:}") String fixtureToken) {
        this.store = store;
        this.fixtureToken = fixtureToken;
    }

    @PostMapping("set-canvas-updated-at")
    @Operation(operationId = "reference.fixture.canvas-updated-at", summary = "Set deterministic canvas timestamps")
    public CanvasTimestampFixtureResponse setCanvasUpdatedAt(HttpServletRequest request, @Valid @RequestBody CanvasTimestampFixtureRequest body) {
        authorize(request);
        Set<String> ids = new HashSet<>(body.projectIds());
        if (ids.size() != body.projectIds().size() || body.projectIds().size() > 32) {
            throw new ApiException(400, 400, "bad_request", "projectIds must be distinct and bounded");
        }
        int updated = store.update(root -> {
            int count = 0;
            for (String id : body.projectIds()) {
                JsonNode canvas = root.with("canvases").path(id);
                if (canvas.isObject()) {
                    ((com.fasterxml.jackson.databind.node.ObjectNode) canvas).path("project");
                    ((com.fasterxml.jackson.databind.node.ObjectNode) canvas).with("project").put("updatedAt", body.updatedAt());
                    count++;
                }
            }
            return count;
        });
        return new CanvasTimestampFixtureResponse(updated, body.updatedAt());
    }

    @PostMapping("reset")
    @Operation(operationId = "reference.fixture.reset", summary = "Reset the private reference state")
    public java.util.Map<String, Boolean> reset(HttpServletRequest request) {
        authorize(request);
        store.reset();
        return java.util.Map.of("reset", true);
    }

    @PostMapping("expire-upload-reservation")
    @Operation(operationId = "reference.fixture.expire-upload", summary = "Expire a mock upload reservation")
    public UploadExpiryFixtureResponse expireUpload(HttpServletRequest request, @Valid @RequestBody UploadExpiryFixtureRequest body) {
        authorize(request);
        boolean expired = store.update(root -> {
            JsonNode upload = root.with("uploads").path(body.uploadId());
            if (!upload.isObject()) {
                return false;
            }
            ((com.fasterxml.jackson.databind.node.ObjectNode) upload).put("status", "expired");
            return true;
        });
        return new UploadExpiryFixtureResponse(expired, body.uploadId());
    }

    @PostMapping("expire-share")
    @Operation(operationId = "reference.fixture.expire-share", summary = "Expire a mock public share")
    public ShareExpiryFixtureResponse expireShare(HttpServletRequest request, @Valid @RequestBody ShareExpiryFixtureRequest body) {
        authorize(request);
        boolean expired = store.update(root -> {
            JsonNode canvas = root.with("canvases").path(body.projectId());
            if (!canvas.isObject()) {
                return false;
            }
            JsonNode share = canvas.path("share");
            if (!share.isObject()) {
                return false;
            }
            ((com.fasterxml.jackson.databind.node.ObjectNode) share).put("expiresAt", "2000-01-01T00:00:00Z");
            return true;
        });
        return new ShareExpiryFixtureResponse(expired, body.projectId());
    }

    private void authorize(HttpServletRequest request) {
        if (fixtureToken == null || fixtureToken.isBlank() || !("Bearer " + fixtureToken).equals(request.getHeader("Authorization"))) {
            throw new ApiException(401, 401, "unauthorized", "fixture authorization required");
        }
    }
}
