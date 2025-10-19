package main

import (
	"asdf/internal/server"
	"log"
)

func main() {
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
