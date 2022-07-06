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

	case *github.IssueCommentEvent:
		return s.OnIssueComment(ctx, ev)

	case *github.PullRequestReviewCommentEvent:
		return s.OnPRReviewComment(ctx, ev)

	case *github.PullRequestReviewThreadEvent:
		return s.OnPRReviewThread(ctx, ev)
	}

	return fmt.Errorf("unknown webhook payload type %T", ev)
}

func (s *Service) OnPR(ctx context.Context, ev *github.PullRequestEvent) error {
	return s.Tenants.WithTenant(ctx, 0, *ev.Repo.HTMLURL, "", func(ctx context.Context, tenant *Tenant) error {
		debugf("In OnPR, tenantID is %d", tenant.TenantID)

		switch ev.GetAction() {
		case "opened":
			return s.PROpened(ctx, tenant, ev)

		case "edited":
			return s.PREdited(ctx, tenant, ev)

		case "closed":
			return s.PRClosed(ctx, tenant, ev)

		case "reopened":
			return s.PRReopened(ctx, tenant, ev)

		case "assigned":
			return s.PRAssigned(ctx, tenant, ev)

		case "unassigned":
			return s.PRUnassigned(ctx, tenant, ev)

		case "review_requested":
			return s.PRReviewRequested(ctx, tenant, ev)

		case "review_request_removed":
			return s.PRReviewRequestRemoved(ctx, tenant, ev)

		case "labeled":
			return s.PRReviewRequestLabeled(ctx, tenant, ev)

		case "unlabeled":
			return s.PRReviewRequestUnlabeled(ctx, tenant, ev)

		case "synchronize":
			return s.PRReviewRequestSynchronize(ctx, tenant, ev)
		}

		return fmt.Errorf("unknown PR event action %s", ev.GetAction())
	})
}

