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

	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		log.Fatal("$SESSION_KEY environment variable is not set")
	}

	server.Start(":"+port, sessionKey)
}
