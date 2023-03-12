package main

import (
	"fmt"
	"log"
	"os"

	m2e "github.com/harveysanders/protohackers/meanstoend"
)

func main() {
	port := "9002"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}

	srv := m2e.Server{}
	fmt.Printf("Starting server on port: %s\n", port)

	if err := srv.Start(port); err != nil {
		log.Fatal(err)
	}

	if err := srv.Stop(); err != nil {
		log.Fatal(err)
	}
}
