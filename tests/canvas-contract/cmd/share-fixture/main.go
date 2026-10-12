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
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
)

type fixture struct {
	databaseURL string
	token       string
}

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
	token := os.Getenv("CANVAS_CONTRACT_SHARE_FIXTURE_TOKEN")
	if *databaseURL == "" || *readyFile == "" || len(token) < 24 {
		return errors.New("database URL, readiness file and share fixture token (at least 24 bytes) are required")
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
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if subtle.ConstantTimeCompare([]byte(request.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		if request.Method != http.MethodPost || request.URL.Path != "/expire-share" {
			writer.Header().Set("Allow", http.MethodPost)
			http.Error(writer, "not found", http.StatusNotFound)
			return
		}
		var payload struct {
			ProjectID string `json:"projectId"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil || strings.TrimSpace(payload.ProjectID) == "" {
			http.Error(writer, "invalid fixture request", http.StatusBadRequest)
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(writer, "invalid fixture request", http.StatusBadRequest)
			return
		}
		result, err := database.Exec(request.Context(), "UPDATE canvas_shares SET enabled = TRUE, expires_at = CURRENT_TIMESTAMP - INTERVAL '1 second', updated_at = CURRENT_TIMESTAMP WHERE project_id = $1", strings.TrimSpace(payload.ProjectID))
		if err != nil {
			http.Error(writer, "fixture update failed", http.StatusInternalServerError)
			return
		}
		if result.RowsAffected() != 1 {
			http.Error(writer, "share not found", http.StatusNotFound)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"expired": true, "projectId": strings.TrimSpace(payload.ProjectID)})
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
