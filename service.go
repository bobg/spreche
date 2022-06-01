package crocs

import (
	"context"
	"fmt"

	"github.com/google/go-github/v44/github"
	"github.com/slack-go/slack"
)

type Service struct {
	GHSecret    []byte
	SlackSecret string
	GHClient    *github.Client
	SlackClient *slack.Client
	AdminKey    string

	Comments CommentStore
	Users    UserStore
}

type CommentStore interface {
	ByCommentID(ctx context.Context, channelID string, commentID int64) (*Comment, error)
	ByThreadTimestamp(ctx context.Context, channelID, timestamp string) (*Comment, error)
	Update(ctx context.Context, channelID, timestamp string, commentID int64) error
}

type UserStore interface {
	BySlackID(context.Context, string) (*User, error)
	BySlackName(context.Context, string) (*User, error)
	ByGithubName(context.Context, string) (*User, error)
}

type Comment struct {
	ChannelID       string
	ThreadTimestamp string
	CommentID       int64
}

type User struct {
	SlackID    string
	SlackName  string
	GithubName string
}

func ChannelName(repo *github.Repository, prnum int) string {
	// xxx Sanitize strings - only a-z0-9 allowed, plus hyphen and underscore. N.B. no capitals!
	return fmt.Sprintf("pr-%s-%s-%d", *repo.Owner.Login, *repo.Name, prnum)
}
