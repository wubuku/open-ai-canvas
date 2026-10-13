package com.wubuku.canvas.contract.reference.api.common;

import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import com.fasterxml.jackson.databind.JsonNode;
import io.swagger.v3.oas.annotations.media.Schema;

/**
 * A named extension boundary for wire fields whose schema is intentionally owned by a client node type.
 * It is not used as the type of a complete canvas document or response.
 */
@Schema(name = "OpaqueJsonValue", description = "Explicitly named client-owned extension JSON. This is not the type of a complete canvas document.")
public record OpaqueJsonValue(JsonNode value) {
    @JsonCreator(mode = JsonCreator.Mode.DELEGATING)
    public OpaqueJsonValue {
    }

    @JsonValue
    public JsonNode json() {
        return value;
    }
}
