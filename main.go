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

	// map the public web root of the server to `./public`
	webRoot := http.FileServer(http.Dir("./public"))
	reqMux.Handle("/", webRoot)

	log.Printf("server active on port %v\n", port)
	// start the server
	log.Fatal(server.ListenAndServe())
}
