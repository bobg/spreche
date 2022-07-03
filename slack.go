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

type eventBlocks struct {
	Event struct {
		Blocks []slack.Block `json:"blocks"`
	} `json:"event"`
}

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

		return s.Tenants.WithTenant(ctx, 0, "", teamID, func(ctx context.Context, tenant *Tenant) error {
			debugf("In OnSlackEvent, tenant ID %d", tenant.TenantID)

			gh, err := tenant.GHClient()
			if err != nil {
				return errors.Wrap(err, "getting GitHub client")
			}

			switch ev := ev.InnerEvent.Data.(type) {
			case *slackevents.MessageEvent:
				var evBlocks struct {
					Event struct {
						Blocks json.RawMessage `json:"blocks"`
					} `json:"event"`
				}
				var blocks []slack.Block
				if err = json.Unmarshal(body, &evBlocks); err == nil { // sic
					var b slack.Blocks
					if err = json.Unmarshal(evBlocks.Event.Blocks, &b); err == nil { // sic
						blocks = b.BlockSet
					}
				}
				if len(blocks) > 0 {
					debugf("Parsed blocks: %v", blocks)
				} else {
					debugf("Did not parse blocks")
				}
				return s.OnMessage(ctx, teamID, gh, ev, blocks)

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

func (s *Service) OnMessage(ctx context.Context, teamID string, gh *github.Client, ev *slackevents.MessageEvent, blocks []slack.Block) error {
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

	return s.Tenants.WithTenant(ctx, 0, "", teamID, func(ctx context.Context, tenant *Tenant) error {
		sc := tenant.SlackClient()

		channel, err := s.Channels.ByChannelID(ctx, tenant.TenantID, ev.Channel)
		if err != nil {
			return errors.Wrapf(err, "getting info for channelID %s", ev.Channel)
		}

		user, err := s.Users.BySlackID(ctx, tenant.TenantID, ev.User)
		if errors.Is(err, ErrNotFound) {
			debugf("Found no GitHub user for slack ID %s", ev.User)
			user = nil
		} else if err != nil {
			return errors.Wrapf(err, "getting info for userID %s", ev.User)
		} else {
			debugf("Found GitHub user %s for slack ID %s", user.GHLogin, ev.User)
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

		body := textOrBlocksToGH(commentURL, slackUser.Name, ev.Text, blocks)

		var ghuser *github.User
		if user != nil {
			ghuser = &github.User{Login: &user.GHLogin}
		}

		if ev.ThreadTimeStamp != "" {
			comment, err := s.Comments.ByThreadTimestamp(ctx, tenant.TenantID, channel.ChannelID, ev.ThreadTimeStamp)
			if err != nil {
				return errors.Wrapf(err, "getting latest comment in thread %s", ev.ThreadTimeStamp)
			}
			debugf("Creating comment (%s/%s/%d) in reply to %d", channel.Owner, channel.Repo, channel.PR, comment.CommentID)
			_, _, err = gh.PullRequests.CreateCommentInReplyTo(ctx, channel.Owner, channel.Repo, channel.PR, body, comment.CommentID)
			return errors.Wrap(err, "creating comment")
		}

		debugf("Creating new top-level comment (%s/%s/%d)", channel.Owner, channel.Repo, channel.PR)

		issueComment, _, err := gh.Issues.CreateComment(ctx, channel.Owner, channel.Repo, channel.PR, &github.IssueComment{
			Body: &body,
			User: ghuser,
		})
		if err != nil {
			return errors.Wrap(err, "creating comment")
		}

		return s.Comments.Add(ctx, tenant.TenantID, channel.ChannelID, ev.TimeStamp, *issueComment.ID)
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
