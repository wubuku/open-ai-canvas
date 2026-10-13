package com.wubuku.canvas.contract.reference.api.common;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ReadListener;
import jakarta.servlet.ServletInputStream;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletRequestWrapper;
import jakarta.servlet.http.HttpServletResponse;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.BufferedReader;
import java.nio.charset.StandardCharsets;
import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.stereotype.Component;
import org.springframework.util.StreamUtils;
import org.springframework.web.filter.OncePerRequestFilter;

@Component
@Order(Ordered.HIGHEST_PRECEDENCE)
public class ImportBodyBudgetFilter extends OncePerRequestFilter {
    public static final String OVERSIZE_ATTRIBUTE = ImportBodyBudgetFilter.class.getName() + ".oversize";
    public static final int MAX_JSON_VALUE_BYTES = 32 * 1024 * 1024;
    private static final int MAX_CAPTURE_BYTES = 64 * 1024 * 1024;

    @Override
    protected void doFilterInternal(
            HttpServletRequest request,
            HttpServletResponse response,
            FilterChain filterChain) throws IOException, jakarta.servlet.ServletException {
        if (!isProjectUnitImport(request)) {
            filterChain.doFilter(request, response);
            return;
        }

        byte[] body = readAtMost(request.getInputStream(), MAX_CAPTURE_BYTES);
        boolean truncated = body.length > MAX_CAPTURE_BYTES;
        int jsonValueEnd = firstJsonValueEnd(body);
        request.setAttribute(OVERSIZE_ATTRIBUTE, truncated || jsonValueEnd > MAX_JSON_VALUE_BYTES);
        filterChain.doFilter(new CachedBodyRequest(request, body), response);
    }

    private static boolean isProjectUnitImport(HttpServletRequest request) {
        String path = request.getRequestURI();
        return "POST".equalsIgnoreCase(request.getMethod())
                && path != null
                && path.matches(".*/api/projects/[^/]+/units/import");
    }

    private static byte[] readAtMost(InputStream input, int maximumBytes) throws IOException {
        ByteArrayOutputStream output = new ByteArrayOutputStream(Math.min(maximumBytes, 8192));
        byte[] buffer = new byte[8192];
        int total = 0;
        int read;
        while ((read = input.read(buffer)) != -1) {
            int remaining = maximumBytes + 1 - total;
            if (remaining <= 0) {
                break;
            }
            int accepted = Math.min(read, remaining);
            output.write(buffer, 0, accepted);
            total += accepted;
            if (accepted < read) {
                break;
            }
        }
        return output.toByteArray();
    }

    private static int firstJsonValueEnd(byte[] body) {
        int start = 0;
        while (start < body.length && isJsonWhitespace(body[start])) {
            start++;
        }
        if (start >= body.length) {
            return body.length;
        }
        byte first = body[start];
        if (first != '{' && first != '[') {
            int cursor = start;
            while (cursor < body.length && !isJsonWhitespace(body[cursor])) {
                cursor++;
            }
            return cursor;
        }

        int depth = 0;
        boolean inString = false;
        boolean escaped = false;
        for (int index = start; index < body.length; index++) {
            byte current = body[index];
            if (inString) {
                if (escaped) {
                    escaped = false;
                } else if (current == '\\') {
                    escaped = true;
                } else if (current == '"') {
                    inString = false;
                }
                continue;
            }
            if (current == '"') {
                inString = true;
            } else if (current == '{' || current == '[') {
                depth++;
            } else if (current == '}' || current == ']') {
                depth--;
                if (depth == 0) {
                    return index + 1;
                }
            }
        }
        return body.length;
    }

    private static boolean isJsonWhitespace(byte value) {
        return value == ' ' || value == '\t' || value == '\r' || value == '\n';
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
                public int read() {
                    return input.read();
                }

                @Override
                public int read(byte[] bytes, int offset, int length) {
                    return input.read(bytes, offset, length);
                }

                @Override
                public boolean isFinished() {
                    return input.available() == 0;
                }

                @Override
                public boolean isReady() {
                    return true;
                }

                @Override
                public void setReadListener(ReadListener listener) {
                }
            };
        }

        @Override
        public BufferedReader getReader() {
            return new BufferedReader(new java.io.InputStreamReader(getInputStream(), StandardCharsets.UTF_8));
        }

        @Override
        public int getContentLength() {
            return body.length;
        }

        @Override
        public long getContentLengthLong() {
            return body.length;
        }
    }
}
