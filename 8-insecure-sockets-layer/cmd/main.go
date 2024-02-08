package main

import (
	"context"
	"log"
	"os"

	isl "github.com/harveysanders/protohackers/8-insecure-sockets-layer"
)

func main() {
	port := "9999"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}

	srv := isl.Server{}
	if err := srv.Start(port); err != nil {
		log.Fatal(err)
	}
	if err := srv.Serve(context.Background()); err != nil {
		log.Fatal(err)
	}
}
