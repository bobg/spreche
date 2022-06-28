package pg

import (
	"context"
	"database/sql"

	"github.com/bobg/sqlutil"
	"github.com/pkg/errors"

	"spreche"
)

type userStore struct {
	db *sql.DB
}

var _ spreche.UserStore = userStore{}

func (u userStore) BySlackID(ctx context.Context, tenantID int64, slackID string) (*spreche.User, error) {
	const q = `SELECT github_name FROM users WHERE tenant_id = $1 AND slack_id = $2`
	result := &spreche.User{
		SlackID: slackID,
	}
	err := sqlutil.QueryRowContext(ctx, u.db, q, tenantID, slackID).Scan(&result.GithubName)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (u userStore) ByGithubName(ctx context.Context, tenantID int64, githubName string) (*spreche.User, error) {
	const q = `SELECT slack_id FROM users WHERE tenant_id = $1 AND github_name = $2`
	result := &spreche.User{
		GithubName: githubName,
	}
	err := sqlutil.QueryRowContext(ctx, u.db, q, tenantID, githubName).Scan(&result.SlackID)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (u userStore) Add(ctx context.Context, tenantID int64, user *spreche.User) error {
	const q = `INSERT INTO users (tenant_id, slack_id, github_name) VALUES ($1, $2, $3)`
	_, err := u.db.ExecContext(ctx, q, tenantID, user.SlackID, user.GithubName)
	return err
}
