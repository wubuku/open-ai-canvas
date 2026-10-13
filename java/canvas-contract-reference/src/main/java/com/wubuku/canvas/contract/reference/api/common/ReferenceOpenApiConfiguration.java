package com.wubuku.canvas.contract.reference.api.common;

import io.swagger.v3.oas.models.Components;
import io.swagger.v3.oas.models.OpenAPI;
import io.swagger.v3.oas.models.media.Schema;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class ReferenceOpenApiConfiguration {
    @Bean
    public OpenAPI referenceOpenAPI() {
        Schema<Object> opaqueJsonValue = new Schema<>();
        opaqueJsonValue.setType("object");
        opaqueJsonValue.setDescription("Named client-owned extension JSON; never a complete canvas document.");
        opaqueJsonValue.setAdditionalProperties(true);
        return new OpenAPI()
                .components(new Components().addSchemas("OpaqueJsonValue", opaqueJsonValue));
    }
}
