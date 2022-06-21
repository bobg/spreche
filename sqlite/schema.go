package sqlite

import (
	"context"
	"database/sql"

	"github.com/google/go-github/v45/github"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"

	"spreche"
)

const schema = `
CREATE TABLE IF NOT EXISTS channels (
  tenant_id INT NOT NULL,
  channel_id TEXT NOT NULL,
  owner TEXT NOT NULL,
  repo TEXT NOT NULL,
  pr INT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS channel_id_index ON channels (tenant_id, channel_id);
CREATE UNIQUE INDEX IF NOT EXISTS owner_repo_pr_index ON channels (tenant_id, owner, repo, pr);

CREATE TABLE IF NOT EXISTS comments (
  tenant_id INT NOT NULL,
  channel_id TEXT NOT NULL,
  thread_timestamp TEXT NOT NULL,
  comment_id INT NOT NULL,
  PRIMARY KEY (tenant_id, channel_id, thread_timestamp)
);

CREATE INDEX IF NOT EXISTS channel_comment_index ON comments (tenant_id, channel_id, comment_id);

CREATE TABLE IF NOT EXISTS tenants (
  tenant_id INT NOT NULL PRIMARY KEY AUTOINCREMENT,
  gh_installation_id INT NOT NULL,
  gh_priv_key BLOB NOT NULL,
  gh_api_url TEXT NOT NULL,
  gh_upload_url TEXT NOT NULL,
  slack_token TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tenant_repos (
  repo_url TEXT NOT NULL PRIMARY KEY,
  tenant_id INT NOT NULL REFERENCES tenants (tenant_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS tenant_teams (
  team_id TEXT NOT NULL PRIMARY KEY,
  tenant_id INT NOT NULL REFERENCES tenants (tenant_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS users (
  tenant_id INT NOT NULL,
  slack_id TEXT NOT NULL,
  github_name TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS slack_id_index ON users (tenant_id, slack_id);
CREATE UNIQUE INDEX IF NOT EXISTS github_name_index ON users (tenant_id, github_name);
`

type Stores struct {
	Channels spreche.ChannelStore
	Comments spreche.CommentStore
	Tenants  spreche.TenantStore
	Users    spreche.UserStore

	db *sql.DB
}

func Open(ctx context.Context, conn string) (Stores, error) {
	db, err := sql.Open("sqlite3", conn)
	if err != nil {
		return Stores{}, errors.Wrapf(err, "opening %s", conn)
	}
	_, err = db.ExecContext(ctx, schema)
	if err != nil {
		db.Close()
		return Stores{}, errors.Wrap(err, "instantiating schema")
	}

	return Stores{
		Channels: channelStore{db: db},
		Comments: commentStore{db: db},
		Users:    userStore{db: db},
		db:       db,
	}, nil
}

func (s Stores) Close() error {
	return s.db.Close()
}

type channelStore struct {
	db *sql.DB
}

var _ spreche.ChannelStore = &channelStore{}

func (c channelStore) Add(ctx context.Context, channelID string, repo *github.Repository, prnum int) error {
	const q = `INSERT INTO channels (channel_id, owner, repo, pr) VALUES ($1, $2, $3, $4)`
	_, err := c.db.ExecContext(ctx, q, channelID, *repo.Owner.Login, *repo.Name, prnum)
	return err
}

