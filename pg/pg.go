package pg

import (
	"context"
	"database/sql"
	"embed"

	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/pressly/goose/v3"

	"spreche"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, dsn string) (stores Stores, err error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return Stores{}, errors.Wrap(err, "opening db")
	}
	defer func() {
		if err != nil {
			db.Close()
		}
	}()
	goose.SetBaseFS(migrations)
	if err = goose.SetDialect("postgres"); err != nil {
		return Stores{}, errors.Wrap(err, "setting migration dialect")
	}
	err = goose.Up(db, "migrations")
	return Stores{
		Channels: channelStore{db: db},
		Comments: commentStore{db: db},
		Tenants:  tenantStore{db: db},
		Users:    userStore{db: db},
		db:       db,
	}, errors.Wrap(err, "performing db migrations")
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
