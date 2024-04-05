package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/harveysanders/protohackers/pestcontrol"
	"github.com/harveysanders/protohackers/pestcontrol/inmem"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	config := pestcontrol.ServerConfig{
		AuthServerAddr: "pestcontrol.protohackers.com:20547",
	}

	port := "9000"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}

	logFilePath := os.Getenv("LOG_FILE")
	logger := log.Default()
	if logFilePath != "" {
		logFile, err := os.Create(logFilePath)
		if err != nil {
			return err
		}
		defer logFile.Close()

		logger = log.New(logFile, "", log.LstdFlags|log.Lshortfile)
	}

	store := inmem.NewStore()
	srv := pestcontrol.NewServer(logger, config, store)
	srvErr := make(chan error)

	go func() {
		addr := fmt.Sprintf(":%s", port)
		log.Printf("server starting @: %s\n", addr)
		err := srv.ListenAndServe(addr)
		if err != nil {
			srvErr <- err
		}
	}()
	defer srv.Close()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case err := <-srvErr:
		return err
	}

}
