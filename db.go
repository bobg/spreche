package main

import "context"

func (s *Service) LookupThreadTimestamp(ctx context.Context, channelID string, commentID int64) (string, error) {
	const q = `SELECT thread_timestamp FROM thread_timestamps WHERE channel_id = $1 AND comment_id = $2`

	var threadTimestamp string
	err := s.DB.QueryRowContext(ctx, q, channelID, commentID).Scan(&threadTimestamp)
	return threadTimestamp, err
}

func (s *Service) SlackToGHUser(ctx context.Context, slackUser string) (string, error) {
	const q = `SELECT github_user FROM user_mapping WHERE slack_user = $1`

	var ghUser string
	err := s.DB.QueryRowContext(ctx, q, slackUser).Scan(&ghUser)
	return ghUser, err
}

func (s *Service) LookupGHCommentIDFromSlackTimestamp(ctx context.Context, channelID, timestamp string) (int64, error) {
	const q = `SELECT comment_id FROM thread_timestamps WHERE channel_id = $1 AND thread_timestamp = $2`

	var commentID int64
	err := s.DB.QueryRowContext(ctx, q, channelID, timestamp).Scan(&commentID)
	return commentID, err
}

const schema = `
CREATE TABLE IF NOT EXISTS thread_timestamps (
  channel_id TEXT NOT NULL,
  comment_id INT NOT NULL,
  thread_timestamp TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS thread_timestamp_index ON thread_timestamps (channel_id, comment_id);
CREATE INDEX IF NOT EXISTS comment_id_index ON thread_timestamps (channel_id, thread_timestamp);

CREATE TABLE IF NOT EXISTS user_mapping (
  github_user TEXT NOT NULL,
  slack_user TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS slack_by_github_user ON user_mapping (github_user);
CREATE UNIQUE INDEX IF NOT EXISTS github_by_slack_user ON user_mapping (slack_user);
`
