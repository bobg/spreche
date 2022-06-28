package pg

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"

	"spreche"
)

const schema = `
CREATE TABLE IF NOT EXISTS channels (
  tenant_id INTEGER NOT NULL,
  channel_id TEXT NOT NULL,
  owner TEXT NOT NULL,
  repo TEXT NOT NULL,
  pr INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS channel_id_index ON channels (tenant_id, channel_id);
CREATE UNIQUE INDEX IF NOT EXISTS owner_repo_pr_index ON channels (tenant_id, owner, repo, pr);

CREATE TABLE IF NOT EXISTS comments (
  tenant_id INTEGER NOT NULL,
  channel_id TEXT NOT NULL,
  thread_timestamp TEXT NOT NULL,
  comment_id INTEGER NOT NULL,
  PRIMARY KEY (tenant_id, channel_id, thread_timestamp)
);

CREATE INDEX IF NOT EXISTS channel_comment_index ON comments (tenant_id, channel_id, comment_id);

CREATE TABLE IF NOT EXISTS tenants (
  tenant_id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  gh_installation_id INTEGER NOT NULL,
  gh_priv_key BLOB NOT NULL,
  gh_api_url TEXT NOT NULL,
  gh_upload_url TEXT NOT NULL,
  slack_token TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tenant_repos (
  tenant_id INT NOT NULL REFERENCES tenants (tenant_id) ON DELETE CASCADE,
  repo_url TEXT NOT NULL PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS tenant_teams (
  tenant_id INT NOT NULL REFERENCES tenants (tenant_id) ON DELETE CASCADE,
  team_id TEXT NOT NULL PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS users (
  tenant_id INTEGER NOT NULL,
  slack_id TEXT NOT NULL,
  github_login TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS slack_id_index ON users (tenant_id, slack_id);
CREATE UNIQUE INDEX IF NOT EXISTS github_login_index ON users (tenant_id, github_login);
`

func Open(ctx context.Context, dsn string) (Stores, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return Stores{}, errors.Wrap(err, "opening db")
	}

	_, err = db.ExecContext(ctx, schema)
	if err != nil {
		db.Close()
		return Stores{}, errors.Wrap(err, "instantiating schema")
	}

	return Stores{
		Channels: channelStore{db: db},
		Comments: commentStore{db: db},
		Tenants:  tenantStore{db: db},
		Users:    userStore{db: db},
		db:       db,
	}, nil
}

type Stores struct {
	Channels spreche.ChannelStore
	Comments spreche.CommentStore
	Tenants  spreche.TenantStore
	Users    spreche.UserStore

	db *sql.DB
}

func (s Stores) Close() error {
	return s.db.Close()
}
