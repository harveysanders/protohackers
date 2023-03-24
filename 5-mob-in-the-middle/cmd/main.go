package main

import (
	"log"
	"os"

	"github.com/harveysanders/protohackers/mobprox"
)

func main() {
	port := "5000"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}

	upstreamAddr := "chat.protohackers.com:16963"
	if UPSTREAM := os.Getenv("UPSTREAM"); UPSTREAM != "" {
		upstreamAddr = UPSTREAM
	}
	tonyBcoinAddress := "7YWHMfk9JZe0LM0g1ZauHuiSxhI"
	srv := mobprox.NewServer(upstreamAddr, tonyBcoinAddress)

	log.Printf("Mob Proxy starting on port: %s", port)

	if err := srv.Start(port); err != nil {
		log.Fatal(err)
	}
}
