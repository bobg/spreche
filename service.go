package spreche

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
)

type Service struct {
	AdminKey           string
	GHSecret           string
	SlackSigningSecret string

	Channels ChannelStore
	Comments CommentStore
	Tenants  TenantStore
	Users    UserStore
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

type TenantStore interface {
	WithTenant(ctx context.Context, repoURL, teamID string, f func(context.Context, *Tenant) error) error
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

type Tenant struct {
	TenantID              int64
	GHInstallationID      int64
	GHPrivKey             []byte
	GHAPIURL, GHUploadURL string
	SlackToken            string
}

const ghAppID = 207677 // https://github.com/settings/apps/spreche

func (t *Tenant) GHClient() (*github.Client, error) {
	itr, err := ghinstallation.New(http.DefaultTransport, ghAppID, t.GHInstallationID, t.GHPrivKey)
	if err != nil {
		return nil, errors.Wrap(err, "creating transport for GitHub client")
	}
	itr.BaseURL = t.GHAPIURL
	return github.NewEnterpriseClient(t.GHAPIURL, t.GHUploadURL, &http.Client{Transport: itr})
}

func (t *Tenant) SlackClient() *slack.Client {
	return slack.New(t.SlackToken)
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
