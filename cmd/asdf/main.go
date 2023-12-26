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

	keyPath := os.Getenv("SSL_KEY_PATH")
	certPath := os.Getenv("SSL_CERT_PATH")
	if certPath == "" || keyPath == "" {
		log.Fatal("SSL certificate or key path not set in environment variables")
	}

	server.Start(":"+port, certPath, keyPath)
}