func (c channelStore) ByChannelID(ctx context.Context, channelID string) (*spreche.Channel, error) {
	const q = `SELECT owner, repo, pr FROM channels WHERE channel_id = $1`
	result := &spreche.Channel{
		ChannelID: channelID,
	}
	err := c.db.QueryRowContext(ctx, q, channelID).Scan(&result.Owner, &result.Repo, &result.PR)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (c channelStore) ByRepoPR(ctx context.Context, repo *github.Repository, prnum int) (*spreche.Channel, error) {
	const q = `SELECT channel_id FROM channels WHERE owner = $1 AND repo = $2 AND pr = $3`
	result := &spreche.Channel{
		Owner: *repo.Owner.Login,
		Repo:  *repo.Name,
		PR:    prnum,
	}
	err := c.db.QueryRowContext(ctx, q, *repo.Owner.Login, *repo.Name, prnum).Scan(&result.ChannelID)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

type commentStore struct {
	db *sql.DB
}

var _ spreche.CommentStore = &commentStore{}

func (c commentStore) ByCommentID(ctx context.Context, channelID string, commentID int64) (*spreche.Comment, error) {
	const q = `SELECT thread_timestamp FROM comments WHERE channel_id = $1 AND comment_id = $2`
	result := &spreche.Comment{
		ChannelID: channelID,
		CommentID: commentID,
	}
	err := c.db.QueryRowContext(ctx, q, channelID, commentID).Scan(&result.ThreadTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (c commentStore) ByThreadTimestamp(ctx context.Context, channelID, timestamp string) (*spreche.Comment, error) {
	const q = `SELECT comment_id FROM comments WHERE channel_id = $1 AND thread_timestamp = $2`
	result := &spreche.Comment{
		ChannelID:       channelID,
		ThreadTimestamp: timestamp,
	}
	err := c.db.QueryRowContext(ctx, q, channelID, timestamp).Scan(&result.CommentID)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (c commentStore) Add(ctx context.Context, channelID, timestamp string, commentID int64) error {
	const q = `INSERT INTO comments (channel_id, thread_timestamp, comment_id) VALUES ($1, $2, $3)`
	_, err := c.db.ExecContext(ctx, q, channelID, timestamp, commentID)
	return err
}

type tenantStore struct {
	db *sql.DB
}

var _ spreche.TenantStore = &tenantStore{}

func (t *tenantStore) ByRepoURL(ctx context.Context, repoURL string) (*spreche.Tenant, error) {
	const q = `
		SELECT r.tenant_id, t.gh_installation_id, t.gh_priv_key, t.gh_api_url, t.gh_upload_url, t.slack_token
			FROM tenant_repos r, tenants t
			WHERE r.tenant_id = t.tenant_id AND t.repo_url = $1
	`
	var result speche.Tenant
	err := t.db.QueryRowContext(ctx, q, repoURL).Scan(
		&result.TenantID,
		&result.GHInstallationID,
		&result.GHPrivKey,
		&result.GHAPIURL,
		&result.GHUploadURL,
		&result.SlackToken,
	)
	return &result, err
}

func (t *tenantStore) ByTeamID(ctx context.Context, teamID string) (*spreche.Tenant, error) {
	const q = `
		SELECT tt.tenant_id, t.gh_installation_id, t.gh_priv_key, t.gh_api_url, t.gh_upload_url, t.slack_token
			FROM tenant_teams tt, tenants t
			WHERE tt.tenant_id = t.tenant_id AND t.team_id = $1
	`
	var result speche.Tenant
	err := t.db.QueryRowContext(ctx, q, repoURL).Scan(
		&result.TenantID,
		&result.GHInstallationID,
		&result.GHPrivKey,
		&result.GHAPIURL,
		&result.GHUploadURL,
		&result.SlackToken,
	)
	return &result, err
}

type userStore struct {
	db *sql.DB
}

var _ spreche.UserStore = &userStore{}

func (u userStore) BySlackID(ctx context.Context, slackID string) (*spreche.User, error) {
	const q = `SELECT github_name FROM users WHERE slack_id = $1`
	result := &spreche.User{
		SlackID: slackID,
	}
	err := u.db.QueryRowContext(ctx, q, slackID).Scan(&result.GithubName)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (u userStore) ByGithubName(ctx context.Context, githubName string) (*spreche.User, error) {
	const q = `SELECT slack_id FROM users WHERE github_name = $1`
	result := &spreche.User{
		GithubName: githubName,
	}
	err := u.db.QueryRowContext(ctx, q, githubName).Scan(&result.SlackID)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (u userStore) Add(ctx context.Context, user *spreche.User) error {
	const q = `INSERT INTO users (slack_id, github_name) VALUES ($1, $2)`
	_, err := u.db.ExecContext(ctx, q, user.SlackID, user.GithubName)
	return err
}
