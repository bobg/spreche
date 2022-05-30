package crocs

import (
	"context"
	"database/sql"

	"github.com/google/go-github/v44/github"
	"github.com/slack-go/slack"
)

type Service struct {
	GHSecret    []byte
	SlackSecret string
	GHClient    *github.Client
	SlackClient *slack.Client

	CommentStore CommentStore
	UserStore    UserStore

	DB *sql.DB
}

type CommentStore interface {
	ByCommentID(ctx context.Context, channelID string, commentID int64) (*Comment, error)
	ByThreadTimestamp(ctx context.Context, channelID, timestamp string) (*Comment, error)
	Update(ctx context.Context, channelID, timestamp string, commentID int64) error
}

type UserStore interface {
	BySlackID(context.Context, string) (*User, error)
	BySlackName(context.Context, string) (*User, error)
	ByGHName(context.Context, string) (*User, error)
}

type Comment struct {
	ChannelID       string
	ThreadTimestamp string
	CommentID       int64
}

type User struct {
	SlackID   string
	SlackName string
	GHName    string
}
