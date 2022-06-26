package spreche

import (
	"context"

	"github.com/bobg/subcmd/v2"
)

func (a admincmd) doUser(ctx context.Context, tenantID int64, args []string) error {
	return a.s.Tenants.WithTenant(ctx, tenantID, "", "", func(ctx context.Context, tenant *Tenant) error {
		return subcmd.Run(ctx, usercmd{s: a.s, tenant: tenant}, args)
	})
}

type usercmd struct {
	s      *Service
	tenant *Tenant
}

func (u usercmd) Subcmds() subcmd.Map {
	return subcmd.Commands(
		"add", u.doAdd, "add a user", subcmd.Params(
			"-slack", subcmd.String, "", "Slack user ID",
			"-github", subcmd.String, "", "GitHub login",
		),
		/*
			"del", u.doDel, "remove a user", subcmd.Params(
				"-slack", subcmd.String, "", "Slack user ID",
				"-github", subcmd.String, "", "GitHub login",
			),
			"list", u.doList, "list users", nil,
		*/
	)
}

func (u usercmd) doAdd(ctx context.Context, slackID, githubLogin string, _ []string) error {
	return u.s.Users.Add(ctx, u.tenant.TenantID, &User{
		SlackID:    slackID,
		GithubName: githubLogin,
	})
}
