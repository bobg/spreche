package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"

	"github.com/bobg/mid"
	"github.com/google/go-github/v44/github"
	_ "github.com/mattn/go-sqlite3"
	"github.com/slack-go/slack"
)

func main() {
	var (
		ghSecret    = flag.String("ghsecret", "", "GitHub secret")
		slackSecret = flag.String("slacksecret", "", "Slack secret")
		slackToken  = flag.String("slacktoken", "", "Slack token")
		ghURL       = flag.String("github", "https://github.com", "GitHub server URL")
		dbstr       = flag.String("db", "crocs.db", "path to crocs Sqlite3 database")
	)
	flag.Parse()

	ghClient, err := github.NewEnterpriseClient(*ghURL, *ghURL, nil)
	if err != nil {
		// xxx
	}

	slackClient := slack.New(*slackToken)

	db, err := sql.Open("sqlite3", *dbstr)
	if err != nil {
		log.Fatal(err)
	}

	s := &Service{
		GHSecret:    []byte(*ghSecret), // xxx does this need base64-encoding?
		SlackSecret: *slackSecret,
		GHClient:    ghClient,
		SlackClient: slackClient,
		DB:          db,
	}

	http.Handle("/github", mid.Err(s.OnGHWebhook))
	http.Handle("/slack", mid.Err(s.OnSlackEvent))
}
