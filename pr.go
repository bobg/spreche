package crocs

import (
	"context"
	"fmt"

	"github.com/google/go-github/v44/github"
)

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
