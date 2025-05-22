package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32 // safe across goroutines
}

func (cfg *apiConfig) middlewareMetricsIncrement(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Add("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write(fmt.Appendf(nil, `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileserverHits.Load()))
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, _ *http.Request) {
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
}

func handlerHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func respondWithError(w http.ResponseWriter, statusCode int, context string, err error) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	errorMessage := fmt.Sprint(fmt.Errorf("%v: %w", context, err))
	respondWithJSON(w, statusCode, errorMessage)
}

func respondWithJSON(w http.ResponseWriter, statusCode int, payload any) {
	jsonResponse, err := json.Marshal(payload)
	if err != nil {
		log.Println(fmt.Errorf("encoding JSON: %w", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	w.Write(jsonResponse)
}

func handlerValidateChirp(w http.ResponseWriter, r *http.Request) {
	type jsonRequest struct {
		Body string `json:"body"`
	}
	type validResponse struct {
		CleanedBody string `json:"cleaned_body"`
	}

	// we use JSON Decode instead of Unmarshal because we're dealing with a stream
	// of data instead of a []byte in memory
	decoder := json.NewDecoder(r.Body)
	var data jsonRequest
	err := decoder.Decode(&data)
	if err != nil {
		log.Print(fmt.Errorf("[ error ] decoding JSON stream: %w", err))
		respondWithError(w, http.StatusBadRequest, "non-conforming JSON received", err)
		return
	}

	const maxChirpLength = 140
	if chirpLength := len([]rune(data.Body)); chirpLength > maxChirpLength {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Chirp is %d characters long", chirpLength), fmt.Errorf("remove, at least, %d characters to make it %d", chirpLength-maxChirpLength, maxChirpLength))
		return
	}

	badWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}
	cleanedChirp := censorChirp(data.Body, badWords)

	respondWithJSON(w, http.StatusOK, validResponse{CleanedBody: cleanedChirp})
}

func censorChirp(message string, badWords map[string]struct{}) string {
	// With strings.Fields we're splitting the message at every instance of 1+
	// consecutive whitespace characters according to what's defined in
	// unicode.IsSpace (where,  for example, \n is considered whitespace).
	// The alternative: strings.Split(message, " "), where we'd split the message at
	// every instance of " ".
	// What to actually use in the end will depend on the app requirements.
	words := strings.Fields(message)
	for i, word := range words {
		_, ok := badWords[strings.ToLower(word)]
		if ok {
			words[i] = "****"
		}
	}
	return strings.Join(words, " ")
}

func main() {
	// create an HTTP request multiplexer
	mux := http.NewServeMux()

	// set server parameters
	const port = 8080
	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: mux,
	}

	// declare and initialize server configuration
	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
	}

	// map server folders and routes for network access
	app := http.FileServer(http.Dir("./app"))
	assets := http.FileServer(http.Dir("./assets"))

	mux.Handle("/app/", apiCfg.middlewareMetricsIncrement(http.StripPrefix("/app/", app)))
	mux.Handle("/app/assets/", apiCfg.middlewareMetricsIncrement(http.StripPrefix("/app/assets/", assets)))
	mux.HandleFunc("GET /api/healthz", handlerHealth)
	mux.HandleFunc("POST /api/validate_chirp", handlerValidateChirp)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)

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
