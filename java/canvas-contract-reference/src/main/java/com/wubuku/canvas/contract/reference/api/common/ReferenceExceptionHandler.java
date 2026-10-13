package com.wubuku.canvas.contract.reference.api.common;

import com.wubuku.canvas.contract.reference.contract.ApiException;
import com.wubuku.canvas.contract.reference.contract.ReferenceService;
import jakarta.validation.ConstraintViolationException;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.web.HttpMediaTypeNotSupportedException;
import org.springframework.web.HttpRequestMethodNotSupportedException;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.method.annotation.MethodArgumentTypeMismatchException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;
import org.springframework.web.bind.MissingServletRequestParameterException;
import jakarta.servlet.http.HttpServletRequest;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import static com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.ApiEnvelope;
import static com.wubuku.canvas.contract.reference.api.common.ReferenceDtos.WireData;

@RestControllerAdvice
public class ReferenceExceptionHandler {
    private static final Logger LOGGER = LoggerFactory.getLogger(ReferenceExceptionHandler.class);

    private final ReferenceService service;

    public ReferenceExceptionHandler(ReferenceService service) {
        this.service = service;
    }

    @ExceptionHandler(ApiException.class)
    public ResponseEntity<ApiEnvelope<WireData>> handleApi(ApiException exception) {
        var response = ResponseEntity.status(exception.status());
        if (exception.status() == 429 && "rate_limited".equals(exception.reason())
                && "resource request rate limited".equals(exception.getMessage())) {
            response.header("Retry-After", "60");
        }
        return response.body(ApiEnvelope.failure(exception.code(), exception.getMessage(), exception.reason()));
    }

    @ExceptionHandler({
            MethodArgumentNotValidException.class,
            ConstraintViolationException.class,
            HttpMediaTypeNotSupportedException.class,
            HttpRequestMethodNotSupportedException.class,
            MethodArgumentTypeMismatchException.class,
            MissingServletRequestParameterException.class})
    public ResponseEntity<ApiEnvelope<WireData>> handleValidation(
            Exception exception,
            HttpServletRequest request) {
        if (requiresAuthentication(request) && service.currentUserId(request) == null) {
            return ResponseEntity.status(401)
                    .body(ApiEnvelope.failure(401, "authentication required", "unauthorized"));
        }
        return ResponseEntity.badRequest().body(ApiEnvelope.failure(400, "invalid request", "bad_request"));
    }

    @ExceptionHandler(HttpMessageNotReadableException.class)
    public ResponseEntity<ApiEnvelope<WireData>> handleUnreadableBody(
            HttpMessageNotReadableException exception,
            HttpServletRequest request) {
        if (requiresAuthentication(request) && service.currentUserId(request) == null) {
            return ResponseEntity.status(401)
                    .body(ApiEnvelope.failure(401, "authentication required", "unauthorized"));
        }
        return ResponseEntity.badRequest().body(ApiEnvelope.failure(400, "invalid request", "bad_request"));
    }

    private static boolean requiresAuthentication(HttpServletRequest request) {
        String path = request.getRequestURI();
        return path.startsWith("/api/canvas-projects")
                || path.startsWith("/api/projects")
                || path.startsWith("/api/resources")
                || path.startsWith("/api/admin")
                || path.startsWith("/api/tasks")
                || path.startsWith("/api/creation-runs")
                || path.startsWith("/api/agent");
    }

    @ExceptionHandler(Exception.class)
    public ResponseEntity<ApiEnvelope<WireData>> handleUnexpected(Exception exception) {
        LOGGER.error("Unhandled reference endpoint exception", exception);
        return ResponseEntity.internalServerError().body(ApiEnvelope.failure(500, "internal error", "internal"));
    }
}
