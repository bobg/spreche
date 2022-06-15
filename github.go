package spreche

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
)

func (s *Service) OnGHWebhook(w http.ResponseWriter, req *http.Request) error {
	ctx := req.Context()

	payload, err := github.ValidatePayload(req, []byte(s.GHSecret))
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

	chname := ChannelName(repo, *pr.Number)
	ch, err := s.SlackClient.CreateConversationContext(ctx, chname, false)
	if err != nil {
		return errors.Wrapf(err, "creating channel %s", chname)
	}

	err = s.Channels.Add(ctx, ch.ID, repo, *pr.Number)
	if err != nil {
		return errors.Wrapf(err, "storing info for channel %s", chname)
	}

	topic := fmt.Sprintf("Discussion of %s: %s by %s", *pr.HTMLURL, *pr.Title, *pr.User.HTMLURL)
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
	// xxx also pr.RequestedTeams ?

	slackUsers, err := s.GHToSlackUsers(ctx, ghUsers)
	if err != nil {
		return errors.Wrap(err, "mapping GitHub to Slack users")
	}

	if len(slackUsers) > 0 {
		_, err = s.SlackClient.InviteUsersToConversationContext(ctx, ch.ID, slackUsers...)
		if err != nil {
			return errors.Wrap(err, "inviting users to new channel")
		}
	}

	body := "[no content]"
	if pr.Body != nil {
		body = *pr.Body
	}
	postOptions := []slack.MsgOption{
		slack.MsgOptionDisableLinkUnfurl(),
		slack.MsgOptionText(body, false), // xxx convert GH Markdown to Slack mrkdwn (using https://github.com/eritikass/githubmarkdownconvertergo ?)
	}
	_, _, err = s.SlackClient.PostMessageContext(ctx, ch.ID, postOptions...)
	return errors.Wrap(err, "posting new-channel message")
}

// note: reviews do not get placed in the comment store,
// unlike review _comments_
func (s *Service) OnPRReview(ctx context.Context, ev *github.PullRequestReviewEvent) error {
	if ev.Review.Body == nil || *ev.Review.Body == "" {
		return nil
	}
	channel, err := s.Channels.ByRepoPR(ctx, ev.Repo, *ev.PullRequest.Number)
	if err != nil {
		return errors.Wrapf(err, "getting channel for PR %d in %s/%s", *ev.PullRequest.Number, *ev.Repo.Owner.Login, *ev.Repo.Name)
	}

	// xxx ensure channel exists

	blocks := []slack.Block{
		slack.NewContextBlock(
			"",
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("<Review|%s> by <%s|%s>", *ev.Review.HTMLURL, *ev.Review.User.Login, *ev.Review.User.HTMLURL),
				false,
				false,
			),
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"plain_text", // xxx convert GH to Slack markdown
				*ev.Review.Body,
				false,
				false,
			),
			nil,
			nil,
		),
	}

	options := []slack.MsgOption{slack.MsgOptionBlocks(blocks...)}
	u, err := s.Users.ByGithubName(ctx, *ev.Review.User.Login)
	switch {
	case errors.Is(err, ErrNotFound):
		// do nothing
	case err != nil:
		return errors.Wrapf(err, "looking up user %s", *ev.Review.User.Login)
	default:
		options = append(options, slack.MsgOptionUser(u.SlackID), slack.MsgOptionAsUser(true)) // xxx also slack.MsgOptionAsUser(true)?
	}
	_, _, err = s.SlackClient.PostMessageContext(ctx, channel.ChannelID, options...)
	return errors.Wrap(err, "posting message")
}

func (s *Service) OnPRReviewComment(ctx context.Context, ev *github.PullRequestReviewCommentEvent) error {
	if ev.Comment.Body == nil || *ev.Comment.Body == "" {
		return nil
	}
	if ev.Comment.User != nil && ev.Comment.User.Type != nil && *ev.Comment.User.Type == "Bot" {
		return nil
	}
	channel, err := s.Channels.ByRepoPR(ctx, ev.Repo, *ev.PullRequest.Number)
	if err != nil {
		return errors.Wrapf(err, "getting channel for PR %d in %s/%s", *ev.PullRequest.Number, *ev.Repo.Owner.Login, *ev.Repo.Name)
	}
	var (
		options = []slack.MsgOption{slack.MsgOptionDisableLinkUnfurl()}
		isReply bool
	)
	if ev.Comment.InReplyTo != nil && *ev.Comment.InReplyTo != 0 {
		isReply = true
		comment, err := s.Comments.ByCommentID(ctx, channel.ChannelID, *ev.Comment.InReplyTo)
		if err != nil {
			return errors.Wrapf(err, "finding comment in channel %s by commentID %d", channel.ChannelID, *ev.Comment.InReplyTo)
		}
		options = append(options, slack.MsgOptionTS(comment.ThreadTimestamp))
	}

	// xxx ensure channel exists

	contextBlockElements := []slack.MixedElement{
		slack.NewTextBlockObject(
			"mrkdwn",
			fmt.Sprintf("<%s|Review comment> by <%s|%s>", *ev.Comment.HTMLURL, *ev.Comment.User.HTMLURL, *ev.Comment.User.Login),
			false,
			false,
		),
	}
	if !isReply && ev.Comment.DiffHunk != nil && *ev.Comment.DiffHunk != "" {
		contextBlockElements = append(
			contextBlockElements,
			slack.NewTextBlockObject(
				"mrkdwn",
				"```\n"+*ev.Comment.DiffHunk+"\n```", // xxx escaping? etc
				false,
				false,
			),
		)
	}
	blocks := []slack.Block{
		slack.NewContextBlock("", contextBlockElements...),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"plain_text", // xxx convert GH to Slack markdown
				*ev.Comment.Body,
				false,
				false,
			),
			nil,
			nil,
		),
	}
	options = append(options, slack.MsgOptionBlocks(blocks...))
	u, err := s.Users.ByGithubName(ctx, *ev.Comment.User.Login)
	switch {
	case errors.Is(err, ErrNotFound):
		fmt.Printf("xxx did not find entry for GitHub user %s\n", *ev.Comment.User.Login)
		// do nothing
	case err != nil:
		return errors.Wrapf(err, "looking up user %s", *ev.Comment.User.Login)
	default:
		fmt.Printf("xxx %s -> %s\n", *ev.Comment.User.Login, u.SlackID)
		options = append(options, slack.MsgOptionUser(u.SlackID), slack.MsgOptionAsUser(true)) // xxx also slack.MsgOptionAsUser(true)?
	}

	_, timestamp, err := s.SlackClient.PostMessageContext(ctx, channel.ChannelID, options...)
	if err != nil {
		return errors.Wrap(err, "posting message")
	}
	if isReply {
		return nil
	}
	return s.Comments.Add(ctx, channel.ChannelID, timestamp, *ev.Comment.ID)
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
