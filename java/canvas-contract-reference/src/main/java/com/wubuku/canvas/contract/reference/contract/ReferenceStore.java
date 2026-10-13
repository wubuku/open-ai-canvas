package com.wubuku.canvas.contract.reference.contract;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.wubuku.canvas.contract.reference.persistence.ReferenceStateEntity;
import com.wubuku.canvas.contract.reference.persistence.ReferenceStateRepository;
import jakarta.annotation.PostConstruct;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.util.Base64;
import java.util.UUID;
import java.util.function.Function;
import java.util.zip.GZIPInputStream;
import java.util.zip.GZIPOutputStream;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;

@Component
public class ReferenceStore {
    private static final long STATE_ID = 1L;
    private static final String COMPRESSED_PREFIX = "gzip-base64:";

    private final ObjectMapper mapper;
    private final ReferenceStateRepository repository;
    private final boolean seedEnabled;
    private ObjectNode state;

    public ReferenceStore(
            ObjectMapper mapper,
            ReferenceStateRepository repository,
            @Value("${reference.seed.enabled:true}") boolean seedEnabled) {
        this.mapper = mapper;
        this.repository = repository;
        this.seedEnabled = seedEnabled;
    }

    @PostConstruct
    @Transactional
    public synchronized void initialize() {
        ReferenceStateEntity entity = repository.findById(STATE_ID).orElse(null);
        if (entity == null) {
            state = emptyState();
            if (seedEnabled) {
                seedDefaults(state);
            }
            persist();
            return;
        }
        try {
            state = (ObjectNode) readStoredState(entity.getStateJson());
        } catch (IOException exception) {
            throw new IllegalStateException("reference state is not valid JSON", exception);
        }
    }

    public synchronized ObjectNode snapshot() {
        return state.deepCopy();
    }

    public synchronized <T> T update(Function<ObjectNode, T> operation) {
        T result = operation.apply(state);
        persist();
        return result;
    }

    public synchronized String newId(String prefix) {
        return prefix + "-" + UUID.randomUUID();
    }

    public synchronized String encode(byte[] bytes) {
        return Base64.getEncoder().encodeToString(bytes);
    }

    public synchronized byte[] decode(String encoded) {
        return Base64.getDecoder().decode(encoded);
    }

    @Transactional
    public synchronized void reset() {
        state = emptyState();
        if (seedEnabled) {
            seedDefaults(state);
        }
        persist();
    }

    private void persist() {
        try {
            ReferenceStateEntity entity = repository.findById(STATE_ID).orElseGet(() -> new ReferenceStateEntity(STATE_ID, "{}"));
            entity.setStateJson(storeState(state));
            repository.saveAndFlush(entity);
        } catch (IOException exception) {
            throw new IllegalStateException("reference state serialization failed", exception);
        }
    }

    private JsonNode readStoredState(String stored) throws IOException {
        if (stored == null || !stored.startsWith(COMPRESSED_PREFIX)) {
            return mapper.readTree(stored);
        }
        byte[] compressed = Base64.getDecoder().decode(stored.substring(COMPRESSED_PREFIX.length()));
        try (GZIPInputStream input = new GZIPInputStream(new ByteArrayInputStream(compressed))) {
            return mapper.readTree(input);
        }
    }

    private String storeState(ObjectNode value) throws IOException {
        ByteArrayOutputStream compressed = new ByteArrayOutputStream();
        try (GZIPOutputStream output = new GZIPOutputStream(compressed)) {
            mapper.writeValue(output, value);
        }
        return COMPRESSED_PREFIX + Base64.getEncoder().encodeToString(compressed.toByteArray());
    }

    private ObjectNode emptyState() {
        ObjectNode root = JsonNodeFactory.instance.objectNode();
        root.put("seedVersion", "reference-v1");
        for (String name : new String[]{"users", "sessions", "canvases", "projects", "resources", "uploads", "assets", "folders", "tasks", "creationRuns", "channels", "models", "ledger"}) {
            root.set(name, JsonNodeFactory.instance.objectNode());
        }
        root.set("events", JsonNodeFactory.instance.arrayNode());
        root.set("features", JsonNodeFactory.instance.objectNode()
                .put("shortDramaEnabled", true)
                .put("canvasEnabled", true));
        ObjectNode resourcePolicy = JsonNodeFactory.instance.objectNode()
                .put("resourceUploadMB", 50)
                .put("generatedFileMB", 64)
                .put("dailyUploadMB", 2048)
                .put("storedFileGB", 20)
                .put("structuredDataMB", 256)
                .put("taskDataGB", 1)
                .put("assetCount", 2_000)
                .put("canvasCount", 1_000)
                .put("taskCount", 20_000)
                .put("apiCallLogCount", 100_000)
                .put("recycleBinRetentionDays", 30);
        ObjectNode storagePolicy = JsonNodeFactory.instance.objectNode()
                .put("transferTimeoutSeconds", 120)
                .put("accessURLTTLSeconds", 14_400)
                .put("providerAccessURLTTLSeconds", 14_400)
                .put("nonSeekableBufferMB", 64)
                .put("errorBodyKB", 1);
        ObjectNode taskPolicy = JsonNodeFactory.instance.objectNode()
                .put("workerConcurrency", 8)
                .put("channelConcurrency", 8)
                .put("activeTaskLimit", 5)
                .put("imageTimeoutMinutes", 8)
                .put("textTimeoutMinutes", 8)
                .put("audioTimeoutMinutes", 8)
                .put("videoTimeoutMinutes", 60)
                .put("storyboardTimeoutMinutes", 20)
                .put("defaultTimeoutMinutes", 10)
                .put("agentStepMaxOutputTokens", 16_384)
                .put("agentStepTimeoutSeconds", 0)
                .put("agentMaxSessions", 30);
        ObjectNode requestPolicy = JsonNodeFactory.instance.objectNode()
                .put("taskCreatePerMinute", 30)
                .put("resourceUploadPerMinute", 30)
                .put("resourceImportPerMinute", 30)
                .put("assetWritePerMinute", 120)
                .put("canvasWritePerMinute", 120)
                .put("registerPerHour", 30)
                .put("emailCodePerHour", 10)
                .put("loginIPPerTenMinutes", 30)
                .put("loginAccountPerTenMinutes", 10)
                .put("systemRelayPerMinute", 120)
                .put("customRelayPerMinute", 120)
                .put("customRelayConcurrency", 8)
                .put("customRelayRequestMB", 8)
                .put("customRelayResponseMB", 32)
                .put("customRelayTimeoutMinutes", 10)
                .put("systemRelayRequestMB", 8)
                .put("systemRelayResponseMB", 32)
                .put("channelCircuitFailureCount", 5)
                .put("channelCircuitOpenSeconds", 30);
        ObjectNode runtimePolicy = JsonNodeFactory.instance.objectNode();
        runtimePolicy.set("resource", resourcePolicy);
        runtimePolicy.set("storage", storagePolicy);
        runtimePolicy.set("task", taskPolicy);
        runtimePolicy.set("request", requestPolicy);
        runtimePolicy.put("configured", false);
        root.set("runtimePolicy", runtimePolicy);
        root.set("settings", JsonNodeFactory.instance.objectNode());
        return root;
    }

    private void seedDefaults(ObjectNode root) {
        ArrayNode events = root.withArray("events");
        events.addObject().put("id", 1L).put("type", "seed.ready").put("cursor", 1L);
    }
}
