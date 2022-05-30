package main

import (
	"database/sql"
	"errors"
	"flag"
	"log"
	"net/http"

	"github.com/bobg/mid"
	"github.com/google/go-github/v44/github"
	_ "github.com/mattn/go-sqlite3"
	"github.com/slack-go/slack"

	"crocs"
)

func main() {
	var (
		addr        = flag.String("addr", ":3853", "listen address")
		certfile    = flag.String("certfile", "", "TLS cert file")
		keyfile     = flag.String("keyfile", "", "TLS key file")
		ghSecret    = flag.String("ghsecret", "", "GitHub secret")
		slackSecret = flag.String("slacksecret", "", "Slack secret")
		slackToken  = flag.String("slacktoken", "", "Slack token")
		ghURL       = flag.String("github", "https://github.com", "GitHub server URL")
		dbstr       = flag.String("db", "crocs.db", "path to crocs Sqlite3 database")
	)
	flag.Parse()

	ghClient, err := github.NewEnterpriseClient(*ghURL, *ghURL, nil)
	if err != nil {
		log.Fatalf("Creating GitHub client: %s", err)
	}

	slackClient := slack.New(*slackToken)

	db, err := sql.Open("sqlite3", *dbstr)
	if err != nil {
		log.Fatal(err)
	}

	s := &crocs.Service{
		GHSecret:    []byte(*ghSecret), // xxx does this need base64-encoding?
		SlackSecret: *slackSecret,
		GHClient:    ghClient,
		SlackClient: slackClient,
		DB:          db,
	}

	mux := http.NewServeMux()
	mux.Handle("/github", mid.Err(s.OnGHWebhook))
	mux.Handle("/slack", mid.Err(s.OnSlackEvent))

	httpServer := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}

	if *certfile != "" && *keyfile != "" {
		err = httpServer.ListenAndServeTLS(*certfile, *keyfile)
	} else {
		err = httpServer.ListenAndServe()
	}
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
