package main

import (
	"log"
	"os"

	fingerServer "github.com/0dayfall/asdf"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}

	fingerServer.Start(":" + port)
}
