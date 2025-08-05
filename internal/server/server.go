package server

import (
	"asdf/internal/rest"
	"asdf/internal/store"
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

const wellKnownWebFinger = "/.well-known/webfinger"

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func Start(addr, certPath, keyPath string) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer pool.Close()

	store := store.NewPostgresStore(pool)

	if os.Getenv("GO_ENV") == "test" {
		if err := store.InitSchemaAndSeed(context.Background()); err != nil {
			log.Fatalf("DB setup failed: %v", err)
		}
	}

	rest.LoadTemplates()

	mux := http.NewServeMux()
	htmlHandler := &rest.HTMLHandler{Data: store}

	// Static assets
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// API endpoints
	mux.HandleFunc(wellKnownWebFinger, htmlHandler.HandleWebFinger)
	mux.HandleFunc("/api/search", htmlHandler.HandleSearchAPI)

	// HTML frontend
	mux.Handle("/", htmlHandler)

	runServer(mux, addr, certPath, keyPath)
}

func runServer(mux *http.ServeMux, addr, certPath, keyPath string) {
	if os.Getenv("GO_ENV") == "test" {
		log.Println("Running in test mode, using HTTP")
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	} else {
		log.Printf("Running in production mode on %s with TLS", addr)
		if err := http.ListenAndServeTLS(addr, certPath, keyPath, mux); err != nil {
			log.Fatalf("HTTPS server failed: %v", err)
		}
	}
}
