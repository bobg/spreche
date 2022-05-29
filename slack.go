package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/google/go-github/v44/github"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

var channelRegex = regexp.MustCompile(`^pr-([^/]+)/([^/]+)-(\d+)$`)

func (s *Service) OnSlackEvent(ctx context.Context, req *http.Request) error {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return errors.Wrap(err, "reading request body")
	}
	v, err := slack.NewSecretsVerifier(req.Header, s.SlackSecret)
	if err != nil {
		return errors.Wrap(err, "creating request verifier")
	}
	_, err = v.Write(body)
	if err != nil {
		return errors.Wrap(err, "writing request body to verifier")
	}
	if err = v.Ensure(); err != nil {
		return errors.Wrap(err, "verifying request signature")
	}
	ev, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		return errors.Wrap(err, "parsing request body")
	}
	switch ev.Type {
	case slackevents.URLVerification:
		return s.OnURLVerification(ctx, ev)

	case slackevents.CallbackEvent:
		switch ev := ev.Data.(type) {
		case *slackevents.MessageEvent:
			// xxx filter out bot messages (like the ones from this program!)
			return s.OnMessage(ctx, ev)

		case *slackevents.ReactionAddedEvent:
			return s.OnReactionAdded(ctx, ev)

		case *slackevents.ReactionRemovedEvent:
			return s.OnReactionRemoved(ctx, ev)
		}

		return fmt.Errorf("unknown data type %T for CallbackEvent", ev.Data)
	}
}

func (s *Service) OnURLVerification(ctx context.Context, ev slackevents.EventsAPIEvent) error {
	// xxx
	return nil
}

func (s *Service) OnMessage(ctx context.Context, ev *slackevents.MessageEvent) error {
	if ev.ChannelType != "channel" {
		return nil
	}
	m := channelRegex.FindStringSubmatch(s.Channel)
	if len(m) == 0 {
		return nil
	}
	owner, repo := m[1], m[2]
	prnum, err := strconv.Atoi(m[3])
	if err != nil {
		// xxx
	}

	// xxx filter out bot messages (like the ones from this program!)

	ghUser, err := s.SlackToGHUser(ev.User)
	if err != nil {
		// xxx
	}

	body := ev.Text // xxx convert Slack mrkdwn to GitHub Markdown

	comment := &github.PullRequestComment{
		Body: &body,
		User: ghUser,
	}
	timestamp := ev.ThreadTimeStamp
	if timestamp != "" {
		// Threaded reply.

		commentID, err := s.LookupGHCommentIDFromSlackTimestamp(ctx, timestamp)
		if err != nil {
			// xxx
		}
		comment, _, err = s.GHClient.PullRequests.CreateCommentInReplyTo(ctx, owner, repo, prnum, body, commentID)
		if err != nil {
			// xxx
		}
	} else {
		// Unthreaded.

		timestamp = ev.TimeStamp
		comment, _, err = s.GHClient.PullRequests.CreateComment(ctx, owner, repo, prnum, comment)
		if err != nil {
			// xxx
		}
		// xxx
	}

	// xxx update db - comment.ID is the new commentID associated with timestamp

	return nil
}

func (s *Service) OnReactionAdded(ctx context.Context, ev *slackevents.ReactionAddedEvent) error {
	// xxx
	return nil
}

func (s *Service) OnReactionRemoved(ctx context.Context, ev *slackevents.ReactionRemovedEvent) error {
	// xxx
	return nil
}

// TODO: cache results
func (s *Service) GetChannelID(ctx context.Context, name string) (string, error) {
	params := slack.GetConversationsParameters{Limit: 100}
	for {
		channels, next, err := s.SlackClient.GetConversationsContext(ctx, &params)
		if err != nil {
			// xxx
		}
		for _, channel := range channels {
			if channel.Name == name {
				return channel.ID, nil
			}
		}
		params.Cursor = next
	}
	return "", fmt.Errorf("channel %s not found", name)
}
