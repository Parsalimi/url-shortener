package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
	"url-shortener/internal/memory"
	"url-shortener/internal/postgres"
	"url-shortener/internal/shortener"

	"github.com/joho/godotenv"
)

type Server struct {
	shortener *shortener.Shortener
	base      string
}

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

func (srv *Server) shortenHandler(w http.ResponseWriter, r *http.Request) {
	// 	curl -s -X POST localhost:8080/api/shorten \
	//   -H 'Content-Type: application/json' \
	//   -d '{"url":"https://go.dev/doc/"}'
	var req shortenRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	code, err := srv.shortener.Shorten(req.URL)
	if err != nil {
		if errors.Is(err, shortener.ErrURLRequired) ||
			errors.Is(err, shortener.ErrInvalidURL) ||
			errors.Is(err, shortener.ErrUnsupportedScheme) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// 201 Created
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := shortenResponse{Code: code, ShortURL: srv.base + "/" + code}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
}

func (srv *Server) redirectHandler(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	longURL, err := srv.shortener.Redirect(code)
	if err != nil {
		if errors.Is(err, shortener.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, longURL.URL, http.StatusFound)
}

type linkResponse struct {
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

func (srv *Server) linkHandler(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	link, err := srv.shortener.Redirect(code)
	if err != nil {
		if errors.Is(err, shortener.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := linkResponse{URL: link.URL, CreatedAt: link.CreatedAt.Format(time.RFC3339)}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
}

func main() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "load .env: %v\n", err)
		os.Exit(1)
	}

	storeType := os.Getenv("STORE")
	dsn := os.Getenv("DATABASE_URL")

	// Get the entry from user
	// go run ./cmd/server -addr :8080 -base http://localhost:8080
	addr := flag.String("addr", ":8080", "HTTP server address")
	base := flag.String("base", "http://localhost:8080", "Base URL used in short URLs")

	flag.Parse()

	var store shortener.Store
	switch storeType {
	case "memory":
		/////////// MEMORY ///////////
		store = memory.NewInMemoryStore()

	case "postgres":
		////////// DATABASE //////////
		if dsn == "" {
			fmt.Fprintln(os.Stderr, "DATABASE_URL is not set")
			os.Exit(1)
		}

		db, err := postgres.Open(dsn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open database: %v\n", err)
			os.Exit(1)
		}

		if err := postgres.Migrate(db); err != nil {
			fmt.Fprintf(os.Stderr, "migrate database: %v\n", err)
			os.Exit(1)
		}
		store = postgres.NewPostgresStore(db)

	default:
		fmt.Fprintln(os.Stderr, "Please set STORE in the environment variables")
		os.Exit(1)
		/////////// FINISHED ///////////
	}

	srv := Server{shortener: shortener.New(store), base: *base}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/shorten", srv.shortenHandler)
	mux.HandleFunc("GET /{code}", srv.redirectHandler)
	mux.HandleFunc("GET /api/v1/links/{code}", srv.linkHandler)

	server := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	err := server.ListenAndServe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
