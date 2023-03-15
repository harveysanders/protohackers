package main

import (
	"log"
	"os"

	chat "github.com/harveysanders/protohackers/budgetchat"
)

func main() {
	port := "9002"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}

	srv := chat.Server{}
	log.Printf("Starting server on port: %s\n", port)

	if err := srv.Start(port); err != nil {
		log.Fatal(err)
	}

	if err := srv.Stop(); err != nil {
		log.Fatal(err)
	}
}
