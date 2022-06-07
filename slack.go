package spreche

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bobg/mid"
	"github.com/google/go-github/v44/github"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func (s *Service) OnSlackEvent(w http.ResponseWriter, req *http.Request) error {
	ctx := req.Context()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return errors.Wrap(err, "reading request body")
	}
	v, err := slack.NewSecretsVerifier(req.Header, s.SlackSigningSecret)
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
		switch ev := ev.InnerEvent.Data.(type) {
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
		return fmt.Errorf("unexpected data type %T", ev.Data)
	}
	return mid.RespondJSON(w, slackevents.ChallengeResponse{Challenge: v.Challenge})
}

func (s *Service) OnMessage(ctx context.Context, ev *slackevents.MessageEvent) error {
	if ev.ChannelType != "channel" {
		return nil
	}

	channel, err := s.Channels.ByChannelID(ctx, ev.Channel)
	if err != nil {
		return errors.Wrapf(err, "getting info for channelID %s", ev.Channel)
	}

	// xxx filter out bot messages (like the ones from this program!)

	user, err := s.Users.BySlackID(ctx, ev.User)
	if errors.Is(err, ErrNotFound) {
		user = nil
	} else if err != nil {
		return errors.Wrapf(err, "getting info for userID %s", ev.User)
	}

	body := ev.Text // xxx convert Slack mrkdwn to GitHub Markdown

	var ghuser *github.User
	if user != nil {
		ghuser = &github.User{Login: &user.GithubName}
	}

	var (
		timestamp = ev.ThreadTimeStamp
		commentID int64
	)
	if timestamp != "" {
		// Threaded reply.

		comment, err := s.Comments.ByThreadTimestamp(ctx, channel.ChannelID, timestamp)
		if err != nil {
			return errors.Wrapf(err, "getting latest comment in thread %s", timestamp)
		}
		prComment, _, err := s.GHClient.PullRequests.CreateComment(ctx, channel.Owner, channel.Repo, channel.PR, &github.PullRequestComment{
			Body:      &body,
			User:      ghuser,
			InReplyTo: &comment.CommentID,
		})
		if err != nil {
			return errors.Wrap(err, "creating comment")
		}
		commentID = *prComment.ID
	} else {
		timestamp = ev.TimeStamp
		issueComment, _, err := s.GHClient.Issues.CreateComment(ctx, channel.Owner, channel.Repo, channel.PR, &github.IssueComment{
			Body: &body,
			User: ghuser,
		})
		if err != nil {
			return errors.Wrap(err, "creating comment")
		}
		commentID = *issueComment.ID
	}

	return s.Comments.Update(ctx, channel.ChannelID, timestamp, commentID)
}

func (s *Service) OnReactionAdded(ctx context.Context, ev *slackevents.ReactionAddedEvent) error {
	// xxx
	return nil
}

func (s *Service) OnReactionRemoved(ctx context.Context, ev *slackevents.ReactionRemovedEvent) error {
	// xxx
	return nil
}
