package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"canvas-contract/harness"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:18082", "API bind address; non-loopback only in an isolated container")
	origin := flag.String("target-origin", "", "single private fixture target origin")
	flag.Parse()
	handler, err := harness.HTTPGateway(*origin)
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("private fixture API gateway failed")
	}
}
