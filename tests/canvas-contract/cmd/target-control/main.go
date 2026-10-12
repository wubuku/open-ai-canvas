package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"canvas-contract/harness"
)

type arguments []string

func (values *arguments) String() string { return "configured target arguments" }
func (values *arguments) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	binary := flag.String("target-bin", "", "owned target executable")
	directory := flag.String("target-dir", "", "owned target working directory")
	origin := flag.String("target-origin", "", "owned target loopback HTTP origin")
	healthPath := flag.String("health-path", "/api/health", "target readiness path")
	readyFile := flag.String("ready-file", "", "private output file for control origin")
	var targetArguments arguments
	flag.Var(&targetArguments, "target-arg", "repeat for each target argument")
	flag.Parse()
	token := os.Getenv("CANVAS_CONTRACT_CONTROL_TOKEN")
	if len(token) < 24 || *readyFile == "" {
		return errors.New("private readiness file and control token (at least 24 bytes) are required")
	}
	environment := make([]string, 0)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "CANVAS_CONTRACT_CONTROL_TOKEN=") {
			environment = append(environment, value)
		}
	}
	target, err := harness.NewTarget(harness.Config{
		Binary: *binary, Arguments: targetArguments, Directory: *directory, Environment: environment,
		Origin: *origin, HealthPath: *healthPath, Output: os.Stdout, ReadyTimeout: 45 * time.Second,
	})
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()
	defer target.Stop()
	if err := target.Start(ctx); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return errors.New("control loopback listener failed")
	}
	defer listener.Close()
	encoded, err := json.Marshal(map[string]string{"origin": "http://" + listener.Addr().String()})
	if err != nil || os.WriteFile(*readyFile, encoded, 0o600) != nil {
		return errors.New("private control readiness file failed")
	}
	server := &http.Server{
		Handler: harness.ControlHandler(ctx, target, token), ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout: 5 * time.Second, WriteTimeout: 55 * time.Second, IdleTimeout: 5 * time.Second,
	}
	finished := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutdownContext, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(shutdownContext)
		close(finished)
	}()
	serveErr := server.Serve(listener)
	cancel()
	<-finished
	if serveErr != nil && serveErr != http.ErrServerClosed {
		return errors.New("control server failed")
	}
	return nil
}
