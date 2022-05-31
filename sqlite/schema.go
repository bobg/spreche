package sqlite

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"

	"crocs"
)

const schema = `
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

func Open(ctx context.Context, conn string) (crocs.CommentStore, crocs.UserStore, func() error, error) {
	db, err := sql.Open("sqlite3", conn)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "opening %s", conn)
	}
	_, err = db.ExecContext(ctx, schema)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "instantiating schema") // xxx should close db
	}
	closer := db.Close
	return &commentStore{db: db}, &userStore{db: db}, closer, nil
}

type commentStore struct {
	db *sql.DB
}

var _ crocs.CommentStore = &commentStore{}

func (c *commentStore) ByCommentID(ctx context.Context, channelID string, commentID int64) (*crocs.Comment, error) {
	const q = `SELECT thread_timestamp FROM comments WHERE channel_id = $1 AND comment_id = $2`
	result := &crocs.Comment{
		ChannelID: channelID,
		CommentID: commentID,
	}
	err := c.db.QueryRowContext(ctx, q, channelID, commentID).Scan(&result.ThreadTimestamp)
	return result, err
}

func (c *commentStore) ByThreadTimestamp(ctx context.Context, channelID, timestamp string) (*crocs.Comment, error) {
	const q = `SELECT comment_id FROM comments WHERE channel_id = $1 AND thread_timestamp = $2`
	result := &crocs.Comment{
		ChannelID:       channelID,
		ThreadTimestamp: timestamp,
	}
	err := c.db.QueryRowContext(ctx, q, channelID, timestamp).Scan(&result.CommentID)
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

var _ crocs.UserStore = &userStore{}

func (u *userStore) BySlackID(ctx context.Context, slackID string) (*crocs.User, error) {
	const q = `SELECT slack_name, github_name FROM users WHERE slack_id = $1`
	result := &crocs.User{
		SlackID: slackID,
	}
	err := u.db.QueryRowContext(ctx, q, slackID).Scan(&result.SlackName, &result.GithubName)
	return result, err
}

func (u *userStore) BySlackName(ctx context.Context, slackName string) (*crocs.User, error) {
	const q = `SELECT slack_id, github_name FROM users WHERE slack_name = $1`
	result := &crocs.User{
		SlackName: slackName,
	}
	err := u.db.QueryRowContext(ctx, q, slackName).Scan(&result.SlackID, &result.GithubName)
	return result, err
}

func (u *userStore) ByGithubName(ctx context.Context, githubName string) (*crocs.User, error) {
	const q = `SELECT slack_id, slack_name FROM users WHERE github_name = $1`
	result := &crocs.User{
		GithubName: githubName,
	}
	err := u.db.QueryRowContext(ctx, q, githubName).Scan(&result.SlackID, &result.GithubName)
	return result, err
}
