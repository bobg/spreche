package crocs

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v44/github"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
)

func (s *Service) OnGHWebhook(w http.ResponseWriter, req *http.Request) error {
	ctx := req.Context()

	payload, err := github.ValidatePayload(req, s.GHSecret)
	if err != nil {
		return errors.Wrap(err, "validating webhook payload")
	}
	typ := github.WebHookType(req)
	ev, err := github.ParseWebHook(typ, payload)
	if err != nil {
		return errors.Wrap(err, "parsing webhook payload")
	}
	switch ev := ev.(type) {
	case *github.PullRequestEvent:
		return s.OnPR(ctx, ev)

	case *github.PullRequestReviewEvent:
		return s.OnPRReview(ctx, ev)

	case *github.PullRequestReviewCommentEvent:
		return s.OnPRReviewComment(ctx, ev)

	case *github.PullRequestReviewThreadEvent:
		return s.OnPRReviewThread(ctx, ev)
	}

	return fmt.Errorf("unknown webhook payload type %T", ev)
}

func (s *Service) OnPR(ctx context.Context, ev *github.PullRequestEvent) error {
	switch ev.GetAction() {
	case "opened":
		return s.PROpened(ctx, ev)

	case "edited":
		return s.PREdited(ctx, ev)

	case "closed":
		return s.PRClosed(ctx, ev)

	case "reopened":
		return s.PRReopened(ctx, ev)

	case "assigned":
		return s.PRAssigned(ctx, ev)

	case "unassigned":
		return s.PRUnassigned(ctx, ev)

	case "review_requested":
		return s.PRReviewRequested(ctx, ev)

	case "review_request_removed":
		return s.PRReviewRequestRemoved(ctx, ev)

	case "labeled":
		return s.PRReviewRequestLabeled(ctx, ev)

	case "unlabeled":
		return s.PRReviewRequestUnlabeled(ctx, ev)

	case "synchronize":
		return s.PRReviewRequestSynchronize(ctx, ev)
	}

	return fmt.Errorf("unknown PR event action %s", ev.GetAction())
}

func (s *Service) PROpened(ctx context.Context, ev *github.PullRequestEvent) error {
	var (
		repo = ev.Repo
		pr   = ev.PullRequest
	)

	chname := fmt.Sprintf("pr-%s-%d", *repo.FullName, *ev.Number)
	ch, err := s.SlackClient.CreateConversationContext(ctx, chname, false)
	if err != nil {
		return errors.Wrapf(err, "creating channel %s", chname)
	}

	topic := fmt.Sprintf("Discussion of %s: %s", *pr.URL, *pr.Title)
	_, err = s.SlackClient.SetTopicOfConversationContext(ctx, ch.ID, topic)
	if err != nil {
		return errors.Wrapf(err, "setting topic of channel %s", chname)
	}

	ghUsers := []*github.User{
		pr.User,
		pr.Assignee,
	}
	ghUsers = append(ghUsers, pr.Assignees...)
	ghUsers = append(ghUsers, pr.RequestedReviewers...)
	// xxx also pr.RequestedTeams

	// xxx bleh
	slackUsers, err := s.GHToSlackUsers(ctx, ghUsers)
	if err != nil {
		// xxx
	}

	if len(slackUsers) > 0 {
		_, err = s.SlackClient.InviteUsersToConversationContext(ctx, ch.ID, slackUsers...)
		if err != nil {
			// xxx
		}
	}

	postOptions := []slack.MsgOption{
		slack.MsgOptionText(*pr.Body, false), // xxx convert GH Markdown to Slack mrkdwn (using https://github.com/eritikass/githubmarkdownconvertergo ?)
	}
	_, msgTimestamp, err := s.SlackClient.PostMessageContext(ctx, ch.ID, postOptions...)
	if err != nil {
		// xxx
	}

	_ = msgTimestamp // xxx

	// xxx update DB

	return nil
}

func (s *Service) OnPRReview(ctx context.Context, ev *github.PullRequestReviewEvent) error {
	channelName := fmt.Sprintf("pr-%s-%d", *ev.Repo.FullName, *ev.PullRequest.Number)
	channelID, err := s.GetChannelID(ctx, channelName)
	if err != nil {
		// xxx
	}
	postOptions := []slack.MsgOption{
		slack.MsgOptionText(*ev.Review.Body, false), // xxx convert GH Markdown to Slack mrkdwn (using https://github.com/eritikass/githubmarkdownconvertergo ?)
	}
	_, msgTimestamp, err := s.SlackClient.PostMessageContext(ctx, channelID, postOptions...)
	if err != nil {
		// xxx
	}

	_ = msgTimestamp // xxx

	// xxx update DB

	return nil
}

func (s *Service) OnPRReviewComment(ctx context.Context, ev *github.PullRequestReviewCommentEvent) error {
	channelName := fmt.Sprintf("pr-%s-%d", *ev.Repo.FullName, *ev.PullRequest.Number)
	channelID, err := s.GetChannelID(ctx, channelName)
	if err != nil {
		// xxx
	}

	postOptions := []slack.MsgOption{
		slack.MsgOptionText(*ev.Comment.Body, false), // xxx convert GH Markdown to Slack mrkdwn (using https://github.com/eritikass/githubmarkdownconvertergo ?)
	}
	if ev.Comment.InReplyTo != nil && *ev.Comment.InReplyTo != 0 {
		comment, err := s.Comments.ByCommentID(ctx, channelID, *ev.Comment.InReplyTo)
		if err != nil {
			// xxx
		}
		postOptions = append(postOptions, slack.MsgOptionTS(comment.ThreadTimestamp))
	}

	_, msgTimestamp, err := s.SlackClient.PostMessageContext(ctx, channelID, postOptions...)
	if err != nil {
		// xxx
	}

	_ = msgTimestamp // xxx

	// xxx update DB

	return nil
}

func (s *Service) OnPRReviewThread(ctx context.Context, ev *github.PullRequestReviewThreadEvent) error {
	// xxx
	return nil
}

func (s *Service) PRReviewRequested(ctx context.Context, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRReviewRequestLabeled(ctx context.Context, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRReviewRequestRemoved(ctx context.Context, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRReviewRequestSynchronize(ctx context.Context, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRReviewRequestUnlabeled(ctx context.Context, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRAssigned(ctx context.Context, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRClosed(ctx context.Context, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PREdited(ctx context.Context, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRReopened(ctx context.Context, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRUnassigned(ctx context.Context, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}
