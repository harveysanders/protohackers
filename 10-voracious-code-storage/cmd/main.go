package main

import (
	"fmt"
	"log"
	"os"

	vcs "github.com/harveysanders/protohackers/10-voracious-code-storage"
)

func main() {
	port := "9876"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}

	addr := fmt.Sprintf(":%s", port)
	srv := vcs.New()

	fmt.Printf("Voracious Code Storage server starting on %s...\n", addr)
	err := srv.Start(addr)
	if err != nil {
		log.Fatal(err)
	}

	_ = srv.Close()
}