func (s *Service) PROpened(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent) error {
	var (
		sc   = tenant.SlackClient()
		repo = ev.Repo
		pr   = ev.PullRequest
	)

	chname := ChannelName(repo, *pr.Number)
	ch, err := sc.CreateConversationContext(ctx, chname, false)
	if err != nil {
		return errors.Wrapf(err, "creating channel %s", chname)
	}

	debugf("Created channel %s, ID %s", chname, ch.ID)

	err = s.Channels.Add(ctx, tenant.TenantID, ch.ID, repo, *pr.Number)
	if err != nil {
		return errors.Wrapf(err, "storing info for channel %s", chname)
	}

	topic := fmt.Sprintf("Discussion of %s: %s by %s", *pr.HTMLURL, *pr.Title, *pr.User.HTMLURL)
	_, err = sc.SetTopicOfConversationContext(ctx, ch.ID, topic)
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

	slackUsers, err := s.GHToSlackUsers(ctx, tenant.TenantID, ghUsers)
	if err != nil {
		return errors.Wrap(err, "mapping GitHub to Slack users")
	}

	if len(slackUsers) > 0 {
		_, err = sc.InviteUsersToConversationContext(ctx, ch.ID, slackUsers...)
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
	return s.postToSlack(ctx, tenant, ch.ID, 0, postOptions...)
}

func (s *Service) OnPRReview(ctx context.Context, ev *github.PullRequestReviewEvent) error {
	return s.someKindOfComment(ctx, ev, nil, nil)
}

func (s *Service) OnIssueComment(ctx context.Context, ev *github.IssueCommentEvent) error {
	if ev.Issue.PullRequestLinks == nil {
		return nil
	}
	return s.someKindOfComment(ctx, nil, ev, nil)
}

func (s *Service) someKindOfComment(ctx context.Context, review *github.PullRequestReviewEvent, issue *github.IssueCommentEvent, reviewComment *github.PullRequestReviewCommentEvent) error {
	var (
		repo           *github.Repository
		prnum          int
		user           *github.User
		action         string
		commentID      int64
		body, diffhunk *string
		htmlURL, typ   string
		isReply        bool
	)
	switch {
	case review != nil:
		repo = review.Repo
		prnum = *review.PullRequest.Number
		user = review.Review.User
		action = *review.Action
		commentID = *review.Review.ID
		body = review.Review.Body
		htmlURL = *review.Review.HTMLURL
		typ = "Review"

	case issue != nil:
		repo = issue.Repo
		prnum = *issue.Issue.Number
		user = issue.Comment.User
		action = *issue.Action
		commentID = *issue.Comment.ID
		body = issue.Comment.Body
		htmlURL = *issue.Comment.HTMLURL
		typ = "Comment"

	case reviewComment != nil:
		repo = reviewComment.Repo
		prnum = *reviewComment.PullRequest.Number
		user = reviewComment.Comment.User
		action = *reviewComment.Action
		commentID = *reviewComment.Comment.ID
		body = reviewComment.Comment.Body
		htmlURL = *reviewComment.Comment.HTMLURL
		typ = "Review comment"
		isReply = reviewComment.Comment.InReplyTo != nil && *reviewComment.Comment.InReplyTo != 0
		diffhunk = reviewComment.Comment.DiffHunk
	}
	if body == nil || *body == "" {
		return nil
	}
	if user != nil && user.Type != nil && *user.Type == "Bot" {
		return nil
	}
	return s.Tenants.WithTenant(ctx, 0, *repo.HTMLURL, "", func(ctx context.Context, tenant *Tenant) error {
		debugf("In someKindOfComment, tenant ID %d", tenant.TenantID)

		channel, err := s.Channels.ByRepoPR(ctx, tenant.TenantID, repo, prnum)
		if err != nil {
			return errors.Wrapf(err, "getting channel for PR %d in %s", prnum, *repo.HTMLURL)
		}

		// xxx ensure channel exists

		contextBlockElements := []slack.MixedElement{slack.NewTextBlockObject(
			"mrkdwn",
			fmt.Sprintf("<%s|%s> by <%s|%s>", htmlURL, typ, *user.HTMLURL, *user.Login),
			false,
			false,
		)}
		if !isReply && diffhunk != nil && *diffhunk != "" {
			contextBlockElements = append(contextBlockElements, slack.NewTextBlockObject(
				"mrkdwn",
				"```\n"+*diffhunk+"\n```", // xxx escaping? etc
				false,
				false,
			))
		}

		blocks := []slack.Block{
			slack.NewContextBlock("", contextBlockElements...),
			slack.NewSectionBlock(
				slack.NewTextBlockObject(
					"plain_text", // xxx convert GH to Slack markdown
					*body,
					false,
					false,
				),
				nil,
				nil,
			),
		}

		options := []slack.MsgOption{slack.MsgOptionBlocks(blocks...), slack.MsgOptionDisableLinkUnfurl()}
		u, err := s.Users.ByGHLogin(ctx, tenant.TenantID, *user.Login)
		switch {
		case errors.Is(err, ErrNotFound):
			// do nothing
		case err != nil:
			return errors.Wrapf(err, "looking up user %s", *user.Login)
		default:
			options = append(options, slack.MsgOptionUser(u.SlackID), slack.MsgOptionAsUser(true)) // xxx ?
		}

		if isReply {
			comment, err := s.Comments.ByCommentID(ctx, tenant.TenantID, channel.ChannelID, *reviewComment.Comment.InReplyTo)
			if err != nil {
				return errors.Wrap(err, "finding in-reply-to comment")
			}
			options = append(options, slack.MsgOptionTS(comment.ThreadTimestamp))
		}

		if action == "created" {
			return s.postToSlack(ctx, tenant, channel.ChannelID, commentID, options...)
		}

		comment, err := s.Comments.ByCommentID(ctx, tenant.TenantID, channel.ChannelID, commentID)
		if err != nil {
			return errors.Wrap(err, "getting comment record")
		}

		sc := tenant.SlackClient()

		switch action {
		case "edited":
			_, _, _, err = sc.UpdateMessageContext(ctx, channel.ChannelID, comment.ThreadTimestamp, options...)
			return errors.Wrap(err, "updating Slack comment")

		case "deleted":
			_, _, err = sc.DeleteMessageContext(ctx, channel.ChannelID, comment.ThreadTimestamp)
			return errors.Wrap(err, "deleting Slack comment")

		default:
			return fmt.Errorf("unknown action %s", action)
		}
	})
}

func (s *Service) OnPRReviewComment(ctx context.Context, ev *github.PullRequestReviewCommentEvent) error {
	return s.someKindOfComment(ctx, nil, nil, ev)
}

func (s *Service) OnPRReviewThread(ctx context.Context, ev *github.PullRequestReviewThreadEvent) error {
	return s.Tenants.WithTenant(ctx, 0, *ev.Repo.HTMLURL, "", func(ctx context.Context, tenant *Tenant) error {
		debugf("In OnPRReviewThread, tenant ID %d", tenant.TenantID)

		// xxx ensure channel exists

		channel, err := s.Channels.ByRepoPR(ctx, tenant.TenantID, ev.Repo, *ev.PullRequest.Number)
		if err != nil {
			return errors.Wrapf(err, "getting channel for PR %d in %s", *ev.PullRequest.Number, *ev.Repo.HTMLURL)
		}

		options := []slack.MsgOption{
			// xxx slack.MsgOptionsTs(...)?
			// xxx slack.MsgOptionUser(...)?
			// xxx slack.MsgOptionAsUser(...)?
			slack.MsgOptionBlocks(slack.NewContextBlock("", slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("_This thread was marked %s by %s_", *ev.Action, *ev.Sender.Login),
				false,
				false,
			))),
		}
		return s.postToSlack(ctx, tenant, channel.ChannelID, *ev.Thread.ID, options...)
	})
}

