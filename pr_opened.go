package crocs

import (
	"context"
	"fmt"

	"github.com/bobg/go-generics/iter"
	"github.com/google/go-github/v44/github"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
)

func (s *Service) PROpened(ctx context.Context, ev *github.PullRequestEvent) error {
	var (
		repo = ev.Repo
		pr   = ev.PullRequest
	)

	chname := fmt.Sprintf("pr-%s/%s-%d", *repo.Organization.Name, *repo.Name, *ev.Number)
	ch, err := s.SlackClient.CreateConversationContext(ctx, chname, false)
	if err != nil {
		return errors.Wrapf(err, "creating channel %s")
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
