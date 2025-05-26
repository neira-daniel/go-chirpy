package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/neira-daniel/go-chirpy/internal/database"
)

type User struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func addTagsToChirp(chirp database.Chirp) Chirp {
	return Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}
}

type apiConfig struct {
	db             *database.Queries
	platform       string
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

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if err := cfg.db.ResetDatabase(r.Context()); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		cfg.fileserverHits.Store(0)
		w.WriteHeader(http.StatusOK)
	}
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

func validateChirp(w http.ResponseWriter, r *http.Request) (string, uuid.UUID, bool) {
	type jsonRequest struct {
		Body   string    `json:"body"`
		UserId uuid.UUID `json:"user_id"`
	}

	// we use JSON Decode instead of Unmarshal because we're dealing with a stream
	// of data instead of a []byte in memory
	decoder := json.NewDecoder(r.Body)
	var data jsonRequest
	err := decoder.Decode(&data)
	if err != nil {
		log.Print(fmt.Errorf("[ error ] decoding JSON stream: %w", err))
		respondWithError(w, http.StatusBadRequest, "non-conforming JSON received", err)
		return "", uuid.Nil, false
	}

	const maxChirpLength = 140
	if chirpLength := len([]rune(data.Body)); chirpLength > maxChirpLength {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Chirp is %d characters long", chirpLength), fmt.Errorf("remove, at least, %d characters to make it %d", chirpLength-maxChirpLength, maxChirpLength))
		return "", uuid.Nil, false
	}

	badWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}
	cleanedChirp := censorChirp(data.Body, badWords)

	// respondWithJSON(w, http.StatusOK, validResponse{CleanedBody: cleanedChirp})
	return cleanedChirp, data.UserId, true
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

func (cfg *apiConfig) handlerUser(w http.ResponseWriter, r *http.Request) {
	type jsonRequest struct {
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	var data jsonRequest
	err := decoder.Decode(&data)
	if err != nil {
		log.Print(fmt.Errorf("[ error ] decoding JSON stream: %w", err))
		respondWithError(w, http.StatusBadRequest, "non-conforming JSON received", err)
		return
	}

	user, err := cfg.db.CreateUser(r.Context(), data.Email)
	if err != nil {
		log.Print(fmt.Errorf("[ error ] creating new DB user for %q: %w", data.Email, err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't create user", err)
		return
	}

	log.Printf("[  ok   ] user %q created", user.Email)
	respondWithJSON(w, http.StatusCreated, User{
		Id:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	})
}

func (cfg *apiConfig) handlerChirps(w http.ResponseWriter, r *http.Request) {
	body, user_id, ok := validateChirp(w, r)
	if !ok {
		return
	}
	chirp, err := cfg.db.SaveChirp(r.Context(), database.SaveChirpParams{Body: body, UserID: user_id})
	if err != nil {
		log.Print(fmt.Errorf("[ error ] storing chirp in DB: %w", err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't store chirp", err)
		return
	}

	log.Print("[  ok   ] chirp stored")
	respondWithJSON(w, http.StatusCreated, addTagsToChirp(chirp))
}

func (cfg *apiConfig) handlerGETChirps(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.db.GetChirps(r.Context())
	if err != nil {
		log.Print(fmt.Errorf("[ error ] getting chirps from DB: %w", err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't retrieve chirps", err)
		return
	}

	log.Print("[  ok   ] chirps served")
	chirpsWithTags := make([]Chirp, len(chirps))
	for i, chirp := range chirps {
		chirpsWithTags[i] = addTagsToChirp(chirp)
	}
	respondWithJSON(w, http.StatusOK, chirpsWithTags)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(fmt.Errorf("[ error ] loading .env file: %w", err))
	}

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(fmt.Errorf("[ error ] preparing database abstraction: %w", err))
	}
	defer db.Close()
	dbQueries := database.New(db)

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
		db:             dbQueries,
		platform:       os.Getenv("PLATFORM"),
		fileserverHits: atomic.Int32{},
	}

	// map server folders and routes for network access
	app := http.FileServer(http.Dir("./app"))
	assets := http.FileServer(http.Dir("./assets"))

	mux.Handle("/app/", apiCfg.middlewareMetricsIncrement(http.StripPrefix("/app/", app)))
	mux.Handle("/app/assets/", apiCfg.middlewareMetricsIncrement(http.StripPrefix("/app/assets/", assets)))
	mux.HandleFunc("GET /api/healthz", handlerHealth)
	mux.HandleFunc("GET /api/chirps", apiCfg.handlerGETChirps)
	mux.HandleFunc("POST /api/chirps", apiCfg.handlerChirps)
	mux.HandleFunc("POST /api/users", apiCfg.handlerUser)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)

	// start the server
	log.Printf("server is listening for requests on port %v\n", port)
	if err := server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			log.Fatal(fmt.Errorf("[ error ] server failed: %w", err))
		} else {
			log.Println("server exited gracefully")
		}
	}
}
