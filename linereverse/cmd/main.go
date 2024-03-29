package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/harveysanders/protohackers/linereverse"
)

func main() {
	host := "fly-global-services"
	if HOST := os.Getenv("HOST"); HOST != "" {
		host = HOST
	}

	port := "9999"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}

	address := fmt.Sprintf("%s:%s", host, port)
	log.Printf("LRCP server starting @: %s", address)

	app := linereverse.New()
	if err := app.Run(context.TODO(), address); err != nil {
		log.Fatal(err)
	}
}
