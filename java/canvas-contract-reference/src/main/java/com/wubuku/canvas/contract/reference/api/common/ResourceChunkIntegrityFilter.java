package com.wubuku.canvas.contract.reference.api.common;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.wubuku.canvas.contract.reference.contract.ReferenceService;
import com.wubuku.canvas.contract.reference.contract.ReferenceStore;
import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.ServletInputStream;
import jakarta.servlet.ReadListener;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletRequestWrapper;
import jakarta.servlet.http.HttpServletResponse;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

@Component
@Order(Ordered.HIGHEST_PRECEDENCE + 1)
public class ResourceChunkIntegrityFilter extends OncePerRequestFilter {
    private static final Pattern CHUNK_PATH = Pattern.compile("/api/resources/uploads/([^/]+)/chunks/(\\d+)");
    private static final int MAX_CAPTURE_BYTES = 64 * 1024 * 1024;

    private final ObjectMapper mapper;
    private final ReferenceService service;
    private final ReferenceStore store;

    public ResourceChunkIntegrityFilter(ObjectMapper mapper, ReferenceService service, ReferenceStore store) {
        this.mapper = mapper;
        this.service = service;
        this.store = store;
    }

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
            throws IOException, ServletException {
        if (!"PUT".equalsIgnoreCase(request.getMethod()) || !isChunkPath(request.getRequestURI())) {
            chain.doFilter(request, response);
            return;
        }

        byte[] body;
        try {
            body = readAtMost(request.getInputStream(), MAX_CAPTURE_BYTES);
        } catch (IOException exception) {
            removeStagedChunk(request);
            writeBadRequest(response);
            return;
        }
        long declared = request.getContentLengthLong();
        if (declared >= 0 && (body.length != declared || body.length > MAX_CAPTURE_BYTES)) {
            removeStagedChunk(request);
            writeBadRequest(response);
            return;
        }
        chain.doFilter(new CachedBodyRequest(request, body), response);
    }

    private static boolean isChunkPath(String path) {
        return path != null && CHUNK_PATH.matcher(path).matches();
    }

    private static byte[] readAtMost(ServletInputStream input, int maximumBytes) throws IOException {
        ByteArrayOutputStream output = new ByteArrayOutputStream(Math.min(maximumBytes, 8192));
        byte[] buffer = new byte[8192];
        int total = 0;
        int read;
        while ((read = input.read(buffer)) != -1) {
            int remaining = maximumBytes + 1 - total;
            if (remaining <= 0) break;
            int accepted = Math.min(read, remaining);
            output.write(buffer, 0, accepted);
            total += accepted;
            if (accepted < read) break;
        }
        return output.toByteArray();
    }

    private void removeStagedChunk(HttpServletRequest request) {
        Matcher matcher = CHUNK_PATH.matcher(request.getRequestURI());
        if (!matcher.matches()) return;
        String userId = service.currentUserId(request);
        if (userId == null) return;
        String uploadId = matcher.group(1);
        String index = matcher.group(2);
        store.update(root -> {
            var upload = root.with("uploads").path(uploadId);
            if (upload.isObject() && userId.equals(ReferenceService.text(upload, "ownerId"))) {
                ((com.fasterxml.jackson.databind.node.ObjectNode) upload).with("rejectedChunks").put(index, true);
            }
            return null;
        });
    }

    private void writeBadRequest(HttpServletResponse response) throws IOException {
        byte[] payload = mapper.writeValueAsBytes(
                ReferenceDtos.ApiEnvelope.failure(400, "invalid request", "bad_request"));
        response.setStatus(HttpServletResponse.SC_BAD_REQUEST);
        response.setContentType(MediaType.APPLICATION_JSON_VALUE);
        response.setContentLength(payload.length);
        response.getOutputStream().write(payload);
        response.flushBuffer();
    }

    private static final class CachedBodyRequest extends HttpServletRequestWrapper {
        private final byte[] body;

        private CachedBodyRequest(HttpServletRequest request, byte[] body) {
            super(request);
            this.body = body;
        }

        @Override
        public ServletInputStream getInputStream() {
            ByteArrayInputStream input = new ByteArrayInputStream(body);
            return new ServletInputStream() {
                @Override
                public int read() { return input.read(); }

                @Override
                public boolean isFinished() { return input.available() == 0; }

                @Override
                public boolean isReady() { return true; }

                @Override
                public void setReadListener(ReadListener listener) { }
            };
        }
    }
}
