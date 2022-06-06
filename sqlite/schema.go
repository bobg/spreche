package sqlite

import (
	"context"
	"database/sql"

	"github.com/google/go-github/v44/github"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"

	"spreche"
)

const schema = `
CREATE TABLE IF NOT EXISTS channels (
  channel_id TEXT NOT NULL,
  owner TEXT NOT NULL,
  repo TEXT NOT NULL,
  pr INT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS channel_id_index ON channels (channel_id);
CREATE UNIQUE INDEX IF NOT EXISTS owner_repo_pr_index ON channels (owner, repo, pr);

CREATE TABLE IF NOT EXISTS comments (
  channel_id TEXT NOT NULL,
  thread_timestamp TEXT NOT NULL,
  comment_id INT NOT NULL,
  PRIMARY KEY (channel_id, thread_timestamp)
);

CREATE INDEX IF NOT EXISTS channel_comment_index ON comments (channel_id, comment_id);

CREATE TABLE IF NOT EXISTS users (
  slack_id TEXT NOT NULL,
  slack_name TEXT NOT NULL,
  github_name TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS slack_id_index ON users (slack_id);
CREATE UNIQUE INDEX IF NOT EXISTS slack_name_index ON users (slack_name);
CREATE UNIQUE INDEX IF NOT EXISTS github_name_index ON users (github_name);
`

func Open(ctx context.Context, conn string) (spreche.ChannelStore, spreche.CommentStore, spreche.UserStore, func() error, error) {
	db, err := sql.Open("sqlite3", conn)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrapf(err, "opening %s", conn)
	}
	_, err = db.ExecContext(ctx, schema)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "instantiating schema") // xxx should close db
	}
	closer := db.Close
	return &channelStore{db: db}, &commentStore{db: db}, &userStore{db: db}, closer, nil
}

type channelStore struct {
	db *sql.DB
}

var _ spreche.ChannelStore = &channelStore{}

func (c *channelStore) Add(ctx context.Context, channelID string, repo *github.Repository, prnum int) error {
	const q = `INSERT INTO channels (channel_id, owner, repo, pr) VALUES ($1, $2, $3, $4)`
	_, err := c.db.ExecContext(ctx, q, channelID, *repo.Owner.Login, *repo.Name, prnum)
	return err
}

func (c *channelStore) ByChannelID(ctx context.Context, channelID string) (*spreche.Channel, error) {
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

func (c *channelStore) ByRepoPR(ctx context.Context, repo *github.Repository, prnum int) (*spreche.Channel, error) {
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

func (c *commentStore) ByCommentID(ctx context.Context, channelID string, commentID int64) (*spreche.Comment, error) {
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

func (c *commentStore) ByThreadTimestamp(ctx context.Context, channelID, timestamp string) (*spreche.Comment, error) {
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

func (c *commentStore) Update(ctx context.Context, channelID, timestamp string, commentID int64) error {
	const q = `INSERT INTO comments (channel_id, thread_timestamp, comment_id) VALUES ($1, $2, $3) ON CONFLICT DO UPDATE SET comment_id = $3 WHERE channel_id = $1 AND thread_timestamp = $2`
	_, err := c.db.ExecContext(ctx, q, channelID, timestamp, commentID)
	return err
}

type userStore struct {
	db *sql.DB
}

var _ spreche.UserStore = &userStore{}

func (u *userStore) BySlackID(ctx context.Context, slackID string) (*spreche.User, error) {
	const q = `SELECT slack_name, github_name FROM users WHERE slack_id = $1`
	result := &spreche.User{
		SlackID: slackID,
	}
	err := u.db.QueryRowContext(ctx, q, slackID).Scan(&result.SlackName, &result.GithubName)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (u *userStore) BySlackName(ctx context.Context, slackName string) (*spreche.User, error) {
	const q = `SELECT slack_id, github_name FROM users WHERE slack_name = $1`
	result := &spreche.User{
		SlackName: slackName,
	}
	err := u.db.QueryRowContext(ctx, q, slackName).Scan(&result.SlackID, &result.GithubName)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (u *userStore) ByGithubName(ctx context.Context, githubName string) (*spreche.User, error) {
	const q = `SELECT slack_id, slack_name FROM users WHERE github_name = $1`
	result := &spreche.User{
		GithubName: githubName,
	}
	err := u.db.QueryRowContext(ctx, q, githubName).Scan(&result.SlackID, &result.GithubName)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}
