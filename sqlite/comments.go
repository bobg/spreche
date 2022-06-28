package sqlite

import (
	"context"
	"database/sql"

	"github.com/bobg/sqlutil"
	"github.com/pkg/errors"

	"spreche"
)

type commentStore struct {
	db *sql.DB
}

var _ spreche.CommentStore = &commentStore{}

func (c commentStore) ByCommentID(ctx context.Context, tenantID int64, channelID string, commentID int64) (*spreche.Comment, error) {
	const q = `SELECT thread_timestamp FROM comments WHERE tenant_id = $1 AND channel_id = $2 AND comment_id = $3`
	result := &spreche.Comment{
		ChannelID: channelID,
		CommentID: commentID,
	}
	err := sqlutil.QueryRowContext(ctx, c.db, q, tenantID, channelID, commentID).Scan(&result.ThreadTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (c commentStore) ByThreadTimestamp(ctx context.Context, tenantID int64, channelID, timestamp string) (*spreche.Comment, error) {
	const q = `SELECT comment_id FROM comments WHERE tenant_id = $1 channel_id = $2 AND thread_timestamp = $3`
	result := &spreche.Comment{
		ChannelID:       channelID,
		ThreadTimestamp: timestamp,
	}
	err := sqlutil.QueryRowContext(ctx, c.db, q, tenantID, channelID, timestamp).Scan(&result.CommentID)
	if errors.Is(err, sql.ErrNoRows) {
		err = spreche.ErrNotFound
	}
	return result, err
}

func (c commentStore) Add(ctx context.Context, tenantID int64, channelID, timestamp string, commentID int64) error {
	const q = `INSERT INTO comments (tenant_id, channel_id, thread_timestamp, comment_id) VALUES ($1, $2, $3, $4)`
	_, err := c.db.ExecContext(ctx, q, tenantID, channelID, timestamp, commentID)
	return err
}
