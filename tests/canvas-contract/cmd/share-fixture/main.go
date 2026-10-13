package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	databaseURL := flag.String("database-url", "", "isolated PostgreSQL URL")
	listenAddress := flag.String("listen", "127.0.0.1:0", "private loopback listen address")
	readyFile := flag.String("ready-file", "", "private readiness file")
	flag.Parse()
	token := os.Getenv("CANVAS_CONTRACT_FIXTURE_TOKEN")
	if *databaseURL == "" || *readyFile == "" || len(token) < 24 {
		return errors.New("database URL, readiness file and database fixture token (at least 24 bytes) are required")
	}
	listenHost, _, err := net.SplitHostPort(*listenAddress)
	if err != nil || net.ParseIP(listenHost) == nil || !net.ParseIP(listenHost).IsLoopback() {
		return errors.New("database fixture must listen on a literal loopback address")
	}
	contextValue, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()
	database, err := pgx.Connect(contextValue, *databaseURL)
	if err != nil {
		return fmt.Errorf("share fixture database connection failed: %w", err)
	}
	defer database.Close(contextValue)
	listener, err := net.Listen("tcp", *listenAddress)
	if err != nil {
		return fmt.Errorf("share fixture listener failed: %w", err)
	}
	defer listener.Close()
	if err := os.WriteFile(*readyFile, []byte(fmt.Sprintf("http://%s", listener.Addr().String())), 0o600); err != nil {
		return fmt.Errorf("share fixture readiness file failed: %w", err)
	}
	var databaseMutex sync.Mutex
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if subtle.ConstantTimeCompare([]byte(request.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", http.MethodPost)
			http.Error(writer, "not found", http.StatusNotFound)
			return
		}
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16<<10))
		decoder.DisallowUnknownFields()
		databaseMutex.Lock()
		defer databaseMutex.Unlock()
		switch request.URL.Path {
		case "/expire-share":
			var payload struct {
				ProjectID string `json:"projectId"`
			}
			if err := decodeFixtureRequest(decoder, &payload); err != nil || strings.TrimSpace(payload.ProjectID) == "" {
				http.Error(writer, "invalid fixture request", http.StatusBadRequest)
				return
			}
			projectID := strings.TrimSpace(payload.ProjectID)
			result, err := database.Exec(request.Context(), "UPDATE canvas_shares SET enabled = TRUE, expires_at = CURRENT_TIMESTAMP - INTERVAL '1 second', updated_at = CURRENT_TIMESTAMP WHERE project_id = $1", projectID)
			if err != nil {
				http.Error(writer, "fixture update failed", http.StatusInternalServerError)
				return
			}
			if result.RowsAffected() != 1 {
				http.Error(writer, "share not found", http.StatusNotFound)
				return
			}
			writeFixtureJSON(writer, map[string]any{"expired": true, "projectId": projectID})
		case "/set-canvas-updated-at":
			var payload struct {
				ProjectIDs []string `json:"projectIds"`
				UpdatedAt  string   `json:"updatedAt"`
			}
			if err := decodeFixtureRequest(decoder, &payload); err != nil || len(payload.ProjectIDs) == 0 || len(payload.ProjectIDs) > 32 {
				http.Error(writer, "invalid fixture request", http.StatusBadRequest)
				return
			}
			updatedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.UpdatedAt))
			if err != nil {
				http.Error(writer, "invalid fixture timestamp", http.StatusBadRequest)
				return
			}
			seen := make(map[string]struct{}, len(payload.ProjectIDs))
			for _, value := range payload.ProjectIDs {
				id := strings.TrimSpace(value)
				if id == "" || len(id) > 80 {
					http.Error(writer, "invalid fixture project id", http.StatusBadRequest)
					return
				}
				if _, exists := seen[id]; exists {
					http.Error(writer, "duplicate fixture project id", http.StatusBadRequest)
					return
				}
				seen[id] = struct{}{}
			}
			tx, err := database.Begin(request.Context())
			if err != nil {
				http.Error(writer, "fixture transaction failed", http.StatusInternalServerError)
				return
			}
			defer tx.Rollback(context.Background())
			for id := range seen {
				result, updateErr := tx.Exec(request.Context(), "UPDATE canvas_projects SET updated_at = $1 WHERE id = $2", updatedAt, id)
				if updateErr != nil || result.RowsAffected() != 1 {
					_ = tx.Rollback(request.Context())
					if updateErr != nil {
						http.Error(writer, "fixture update failed", http.StatusInternalServerError)
					} else {
						http.Error(writer, "canvas project not found", http.StatusNotFound)
					}
					return
				}
			}
			if err := tx.Commit(request.Context()); err != nil {
				http.Error(writer, "fixture commit failed", http.StatusInternalServerError)
				return
			}
			writeFixtureJSON(writer, map[string]any{"updated": len(seen), "updatedAt": updatedAt.UTC().Format(time.RFC3339Nano)})
		case "/expire-upload-reservation":
			var payload struct {
				UploadID string `json:"uploadId"`
			}
			if err := decodeFixtureRequest(decoder, &payload); err != nil || strings.TrimSpace(payload.UploadID) == "" {
				http.Error(writer, "invalid fixture request", http.StatusBadRequest)
				return
			}
			uploadID := strings.TrimSpace(payload.UploadID)
			result, err := database.Exec(request.Context(), "UPDATE upload_reservations SET expires_at = CURRENT_TIMESTAMP - INTERVAL '1 second' WHERE id = $1", uploadID)
			if err != nil {
				http.Error(writer, "fixture update failed", http.StatusInternalServerError)
				return
			}
			if result.RowsAffected() != 1 {
				http.Error(writer, "upload reservation not found", http.StatusNotFound)
				return
			}
			writeFixtureJSON(writer, map[string]any{"expired": true, "uploadId": uploadID})
		default:
			http.NotFound(writer, request)
		}
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	finished := make(chan struct{})
	go func() {
		<-contextValue.Done()
		shutdownContext, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = server.Shutdown(shutdownContext)
		close(finished)
	}()
	serveErr := server.Serve(listener)
	cancel()
	<-finished
	if serveErr != nil && serveErr != http.ErrServerClosed {
		return serveErr
	}
	return nil
}

func decodeFixtureRequest(decoder *json.Decoder, destination any) error {
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("fixture request contains trailing JSON")
		}
		return err
	}
	return nil
}

func writeFixtureJSON(writer http.ResponseWriter, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(payload)
}
