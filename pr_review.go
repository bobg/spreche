package crocs

import (
	"context"
	"fmt"

	"github.com/google/go-github/v44/github"
	"github.com/slack-go/slack"
)

func (s *Service) OnPRReview(ctx context.Context, ev *github.PullRequestReviewEvent) error {
	channelName := fmt.Sprintf("pr-%s/%s-%d", org, repo, prnum)
	channelID, err := s.GetChannelID(ctx, channelName)
	if err != nil {
		// xxx
	}
	postOptions := []slack.MsgOption{
		slack.MsgOptionText(xxx, false), // xxx convert GH Markdown to Slack mrkdwn (using https://github.com/eritikass/githubmarkdownconvertergo ?)
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
	// xxx
	return nil
}

func (s *Service) OnPRReviewThread(ctx context.Context, ev *github.PullRequestReviewThreadEvent) error {
	// xxx
	return nil
}
