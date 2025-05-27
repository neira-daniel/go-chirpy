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

const (
	successTag = "[ success ]"
	warningTag = "[ warning ]"
	errorTag   = "[  error  ]"
	okTag      = "[   ok    ]"
)

type User struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func addTagsToUser(user database.User) User {
	return User{
		Id:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}
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
		log.Printf("%v resetting the database", errorTag)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		cfg.fileserverHits.Store(0)
		log.Printf("%v all database records were cleared", successTag)
		w.WriteHeader(http.StatusOK)
	}
}

func handlerHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func respondWithError(w http.ResponseWriter, statusCode int, message string) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, statusCode, errorResponse{Error: message})
}

func respondWithJSON(w http.ResponseWriter, statusCode int, payload any) {
	jsonResponse, err := json.Marshal(payload)
	if err != nil {
		log.Print(fmt.Errorf("%v encoding JSON response: %w", errorTag, err))
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
		log.Print(fmt.Errorf("%v decoding non-conforming JSON request: %w", errorTag, err))
		respondWithError(w, http.StatusBadRequest, "non-conforming JSON received")
		return "", uuid.Nil, false
	}

	const maxChirpLength = 140
	if chirpLength := len([]rune(data.Body)); chirpLength > maxChirpLength {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Chirp is %d characters longer than allowed", chirpLength-maxChirpLength))
		return "", uuid.Nil, false
	}

	badWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}
	cleanedChirp := censorChirp(data.Body, badWords)

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
		log.Print(fmt.Errorf("%v decoding non-conforming JSON request: %w", errorTag, err))
		respondWithError(w, http.StatusBadRequest, "non-conforming JSON received")
		return
	}

	user, err := cfg.db.CreateUser(r.Context(), data.Email)
	if err != nil {
		log.Print(fmt.Errorf("%v creating new database user for %q: %w", errorTag, data.Email, err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't create user")
		return
	}

	log.Printf("%v user %q created", successTag, user.Email)
	respondWithJSON(w, http.StatusCreated, addTagsToUser(user))
}

func (cfg *apiConfig) handlerChirps(w http.ResponseWriter, r *http.Request) {
	body, user_id, ok := validateChirp(w, r)
	if !ok {
		return
	}

	chirp, err := cfg.db.SaveChirp(r.Context(), database.SaveChirpParams{Body: body, UserID: user_id})
	if err != nil {
		log.Print(fmt.Errorf("%v storing chirp in the database: %w", errorTag, err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't store chirp")
		return
	}

	log.Printf("%v chirp stored in the database", successTag)
	respondWithJSON(w, http.StatusCreated, addTagsToChirp(chirp))
}

func (cfg *apiConfig) handlerGETChirps(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.db.GetChirps(r.Context())
	if err != nil {
		log.Print(fmt.Errorf("%v getting chirps from the database: %w", errorTag, err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't retrieve chirps")
		return
	}

	chirpsWithTags := make([]Chirp, len(chirps))
	for i, chirp := range chirps {
		chirpsWithTags[i] = addTagsToChirp(chirp)
	}
	respondWithJSON(w, http.StatusOK, chirpsWithTags)
}

func (cfg *apiConfig) handlerGETChirpByID(w http.ResponseWriter, r *http.Request) {
	match := r.PathValue("chirpID")
	if match == "" {
		respondWithError(w, http.StatusBadRequest, "request error: missing chirp ID")
		return
	}

	chirp_id, err := uuid.Parse(match)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "request error: not a valid chirp UUID")
		return
	}

	chirp, err := cfg.db.GetChirpByID(r.Context(), chirp_id)
	if err != nil {
		log.Print(fmt.Errorf("%v chirp id=%q not found: %w", warningTag, chirp_id, err))
		respondWithError(w, http.StatusNotFound, "chirp doesn't exist")
		return
	}

	respondWithJSON(w, http.StatusOK, addTagsToChirp(chirp))
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(fmt.Errorf("%v loading .env file: %w", errorTag, err))
	}

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(fmt.Errorf("%v preparing database abstraction: %w", errorTag, err))
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
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handlerGETChirpByID)
	mux.HandleFunc("POST /api/chirps", apiCfg.handlerChirps)
	mux.HandleFunc("POST /api/users", apiCfg.handlerUser)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)

	// start the server
	log.Printf("server is listening for requests on port %v\n", port)
	if err := server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			log.Fatal(fmt.Errorf("%v server failed: %w", errorTag, err))
		} else {
			log.Println("server exited gracefully")
		}
	}
}
