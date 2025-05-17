package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	const port = 8080

	// create an HTTP request multiplexer
	reqMux := http.NewServeMux()
	// set server parameters
	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: reqMux,
	}

	log.Printf("server active on port %v\n", port)
	// start the server
	log.Fatal(server.ListenAndServe())
}
