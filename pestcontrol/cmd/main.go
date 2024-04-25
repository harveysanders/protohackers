package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"

	"github.com/harveysanders/protohackers/pestcontrol"
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

	siteService := sqlite.NewSiteService(db.DB)

	srv := pestcontrol.NewServer(nil, config, siteService)
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
