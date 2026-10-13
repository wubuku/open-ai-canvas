package com.wubuku.canvas.contract.reference.api.common;

import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.media.Content;
import io.swagger.v3.oas.annotations.media.Schema;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.tags.Tag;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api")
@Tag(name = "Reference runtime", description = "Operational endpoints for the deterministic reference process.")
public class HealthController {
    @GetMapping("/health")
    @Operation(operationId = "reference.health", summary = "Report that the reference process is ready")
    @ApiResponse(responseCode = "200", description = "Reference process is ready", content = @Content(schema = @Schema(implementation = HealthResponse.class)))
    public HealthResponse health() {
        return new HealthResponse("ok", "canvas-contract-reference");
    }

    @Schema(name = "ReferenceHealthResponse", description = "Typed readiness response for the reference process.")
    public record HealthResponse(
            @Schema(description = "Readiness status", example = "ok") String status,
            @Schema(description = "Reference process mode", example = "canvas-contract-reference") String mode) {
    }
}
