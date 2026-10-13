package com.wubuku.canvas.contract.reference.api.common;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.util.zip.GZIPOutputStream;
import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;
import org.springframework.web.util.ContentCachingResponseWrapper;

/**
 * Keeps compression and Vary behavior deterministic for the reference wire tests instead of delegating
 * header casing and threshold decisions to the embedded container.
 */
@Order(Ordered.HIGHEST_PRECEDENCE)
@Component
public class ReferenceCompressionFilter extends OncePerRequestFilter {
    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain filterChain)
            throws ServletException, IOException {
        if (isResourceChunk(request)) {
            filterChain.doFilter(request, response);
            response.flushBuffer();
            return;
        }
        ContentCachingResponseWrapper wrapper = new ContentCachingResponseWrapper(response);
        filterChain.doFilter(request, wrapper);
        byte[] body = wrapper.getContentAsByteArray();
        boolean canvasDocument = request.getRequestURI().startsWith("/api/canvas-projects/");
        boolean acceptsGzip = request.getHeader("Accept-Encoding") != null
                    && request.getHeader("Accept-Encoding").toLowerCase().contains("gzip");
        boolean json = wrapper.getContentType() != null && wrapper.getContentType().toLowerCase().contains("application/json");
        if (canvasDocument) {
            wrapper.setHeader("Vary", "Accept-Encoding");
        }
        String path = request.getRequestURI();
        if (path.startsWith("/api/public/canvas-shares/")) {
            boolean redirect = path.contains("/resources/") && wrapper.getHeader("Location") != null;
            wrapper.setHeader("Cache-Control", "1".equals(request.getParameter("access")) || redirect
                    ? "private, no-store"
                    : "no-store");
            wrapper.setHeader("Referrer-Policy", "no-referrer");
            if (!path.contains("/resources/")) {
                wrapper.setHeader("X-Robots-Tag", "noindex, nofollow");
            }
        }
        if ("1".equals(request.getParameter("access"))) {
            wrapper.setHeader("Referrer-Policy", "no-referrer");
        }
        if (acceptsGzip && json && body.length >= 1024 && wrapper.getStatus() >= 200 && wrapper.getStatus() < 300) {
            byte[] compressed = gzip(body);
            wrapper.resetBuffer();
            wrapper.setHeader("Content-Encoding", "gzip");
            wrapper.setHeader("Vary", "Accept-Encoding");
            wrapper.setContentLength(compressed.length);
            wrapper.getOutputStream().write(compressed);
        }
        wrapper.copyBodyToResponse();
    }

    private static byte[] gzip(byte[] body) throws IOException {
        ByteArrayOutputStream output = new ByteArrayOutputStream(body.length);
        try (GZIPOutputStream gzip = new GZIPOutputStream(output)) {
            gzip.write(body);
        }
        return output.toByteArray();
    }

    private static boolean isResourceChunk(HttpServletRequest request) {
        return "PUT".equalsIgnoreCase(request.getMethod())
                && request.getRequestURI() != null
                && request.getRequestURI().matches("/api/resources/uploads/[^/]+/chunks/[0-9]+");
    }
}
