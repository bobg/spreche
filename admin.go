package spreche

import (
	"context"
	"net/http"

	"github.com/bobg/mid"
	"github.com/bobg/subcmd/v2"
)

type AdminCmd struct {
	Key  string   `json:"key"`
	Name string   `json:"name"`
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
		return subcmd.Run(ctx, admincmd{s: s, httpServer: httpServer, ch: ch}, cmd.Args)
	}
}

func (a admincmd) Subcmds() subcmd.Map {
	return subcmd.Commands(
		"shutdown", a.doShutdown, "shut down", nil,
		"user", a.doUser, "manage users", nil,
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

func (a admincmd) doUser(ctx context.Context, args []string) error {
	return subcmd.Run(ctx, usercmd{s: a.s}, args)
}

type usercmd struct{ s *Service }

func (u usercmd) Subcmds() subcmd.Map {
	return subcmd.Commands(
		"add", u.doAdd, "add a user", subcmd.Params(
			"-slackid", subcmd.String, "", "Slack user ID",
			"-slackname", subcmd.String, "", "Slack name",
			"-githublogin", subcmd.String, "", "GitHub login",
		),
		"del", u.doDel, "remove a user", subcmd.Params(
			"-slackid", subcmd.String, "", "Slack user ID",
			"-slackname", subcmd.String, "", "Slack name",
			"-githublogin", subcmd.String, "", "GitHub login",
		),
		"list", u.doList, "list users", nil,
	)
}
