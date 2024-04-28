package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"

	"github.com/harveysanders/protohackers/pestcontrol"
	plog "github.com/harveysanders/protohackers/pestcontrol/log"
	"github.com/harveysanders/protohackers/pestcontrol/sqlite"
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

	dsn := "memory:"
	if DSN := os.Getenv("DSN"); DSN != "" {
		dsn = DSN
	}

	dropTables, _ := strconv.ParseBool(os.Getenv("DROP_TABLES"))
	db := sqlite.NewDB(dsn)
	if err := db.Open(dropTables); err != nil {
		return fmt.Errorf("db.Open: %w", err)
	}

	logFilePath := os.Getenv("LOG_FILE")
	logFile := io.Discard
	if logFilePath != "" {
		f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open log file: %w", err)
		}
		defer f.Close()
		logFile = f
	}

	logger := slog.New(
		slog.NewTextHandler(
			plog.SustainedMultiWriter(os.Stderr, logFile),
			nil),
	).With("name", "PestcontrolServer")

	siteService := sqlite.NewSiteService(db.DB)

	srv := pestcontrol.NewServer(logger, config, siteService)
	srvErr := make(chan error)

	go func() {
		addr := fmt.Sprintf(":%s", port)
		log.Printf("pestcontrol server starting @: %s\n", addr)
		err := srv.ListenAndServe(addr)
		if err != nil {
			srvErr <- err
		}
	}()
	defer srv.Close()
	defer db.Close()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case err := <-srvErr:
		return err
	}

}
