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

	server.Start(":" + port)
}