func (s *Service) PRReviewRequested(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent) error {
	return s.reviewRequest(ctx, tenant, ev, true)
}

func (s *Service) PRReviewRequestRemoved(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent) error {
	return s.reviewRequest(ctx, tenant, ev, false)
}

func (s *Service) reviewRequest(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent, requested bool) error {
	var (
		requestedFromTeam bool
		requestedFrom     string
	)
	if ev.RequestedReviewer != nil {
		requestedFrom = *ev.RequestedReviewer.Login
	} else if ev.RequestedTeam != nil {
		requestedFromTeam = true
		requestedFrom = *ev.RequestedTeam.Name
	}
	if requestedFrom == "" {
		return nil
	}
	if requestedFromTeam {
		requestedFrom = "team " + requestedFrom
	}

	// xxx ensure channel exists

	channel, err := s.Channels.ByRepoPR(ctx, tenant.TenantID, ev.Repo, *ev.PullRequest.Number)
	if err != nil {
		return errors.Wrapf(err, "getting channel for PR %d in %s", *ev.PullRequest.Number, *ev.Repo.HTMLURL)
	}

	var msg string
	if requested {
		msg = fmt.Sprintf("_Review requested from %s by %s_", requestedFrom, *ev.Sender.Login)
	} else {
		msg = fmt.Sprintf("_Review request from %s removed by %s_", requestedFrom, *ev.Sender.Login)
	}

	options := []slack.MsgOption{
		// xxx slack.MsgOptionsTs(...)?
		// xxx slack.MsgOptionUser(...)?
		// xxx slack.MsgOptionAsUser(...)?
		slack.MsgOptionBlocks(slack.NewContextBlock("", slack.NewTextBlockObject("mrkdwn", msg, false, false))),
	}
	return s.postToSlack(ctx, tenant, channel.ChannelID, 0, options...)
}

func (s *Service) PRReviewRequestSynchronize(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRReviewRequestLabeled(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRReviewRequestUnlabeled(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRAssigned(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRUnassigned(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRClosed(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent) error {
	channel, err := s.Channels.ByRepoPR(ctx, tenant.TenantID, ev.Repo, *ev.PullRequest.Number)
	if err != nil {
		return errors.Wrapf(err, "getting channel for PR %d in %s", *ev.PullRequest.Number, *ev.Repo.HTMLURL)
	}
	options := []slack.MsgOption{
		// xxx slack.MsgOptionsTs(...)?
		// xxx slack.MsgOptionUser(...)?
		// xxx slack.MsgOptionAsUser(...)?
		slack.MsgOptionBlocks(slack.NewContextBlock("", slack.NewTextBlockObject(
			"mrkdwn",
			fmt.Sprintf("_This PR was closed by %s_", *ev.Sender.Login),
			false,
			false,
		))),
	}
	return s.postToSlack(ctx, tenant, channel.ChannelID, 0, options...)
}

func (s *Service) PREdited(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent) error {
	// xxx
	return nil
}

func (s *Service) PRReopened(ctx context.Context, tenant *Tenant, ev *github.PullRequestEvent) error {
	channel, err := s.Channels.ByRepoPR(ctx, tenant.TenantID, ev.Repo, *ev.PullRequest.Number)
	if err != nil {
		return errors.Wrapf(err, "getting channel for PR %d in %s", *ev.PullRequest.Number, *ev.Repo.HTMLURL)
	}
	options := []slack.MsgOption{
		// xxx slack.MsgOptionsTs(...)?
		// xxx slack.MsgOptionUser(...)?
		// xxx slack.MsgOptionAsUser(...)?
		slack.MsgOptionBlocks(slack.NewContextBlock("", slack.NewTextBlockObject(
			"mrkdwn",
			fmt.Sprintf("_This PR was reopened by %s_", *ev.Sender.Login),
			false,
			false,
		))),
	}
	return s.postToSlack(ctx, tenant, channel.ChannelID, 0, options...)
}

func (s *Service) postToSlack(ctx context.Context, tenant *Tenant, channelID string, commentID int64, options ...slack.MsgOption) error {
	sc := tenant.SlackClient()
	_, timestamp, err := sc.PostMessageContext(ctx, channelID, options...)
	if err != nil {
		return errors.Wrap(err, "posting message to Slack")
	}
	if commentID == 0 {
		return nil
	}
	return s.Comments.Add(ctx, tenant.TenantID, channelID, timestamp, commentID)
}
