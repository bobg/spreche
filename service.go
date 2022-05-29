package main

import (
	"database/sql"

	"github.com/google/go-github/v44/github"
	"github.com/slack-go/slack"
)

type Service struct {
	GHSecret    []byte
	SlackSecret string
	GHClient    *github.Client
	SlackClient *slack.Client
	DB          *sql.DB
}
