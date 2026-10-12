package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"canvas-contract/harness"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:18081", "fixture bind address; non-loopback only in an isolated container network")
	certificateFile := flag.String("ca-file", "", "private temporary path for the public fixture CA certificate")
	flag.Parse()
	if *certificateFile == "" {
		log.Fatal("ca-file is required")
	}
	proxy, err := harness.NewImportProxy()
	if err != nil {
		log.Fatal("cannot initialize ephemeral TLS fixture")
	}
	defer proxy.Close()
	if err := os.WriteFile(*certificateFile, proxy.CA, 0600); err != nil {
		log.Fatal("cannot write ephemeral fixture CA certificate")
	}
	defer os.Remove(*certificateFile)
	server := &http.Server{Addr: *listen, Handler: proxy, ReadHeaderTimeout: 5 * time.Second}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	go func() {
		<-ctx.Done()
		proxy.Close()
		_ = server.Close()
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("cannot serve controlled import upstream")
	}
}
