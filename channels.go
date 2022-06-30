package spreche

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/github"
)

// ChannelStore is the type of a persistent store for Channels.
type ChannelStore interface {
	Add(ctx context.Context, tenantID int64, channelID string, repo *github.Repository, pr int) error
	ByChannelID(context.Context, int64, string) (*Channel, error)
	ByRepoPR(context.Context, int64, *github.Repository, int) (*Channel, error)
}

// Channel is information about a Slack channel and the GitHub PR it is associated with.
type Channel struct {
	ChannelID string
	Owner     string
	Repo      string
	PR        int
}

// ChannelName computes a Slack channel name for the given GH repo and PR number.
func ChannelName(repo *github.Repository, prnum int) string {
	// xxx Sanitize strings - only a-z0-9 allowed, plus hyphen and underscore. N.B. no capitals!

	var (
		owner = strings.ToLower(*repo.Owner.Login)
		name  = strings.ToLower(*repo.Name)
	)
	return fmt.Sprintf("pr-%s-%s-%d", owner, name, prnum)
}
