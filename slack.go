package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/bobg/mid"
	"github.com/google/go-github/v44/github"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

var channelRegex = regexp.MustCompile(`^pr-([^/]+)/([^/]+)-(\d+)$`)

func (s *Service) OnSlackEvent(w http.ResponseWriter, req *http.Request) error {
	ctx := req.Context()

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
		return s.OnURLVerification(w, ev)

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

	// Ignore other event types. (xxx log them?)
	return nil
}

func (s *Service) OnURLVerification(w http.ResponseWriter, ev slackevents.EventsAPIEvent) error {
	v, ok := ev.Data.(*slackevents.EventsAPIURLVerificationEvent)
	if !ok {
		// xxx
	}
	return mid.RespondJSON(w, slackevents.ChallengeResponse{Challenge: v.Challenge})
}

func (s *Service) OnMessage(ctx context.Context, ev *slackevents.MessageEvent) error {
	if ev.ChannelType != "channel" {
		return nil
	}

	channelID := ev.Channel
	channelName, err := s.GetChannelName(ctx, channelID)
	if err != nil {
		// xxx
	}

	m := channelRegex.FindStringSubmatch(channelName)
	if len(m) == 0 {
		return nil
	}
	owner, repo := m[1], m[2]
	prnum, err := strconv.Atoi(m[3])
	if err != nil {
		// xxx
	}

	// xxx filter out bot messages (like the ones from this program!)

	ghUser, err := s.SlackToGHUser(ctx, ev.User)
	if err != nil {
		// xxx
	}

	body := ev.Text // xxx convert Slack mrkdwn to GitHub Markdown

	comment := &github.PullRequestComment{
		Body: &body,
		User: &github.User{ // xxx ?
			Login: &ghUser,
		},
	}
	timestamp := ev.ThreadTimeStamp
	if timestamp != "" {
		// Threaded reply.

		commentID, err := s.LookupGHCommentIDFromSlackTimestamp(ctx, channelID, timestamp)
		if err != nil {
			// xxx
		}
		comment.InReplyTo = &commentID
	} else {
		timestamp = ev.TimeStamp
	}
	comment, _, err = s.GHClient.PullRequests.CreateComment(ctx, owner, repo, prnum, comment)
	if err != nil {
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
		if next == "" {
			return "", fmt.Errorf("channel %s not found", name)
		}
		params.Cursor = next
	}
}

// TODO: cache results
func (s *Service) GetChannelName(ctx context.Context, channelID string) (string, error) {
	ch, err := s.SlackClient.GetConversationInfoContext(ctx, channelID, false)
	if err != nil {
		// xxx
	}
	return ch.Name, nil
}
