package harness

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
)

func ControlHandler(ctx context.Context, target *Target, token string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		if len(token) < 24 || subtle.ConstantTimeCompare([]byte(request.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		if request.URL.Path != "/restart" {
			http.NotFound(writer, request)
			return
		}
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", http.MethodPost)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Mode string `json:"mode"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 256))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(writer, "invalid restart body", http.StatusBadRequest)
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF || payload.Mode != "crash" && payload.Mode != "graceful" {
			http.Error(writer, "invalid restart body", http.StatusBadRequest)
			return
		}
		generation, err := target.Restart(ctx, payload.Mode)
		if err != nil {
			http.Error(writer, "owned target restart failed", http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"restarted": true, "generation": generation, "mode": payload.Mode})
	})
}
