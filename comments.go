package spreche

import "context"

// CommentStore is a persistent store associating Slack comment threads with GitHub comment threads.
type CommentStore interface {
	ByCommentID(ctx context.Context, tenantID int64, channelID string, commentID int64) (*Comment, error)
	ByThreadTimestamp(ctx context.Context, tenantID int64, channelID, timestamp string) (*Comment, error)
	Add(ctx context.Context, tenantID int64, channelID, timestamp string, commentID int64) error
}

type Comment struct {
	ChannelID       string
	ThreadTimestamp string
	CommentID       int64
}
