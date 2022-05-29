package main

import (
	"context"

	"github.com/bobg/mid"
	"github.com/google/go-github/v44/github"
	"github.com/slack-go/slack"
	"golang.org/x/oauth2"
)

func main() {
	ctx := context.Background()
	httpClient := oauth2.NewClient(ctx, tokenSource)

	var ghClient *github.Client
	if isEnterprise {
		ghClient, err = github.NewEnterpriseClient(baseURL, uploadURL, httpClient)
		if err != nil {
			// xxx
		}
	} else {
		ghClient = github.NewClient(httpClient)
	}

	slackClient := slack.New(slackToken)

	s := &Service{
		GHSecret:    ghSecret,
		SlackSecret: slackSecret,
		GHClient:    ghClient,
		SlackClient: slackClient,
	}

	http.Handle("/github", mid.Err(s.OnGHWebHook))
}
