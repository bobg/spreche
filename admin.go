package spreche

import (
	"context"
	"net/http"

	"github.com/bobg/mid"
	"github.com/bobg/subcmd/v2"
)

type AdminCmd struct {
	Key  string   `json:"key"`
	Args []string `json:"args"`
}

type admincmd struct {
	s          *Service
	httpServer *http.Server
	ch         chan struct{}
}

func (s *Service) OnAdmin(httpServer *http.Server, ch chan struct{}) func(context.Context, AdminCmd) error {
	return func(ctx context.Context, cmd AdminCmd) error {
		if cmd.Key != s.AdminKey {
			return mid.CodeErr{C: http.StatusUnauthorized}
		}
		a := admincmd{
			s:          s,
			httpServer: httpServer,
			ch:         ch,
		}
		return subcmd.Run(ctx, a, cmd.Args)
	}
}

func (a admincmd) Subcmds() subcmd.Map {
	return subcmd.Commands(
		"shutdown", a.doShutdown, "shut down", nil,
		"user", a.doUser, "manage users", subcmd.Params(
			"-tenant", subcmd.Int64, 0, "tenant ID",
		),
		"tenant", a.doTenant, "manage tenants", nil,
	)
}

func (a admincmd) doShutdown(ctx context.Context, _ []string) error {
	// Run the following in a goroutine,
	// so this (presumably running in an http server handler) can finish,
	// which is required for the call to Shutdown to finish.
	// Deadlock otherwise.
	go func() {
		a.httpServer.Shutdown(ctx)
		close(a.ch)
	}()
	return nil
}
