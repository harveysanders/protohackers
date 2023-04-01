package main

import (
	"context"
	"log"
	"os"

	"github.com/harveysanders/protohackers/spdaemon"
)

func main() {
	port := "9999"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}

	srv := spdaemon.NewServer()
	if err := srv.Start(context.Background(), port); err != nil {
		log.Fatal(err)
	}
}
