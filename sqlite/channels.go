package sqlite

import (
	"context"
	"database/sql"

	"github.com/bobg/sqlutil"
	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"

	"spreche"
)

type channelStore struct {
	db *sql.DB
}

var _ spreche.ChannelStore = channelStore{}

func (c channelStore) Add(ctx context.Context, tenantID int64, channelID string, repo *github.Repository, prnum int, prBodyTS string) error {
	const q = `INSERT INTO channels (tenant_id, channel_id, owner, repo, pr, prbody_timestamp) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := c.db.ExecContext(ctx, q, tenantID, channelID, *repo.Owner.Login, *repo.Name, prnum, prBodyTS)
	return err
}

func (c channelStore) ByChannelID(ctx context.Context, tenantID int64, channelID string) (*spreche.Channel, error) {
	const q = `SELECT owner, repo, pr, prbody_timestamp FROM channels WHERE tenant_id = $1 AND channel_id = $2`
	result := &spreche.Channel{
		ChannelID: channelID,
	}
	err := sqlutil.QueryRowContext(ctx, c.db, q, tenantID, channelID).Scan(&result.Owner, &result.Repo, &result.PR, &result.PRBodyTS)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (c channelStore) ByRepoPR(ctx context.Context, tenantID int64, repo *github.Repository, prnum int) (*spreche.Channel, error) {
	const q = `SELECT channel_id, prbody_timestamp FROM channels WHERE tenant_id = $1 AND owner = $2 AND repo = $3 AND pr = $4`
	result := &spreche.Channel{
		Owner: *repo.Owner.Login,
		Repo:  *repo.Name,
		PR:    prnum,
	}
	err := sqlutil.QueryRowContext(ctx, c.db, q, tenantID, *repo.Owner.Login, *repo.Name, prnum).Scan(&result.ChannelID, &result.PRBodyTS)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}
