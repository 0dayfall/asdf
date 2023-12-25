package main

import (
	"asdf/internal/server"
	"log"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}

	certPath := os.Getenv("SSL_CERT_PATH")
	keyPath := os.Getenv("SSL_KEY_PATH")
	if certPath == "" || keyPath == "" {
		log.Fatal("SSL certificate or key path not set in environment variables")
	}

	server.Start(":"+port, certPath, keyPath)
}
