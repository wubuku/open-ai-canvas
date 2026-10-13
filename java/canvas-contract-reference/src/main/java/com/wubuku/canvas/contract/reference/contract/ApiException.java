package com.wubuku.canvas.contract.reference.contract;

public class ApiException extends RuntimeException {
    private final int status;
    private final int code;
    private final String reason;

    public ApiException(int status, int code, String reason, String message) {
        super(message);
        this.status = status;
        this.code = code;
        this.reason = reason;
    }

    public int status() {
        return status;
    }

    public int code() {
        return code;
    }

    public String reason() {
        return reason;
    }
}
