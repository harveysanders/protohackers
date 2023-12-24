package main

import (
	"log"
	"os"

	vcs "github.com/harveysanders/protohackers/10-voracious-code-storage"
)

func main() {

	port := "9876"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}
	srv := vcs.New()

	l, err := srv.Start(":" + port)
	if err != nil {
		log.Fatal(err)
	}

	defer l.Close()

}
