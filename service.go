package spreche

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
)

type Service struct {
	AdminKey           string
	GHClient           *github.Client
	GHSecret           string
	SlackClient        *slack.Client
	SlackSigningSecret string
	SlackTeam          *slack.TeamInfo

	Channels ChannelStore
	Comments CommentStore
	Users    UserStore
}

func NewService(ctx context.Context, gh *github.Client, sl *slack.Client) (*Service, error) {
	result := &Service{
		GHClient:    gh,
		SlackClient: sl,
	}
	team, err := sl.GetTeamInfoContext(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting team info")
	}
	result.SlackTeam = team
	return result, nil
}

var ErrNotFound = errors.New("not found")

type ChannelStore interface {
	Add(ctx context.Context, channelID string, repo *github.Repository, pr int) error
	ByChannelID(context.Context, string) (*Channel, error)
	ByRepoPR(context.Context, *github.Repository, int) (*Channel, error)
}

type CommentStore interface {
	ByCommentID(ctx context.Context, channelID string, commentID int64) (*Comment, error)
	ByThreadTimestamp(ctx context.Context, channelID, timestamp string) (*Comment, error)
	Add(ctx context.Context, channelID, timestamp string, commentID int64) error
}

type UserStore interface {
	BySlackID(context.Context, string) (*User, error)
	ByGithubName(context.Context, string) (*User, error)
	Add(context.Context, *User) error
}

type Channel struct {
	ChannelID string
	Owner     string
	Repo      string
	PR        int
}

type Comment struct {
	ChannelID       string
	ThreadTimestamp string
	CommentID       int64
}

type User struct {
	SlackID    string
	GithubName string
}

func ChannelName(repo *github.Repository, prnum int) string {
	// xxx Sanitize strings - only a-z0-9 allowed, plus hyphen and underscore. N.B. no capitals!

	var (
		owner = strings.ToLower(*repo.Owner.Login)
		name  = strings.ToLower(*repo.Name)
	)
	return fmt.Sprintf("pr-%s-%s-%d", owner, name, prnum)
}
