package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// create an HTTP request multiplexer
	mux := http.NewServeMux()

	// set server parameters
	const port = 8080
	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: mux,
	}

	// map server folders for network access
	app := http.FileServer(http.Dir("./app"))
	assets := http.FileServer(http.Dir("./assets"))
	mux.Handle("/app/", http.StripPrefix("/app/", app))
	mux.Handle("/app/assets/", http.StripPrefix("/app/assets/", assets))

	// register custom handler
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	// start the server
	log.Printf("server is listening for requests on port %v\n", port)
	if err := server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			log.Fatal(fmt.Errorf("server failed: %w", err))
		} else {
			log.Println("server exited gracefully")
		}
	}
}
