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
	"github.com/neira-daniel/go-chirpy/internal/auth"
	"github.com/neira-daniel/go-chirpy/internal/database"
)

const (
	successTag = "[ success ]"
	warningTag = "[ warning ]"
	errorTag   = "[  error  ]"
	okTag      = "[   ok    ]"
)

type User struct {
	Id           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
}

func addTagsToUser(user database.User, token string, refreshToken string) User {
	return User{
		Id:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refreshToken,
		IsChirpyRed:  user.IsChirpyRed,
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
	signingSecret  string
	polkaKey       string
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

func validateChirp(chirp string) error {
	const maxChirpLength = 140
	if chirpLength := len([]rune(chirp)); chirpLength > maxChirpLength {
		return fmt.Errorf("Chirp is %d characters longer than allowed", chirpLength-maxChirpLength)
	}
	return nil
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
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	var data jsonRequest
	err := decoder.Decode(&data)
	if err != nil {
		log.Print(fmt.Errorf("%v decoding non-conforming JSON request: %w", errorTag, err))
		respondWithError(w, http.StatusBadRequest, "non-conforming JSON received")
		return
	}

	hashedPassword, err := auth.HashPassword(data.Password)
	if err != nil {
		log.Print(fmt.Errorf("%v couldn't hash password for user %q: %w", errorTag, data.Email, err))
		respondWithError(w, http.StatusInternalServerError, "server error: couldn't hash password")
		return
	}

	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		Email:          data.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		log.Print(fmt.Errorf("%v creating new database user for %q: %w", errorTag, data.Email, err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't create user")
		return
	}

	log.Printf("%v user %q created", successTag, user.Email)
	respondWithJSON(w, http.StatusCreated, addTagsToUser(user, "", ""))
}

func (cfg *apiConfig) handlerChirps(w http.ResponseWriter, r *http.Request) {
	jwt, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Print(fmt.Errorf("%v getting bearer token: %w", warningTag, err))
		respondWithError(w, http.StatusBadRequest, "invalid request")
		return
	}
	userID, err := auth.ValidateJWT(jwt, cfg.signingSecret)
	if err != nil {
		log.Print(fmt.Errorf("%v validating JWT: %w", warningTag, err))
		respondWithError(w, http.StatusUnauthorized, "unauthorized action")
		return
	}

	type jsonRequest struct {
		Body string `json:"body"`
	}
	// we use JSON Decode instead of Unmarshal because we're dealing with a stream
	// of data instead of a []byte in memory
	decoder := json.NewDecoder(r.Body)
	var data jsonRequest
	if err := decoder.Decode(&data); err != nil {
		log.Print(fmt.Errorf("%v decoding non-conforming JSON request: %w", errorTag, err))
		respondWithError(w, http.StatusBadRequest, "non-conforming JSON received")
	}

	if err := validateChirp(data.Body); err != nil {
		log.Printf("%v invalid chirp", warningTag)
		respondWithError(w, http.StatusBadGateway, "invalid chirp")
		return
	}

	badWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}
	censoredChirp := censorChirp(data.Body, badWords)

	chirp, err := cfg.db.SaveChirp(r.Context(), database.SaveChirpParams{Body: censoredChirp, UserID: userID})
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

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	var data payload
	err := decoder.Decode(&data)
	if err != nil {
		log.Print(fmt.Errorf("%v decoding non-conforming JSON request: %w", errorTag, err))
		respondWithError(w, http.StatusBadRequest, "non-conforming JSON received")
		return
	}

	user, err := cfg.db.GetUserByEmail(r.Context(), data.Email)
	if err != nil {
		log.Print(fmt.Errorf("%v getting user from the database: %w", errorTag, err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't retrieve user")
		return
	}

	if err := auth.CheckPasswordHash(user.HashedPassword, data.Password); err != nil {
		log.Printf("%v wrong password for %q", warningTag, data.Email)
		respondWithError(w, http.StatusUnauthorized, "wrong password")
		return
	}

	JWTDuration := 1 * time.Hour
	jwt, err := auth.MakeJWT(user.ID, cfg.signingSecret, JWTDuration)
	if err != nil {
		log.Print(fmt.Errorf("%v couldn't sign token: %w", errorTag, err))
		respondWithError(w, http.StatusInternalServerError, "couldn't create authentication string")
		return
	}
	refreshToken, _ := auth.MakeRefreshToken()
	var refreshTokenDuration int32 = 60
	if err := cfg.db.StoreRefreshToken(r.Context(), database.StoreRefreshTokenParams{
		Token:  refreshToken,
		UserID: user.ID,
		Days:   refreshTokenDuration,
	}); err != nil {
		log.Print(fmt.Errorf("%v couldn't store refreshToken: %w", errorTag, err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't store refresh token")
		return
	}

	log.Printf("%v user %q has logged-in", successTag, data.Email)
	respondWithJSON(w, http.StatusOK, addTagsToUser(user, jwt, refreshToken))
}

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	refreshTokenReceived, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Print(fmt.Errorf("%v getting bearer token: %w", warningTag, err))
		respondWithError(w, http.StatusBadRequest, "invalid request")
		return
	}

	refreshTokenDB, err := cfg.db.GetRefreshToken(r.Context(), refreshTokenReceived)
	if err != nil || refreshTokenDB.RevokedAt.Valid || time.Now().UTC().After(refreshTokenDB.ExpiresAt) {
		log.Printf("%v got invalid refresh token", warningTag)
		respondWithError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	JWTDuration := 1 * time.Hour
	jwt, err := auth.MakeJWT(refreshTokenDB.UserID, cfg.signingSecret, JWTDuration)
	if err != nil {
		log.Print(fmt.Errorf("%v couldn't sign token: %w", errorTag, err))
		respondWithError(w, http.StatusInternalServerError, "couldn't create authentication string")
		return
	}

	type payload struct {
		Token string `json:"token"`
	}
	log.Printf("%v JWT renewed", successTag)
	respondWithJSON(w, http.StatusOK, payload{Token: jwt})
}

func (cfg *apiConfig) handlerRevokeAccess(w http.ResponseWriter, r *http.Request) {
	refreshTokenReceived, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Print(fmt.Errorf("%v getting bearer token: %w", warningTag, err))
		respondWithError(w, http.StatusBadRequest, "invalid request")
		return
	}

	result, err := cfg.db.RevokeAccess(r.Context(), refreshTokenReceived)
	if err != nil {
		log.Print(fmt.Errorf("%v couldn't revoke refresh token access: %w", errorTag, err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't revoke refresh token")
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Print(fmt.Errorf("%v couldn't inspect rows affected after revoking token: %w", errorTag, err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't verify that refresh token was revoked")
		return
	}

	if rowsAffected == 0 {
		log.Print(fmt.Errorf("%v invalid refresh token; couldn't revoke: %w", errorTag, err))
		respondWithError(w, http.StatusInternalServerError, "invalid bearer token")
		return
	}

	log.Printf("%v access revoked", successTag)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerUpdateCredentials(w http.ResponseWriter, r *http.Request) {
	jwt, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Print(fmt.Errorf("%v getting bearer token: %w", warningTag, err))
		respondWithError(w, http.StatusUnauthorized, "invalid request")
		return
	}
	userID, err := auth.ValidateJWT(jwt, cfg.signingSecret)
	if err != nil {
		log.Print(fmt.Errorf("%v validating JWT: %w", warningTag, err))
		respondWithError(w, http.StatusUnauthorized, "unauthorized action")
		return
	}

	type payload struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	var data payload
	if err := decoder.Decode(&data); err != nil {
		log.Print(fmt.Errorf("%v decoding non-conforming JSON request: %w", errorTag, err))
		respondWithError(w, http.StatusBadRequest, "non-conforming JSON received")
		return
	}

	hashedPassword, err := auth.HashPassword(data.Password)
	if err != nil {
		log.Print(fmt.Errorf("%v couldn't hash password: %w", errorTag, err))
		respondWithError(w, http.StatusInternalServerError, "server error: couldn't hash password")
		return
	}

	user, err := cfg.db.UpdateCredentials(r.Context(), database.UpdateCredentialsParams{
		Email:          data.Email,
		HashedPassword: hashedPassword,
		ID:             userID,
	})

	log.Printf("%v user %q created", successTag, user.Email)
	respondWithJSON(w, http.StatusOK, addTagsToUser(user, "", ""))
}

func (cfg *apiConfig) handlerDELETEChirpByID(w http.ResponseWriter, r *http.Request) {
	match := r.PathValue("chirpID")
	if match == "" {
		respondWithError(w, http.StatusBadRequest, "request error: missing chirp ID")
		return
	}

	chirpID, err := uuid.Parse(match)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "request error: not a valid chirp UUID")
		return
	}

	chirp, err := cfg.db.GetChirpByID(r.Context(), chirpID)
	if err != nil {
		log.Print(fmt.Errorf("%v chirp id=%q not found: %w", warningTag, chirpID, err))
		respondWithError(w, http.StatusNotFound, "chirp doesn't exist")
		return
	}

	jwt, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Print(fmt.Errorf("%v getting bearer token: %w", warningTag, err))
		respondWithError(w, http.StatusUnauthorized, "invalid request")
		return
	}
	userID, err := auth.ValidateJWT(jwt, cfg.signingSecret)
	if err != nil {
		log.Print(fmt.Errorf("%v validating JWT: %w", warningTag, err))
		respondWithError(w, http.StatusUnauthorized, "unauthorized action")
		return
	}

	if userID != chirp.UserID {
		log.Printf("%v user tried to delete chirp from another user", warningTag)
		respondWithError(w, http.StatusForbidden, "unauthorized action")
		return
	}

	if err := cfg.db.DeleteChirpByID(r.Context(), chirpID); err != nil {
		log.Print(fmt.Errorf("%v couldn't delete chirp from database: %w", errorTag, err))
		respondWithError(w, http.StatusInternalServerError, "database error: couldn't delete chirp")
		return
	}

	log.Printf("%v chirp %q deleted", successTag, chirpID)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerUpgradeUser(w http.ResponseWriter, r *http.Request) {
	polkaKey, err := auth.GetAPIKey(r.Header)
	if err != nil || polkaKey != cfg.polkaKey {
		log.Print(fmt.Errorf("%v getting api key: %w", warningTag, err))
		respondWithError(w, http.StatusUnauthorized, "invalid request")
		return
	}

	type payload struct {
		Event string `json:"event"`
		Data  struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(r.Body)
	var data payload
	if err := decoder.Decode(&data); err != nil {
		log.Print(fmt.Errorf("%v decoding non-conforming JSON request: %w", errorTag, err))
		respondWithError(w, http.StatusBadRequest, "non-conforming JSON received")
		return
	}

	if data.Event != "user.upgraded" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	userID, err := uuid.Parse(data.Data.UserID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "request error: not a valid user UUID")
		return
	}

	if _, err := cfg.db.UpgradeUser(r.Context(), userID); err != nil {
		log.Print(fmt.Errorf("%v couldn't upgrade user %q: %w", errorTag, userID, err))
		respondWithError(w, http.StatusNotFound, "couldn't upgrade user: user not found")
		return
	}

	log.Printf("%v user %q upgraded", successTag, userID)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(fmt.Errorf("%v loading .env file: %w", errorTag, err))
	}

	dbURL, ok := os.LookupEnv("DB_URL")
	if !ok || dbURL == "" {
		log.Fatal("connection string to the database was not found")
	}
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
	tokenSecret, ok := os.LookupEnv("JWTSECRET")
	if !ok || tokenSecret == "" {
		log.Fatal("suitable token secret to validate JWT not found")
	}
	polkaKey, ok := os.LookupEnv("POLKA_KEY")
	if !ok || polkaKey == "" {
		log.Fatal("suitable Polka key not found")
	}
	apiCfg := apiConfig{
		db:             dbQueries,
		platform:       os.Getenv("PLATFORM"),
		signingSecret:  tokenSecret,
		polkaKey:       polkaKey,
		fileserverHits: atomic.Int32{},
	}

	// map server folders and routes for network access
	app := http.FileServer(http.Dir("./app"))
	assets := http.FileServer(http.Dir("./assets"))

	mux.Handle("/app/", apiCfg.middlewareMetricsIncrement(http.StripPrefix("/app/", app)))
	mux.Handle("/app/assets/", apiCfg.middlewareMetricsIncrement(http.StripPrefix("/app/assets/", assets)))
	mux.HandleFunc("GET    /api/healthz", handlerHealth)
	mux.HandleFunc("GET    /api/chirps", apiCfg.handlerGETChirps)
	mux.HandleFunc("POST   /api/chirps", apiCfg.handlerChirps)
	mux.HandleFunc("GET    /api/chirps/{chirpID}", apiCfg.handlerGETChirpByID)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.handlerDELETEChirpByID)
	mux.HandleFunc("POST   /api/login", apiCfg.handlerLogin)
	mux.HandleFunc("POST   /api/polka/webhooks", apiCfg.handlerUpgradeUser)
	mux.HandleFunc("POST   /api/refresh", apiCfg.handlerRefresh)
	mux.HandleFunc("POST   /api/revoke", apiCfg.handlerRevokeAccess)
	mux.HandleFunc("POST   /api/users", apiCfg.handlerUser)
	mux.HandleFunc("PUT    /api/users", apiCfg.handlerUpdateCredentials)
	mux.HandleFunc("GET    /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST   /admin/reset", apiCfg.handlerReset)

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
