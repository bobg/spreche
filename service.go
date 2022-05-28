package crocs

import "github.com/slack-go/slack"

type Service struct {
	GHSecret []byte
	SlackClient *slack.Client
}
