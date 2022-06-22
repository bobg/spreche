package spreche

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bobg/mid"
	"github.com/google/go-github/v45/github"
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
		teamID := ev.TeamID

		return s.Tenants.WithTenant(ctx, "", teamID, func(ctx context.Context, tenant *Tenant) error {
			gh, err := tenant.GHClient()
			if err != nil {
				return errors.Wrap(err, "getting GitHub client")
			}

			switch ev := ev.InnerEvent.Data.(type) {
			case *slackevents.MessageEvent:
				return s.OnMessage(ctx, teamID, gh, ev)

			case *slackevents.ReactionAddedEvent:
				return s.OnReactionAdded(ctx, gh, ev)

			case *slackevents.ReactionRemovedEvent:
				return s.OnReactionRemoved(ctx, gh, ev)
			}

			return fmt.Errorf("unknown data type %T for CallbackEvent", ev.Data)
		})
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

func (s *Service) OnMessage(ctx context.Context, teamID string, gh *github.Client, ev *slackevents.MessageEvent) error {
	if ev.ChannelType != "channel" {
		return nil
	}
	if ev.BotID != "" {
		return nil
	}
	switch ev.SubType {
	case "channel_join", "channel_topic":
		return nil
	case "message_changed":
		if ev.Message != nil && ev.Message.BotID != "" {
			return nil
		}
	}

	return s.Tenants.WithTenant(ctx, "", teamID, func(ctx context.Context, tenant *Tenant) error {
		sc := tenant.SlackClient()

		channel, err := s.Channels.ByChannelID(ctx, ev.Channel)
		if err != nil {
			return errors.Wrapf(err, "getting info for channelID %s", ev.Channel)
		}

		user, err := s.Users.BySlackID(ctx, ev.User)
		if errors.Is(err, ErrNotFound) {
			user = nil
		} else if err != nil {
			return errors.Wrapf(err, "getting info for userID %s", ev.User)
		}

		slackUser, err := sc.GetUserInfoContext(ctx, ev.User)
		if err != nil {
			return errors.Wrapf(err, "getting Slack info for user %s", ev.User)
		}

		team, err := sc.GetTeamInfoContext(ctx)
		if err != nil {
			return errors.Wrap(err, "getting team info")
		}

		// Reverse-engineered Slack-comment link.
		eventID := ev.EventTimeStamp.String()
		eventID = strings.Replace(eventID, ".", "", -1)
		commentURL := fmt.Sprintf("https://%s.slack.com/archives/%s/p%s", team.Domain, ev.Channel, eventID)
		if ev.ThreadTimeStamp != "" {
			commentURL += fmt.Sprintf("?thread_ts=%s&cid=%s", ev.ThreadTimeStamp, ev.Channel)
		}

		// xxx convert Slack mrkdwn to GitHub Markdown
		body := fmt.Sprintf("_[[comment](%s) from %s]_\n\n%s", commentURL, slackUser.Name, ev.Text)

		var ghuser *github.User
		if user != nil {
			ghuser = &github.User{Login: &user.GithubName}
		}

		if ev.ThreadTimeStamp != "" {
			comment, err := s.Comments.ByThreadTimestamp(ctx, channel.ChannelID, ev.ThreadTimeStamp)
			if err != nil {
				return errors.Wrapf(err, "getting latest comment in thread %s", ev.ThreadTimeStamp)
			}
			_, _, err = gh.PullRequests.CreateCommentInReplyTo(ctx, channel.Owner, channel.Repo, channel.PR, body, comment.CommentID)
			return errors.Wrap(err, "creating comment")
		}

		issueComment, _, err := gh.Issues.CreateComment(ctx, channel.Owner, channel.Repo, channel.PR, &github.IssueComment{
			Body: &body,
			User: ghuser,
		})
		if err != nil {
			return errors.Wrap(err, "creating comment")
		}
		return s.Comments.Add(ctx, channel.ChannelID, ev.TimeStamp, *issueComment.ID)
	})
}

func (s *Service) OnReactionAdded(ctx context.Context, gh *github.Client, ev *slackevents.ReactionAddedEvent) error {
	// xxx
	return nil
}

func (s *Service) OnReactionRemoved(ctx context.Context, gh *github.Client, ev *slackevents.ReactionRemovedEvent) error {
	// xxx
	return nil
}
