package crocs

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v44/github"
	"github.com/pkg/errors"
)

func (s *Service) OnGHWebhook(ctx context.Context, req *http.Request) error {
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
