package main

import (
	"net/http"
)

func main() {
	// create an HTTP request multiplexer
	reqMux := http.NewServeMux()
	// set server parameters
	server := &http.Server{Addr: ":8080", Handler: reqMux}
	// start the server
	server.ListenAndServe()
}
