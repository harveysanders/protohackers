package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/harveysanders/protohackers/udb"
)

func main() {
	store := udb.NewStoreMap()
	srv := udb.NewServer(store)
	host := "fly-global-services"
	if HOST := os.Getenv("HOST"); HOST != "" {
		host = HOST
	}

	port := "9002"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}

	address := fmt.Sprintf("%s:%s", host, port)
	log.Printf("UDP DB server starting @: %s\n", address)

	if err := srv.ServeUDP(context.Background(), address); err != nil {
		log.Fatal(err)
	}

}
