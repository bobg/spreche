package sqlite

import (
	"context"
	"database/sql"
	"embed"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/pressly/goose/v3"

	"spreche"
)

type Stores struct {
	Channels spreche.ChannelStore
	Comments spreche.CommentStore
	Tenants  spreche.TenantStore
	Users    spreche.UserStore

	db *sql.DB
}

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, conn string) (stores Stores, err error) {
	db, err := sql.Open("sqlite3", conn)
	if err != nil {
		return Stores{}, errors.Wrapf(err, "opening %s", conn)
	}
	defer func() {
		if err != nil {
			db.Close()
		}
	}()
	goose.SetBaseFS(migrations)
	if err = goose.SetDialect("sqlite3"); err != nil {
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

func (s Stores) Close() error {
	return s.db.Close()
}
