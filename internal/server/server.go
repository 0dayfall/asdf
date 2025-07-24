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

const WELL_KNOWN_WEBFINGER = "/.well-known/webfinger"

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

	s := store.NewPostgresStore(pool)

	if os.Getenv("GO_ENV") == "test" {
		err = s.InitSchemaAndSeed(context.Background())
		if err != nil {
			log.Fatalf("DB setup failed: %v", err)
		}
	}

	mux := http.NewServeMux()

	htmlHandler := &rest.HTMLHandler{Data: s}
	rest.LoadTemplates()

	mux.Handle("/", htmlHandler)

	if os.Getenv("GO_ENV") == "test" {
		log.Println("Running in test mode, using HTTP instead of HTTPS")
		addr = "localhost:8080"
		err = http.ListenAndServe(addr, mux)
		if err != nil {
			log.Fatalf("Server failed: %v", err)
		}
		log.Printf("WebFinger server running on %s", addr)
	} else {
		log.Printf("Running in production mode, serving on %s", addr)
		err = http.ListenAndServeTLS(addr, certPath, keyPath, mux)
		if err != nil {
			log.Fatalf("Server failed: %v", err)
		}
		log.Printf("WebFinger server running on %s with TLS", addr)
	}
}
