package main

import (
	"log"
	"os"

	jobcentre "github.com/harveysanders/protohackers/9-job-centre"
	"github.com/harveysanders/protohackers/9-job-centre/inmem"
	"github.com/harveysanders/protohackers/9-job-centre/jcp"
)

func main() {
	port := "9999"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	store := inmem.NewStore()
	srv := &jcp.Server{
		Addr:    ":" + port,
		Handler: jobcentre.NewApp(store),
	}

	srv.SetLogger(log.New(os.Stdout, "job-centre: ", log.LstdFlags))

	log.Print("Listening on port " + port)
	err := srv.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
