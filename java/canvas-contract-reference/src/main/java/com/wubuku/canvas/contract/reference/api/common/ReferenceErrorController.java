package com.wubuku.canvas.contract.reference.api.common;

import static com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ApiEnvelope;
import static com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.WireData;

import io.swagger.v3.oas.annotations.Hidden;
import jakarta.servlet.RequestDispatcher;
import jakarta.servlet.http.HttpServletRequest;
import java.util.regex.Pattern;
import org.springframework.boot.web.servlet.error.ErrorController;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@Hidden
@RestController
public class ReferenceErrorController implements ErrorController {
    private static final Pattern RESOURCE_CHUNK_PATH = Pattern.compile("/api/resources/uploads/[^/]+/chunks/[0-9]+");

    @RequestMapping(value = "/error", produces = MediaType.APPLICATION_JSON_VALUE)
    public ResponseEntity<ApiEnvelope<WireData>> error(HttpServletRequest request) {
        int status = status(request);
        String path = (String) request.getAttribute(RequestDispatcher.ERROR_REQUEST_URI);
        String reason = status == 401 ? "unauthorized" : "bad_request";
        String message = status == 401 ? "authentication required" : "invalid request";
        if (status >= 500) {
            reason = "internal";
            message = "internal error";
        }
        if (path != null && RESOURCE_CHUNK_PATH.matcher(path).matches() && status == 400) {
            reason = "bad_request";
            message = "invalid request";
        }
        return ResponseEntity.status(status)
                .contentType(MediaType.APPLICATION_JSON)
                .body(ApiEnvelope.failure(status, message, reason));
    }

    private static int status(HttpServletRequest request) {
        Object value = request.getAttribute(RequestDispatcher.ERROR_STATUS_CODE);
        return value instanceof Number number && number.intValue() >= 400 ? number.intValue() : 500;
    }
}
