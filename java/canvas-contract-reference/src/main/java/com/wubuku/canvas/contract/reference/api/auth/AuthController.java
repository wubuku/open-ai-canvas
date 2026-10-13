package com.wubuku.canvas.contract.reference.api.auth;

import com.fasterxml.jackson.databind.node.ObjectNode;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ApiEnvelope;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.LoginRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.RegisterRequest;
import com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.WireData;
import com.wubuku.canvas.contract.reference.contract.ReferenceService;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.tags.Tag;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import jakarta.validation.Valid;
import org.springframework.http.ResponseCookie;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/auth")
@Tag(name = "Reference authentication", description = "Deterministic cookie session endpoints for the contract reference.")
public class AuthController {
    private final ReferenceService service;

    public AuthController(ReferenceService service) {
        this.service = service;
    }

    @PostMapping("/register")
    @Operation(operationId = "reference.auth.register", summary = "Register a mock reference user")
    @ApiResponse(responseCode = "200", description = "User created")
    public ResponseEntity<ApiEnvelope<WireData>> register(
            @Valid @RequestBody RegisterRequest request,
            HttpServletResponse response) {
        String userId = service.register(request.username(), request.password(), request.displayName(), null);
        String sessionId = service.login(request.username(), request.password());
        addSessionCookie(response, sessionId);
        return ResponseEntity.ok(ApiEnvelope.success(sessionData(userId)));
    }

    @PostMapping("/login")
    @Operation(operationId = "reference.auth.login", summary = "Create a mock cookie session")
    public ResponseEntity<ApiEnvelope<WireData>> login(
            @Valid @RequestBody LoginRequest request,
            HttpServletResponse response) {
        String sessionId = service.login(request.username(), request.password());
        String userId = service.userIdForSession(sessionId);
        addSessionCookie(response, sessionId);
        return ResponseEntity.ok(ApiEnvelope.success(sessionData(userId)));
    }

    @PostMapping("/logout")
    @Operation(operationId = "reference.auth.logout", summary = "Revoke the current mock session")
    public ResponseEntity<ApiEnvelope<WireData>> logout(HttpServletRequest request, HttpServletResponse response) {
        service.logout(request);
        response.addHeader("Set-Cookie", ResponseCookie.from(ReferenceService.SESSION_COOKIE, "")
                .path("/").httpOnly(true).sameSite("Lax").maxAge(0).build().toString());
        return ResponseEntity.ok(ApiEnvelope.success(new WireData()));
    }

    @GetMapping("/session")
    @Operation(operationId = "reference.auth.session", summary = "Read the current mock session")
    public ResponseEntity<ApiEnvelope<WireData>> session(HttpServletRequest request) {
        String userId = service.requireUser(request);
        return ResponseEntity.ok(ApiEnvelope.success(sessionData(userId)));
    }

    private WireData sessionData(String userId) {
        ObjectNode user = service.user(userId);
        ObjectNode session = user.deepCopy();
        session.remove("password");
        WireData data = new WireData();
        data.session = service.wire(session);
        return data;
    }

    private void addSessionCookie(HttpServletResponse response, String sessionId) {
        response.addHeader("Set-Cookie", ResponseCookie.from(ReferenceService.SESSION_COOKIE, sessionId)
                .path("/").httpOnly(true).sameSite("Lax").build().toString());
    }

}
