package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// create an HTTP request multiplexer
	reqMux := http.NewServeMux()

	// set server parameters
	const port = 8080
	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: reqMux,
	}

	// map server folders for network access
	webRoot := http.FileServer(http.Dir("./public"))
	assets := http.FileServer(http.Dir("./assets"))
	reqMux.Handle("/", webRoot)
	reqMux.Handle("/assets/", http.StripPrefix("/assets/", assets))

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
