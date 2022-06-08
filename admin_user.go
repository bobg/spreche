package spreche

import (
	"context"

	"github.com/bobg/subcmd/v2"
)

func (a admincmd) doUser(ctx context.Context, args []string) error {
	return subcmd.Run(ctx, usercmd{s: a.s}, args)
}

type usercmd struct{ s *Service }

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
	return u.s.Users.Add(ctx, &User{
		SlackID:    slackID,
		GithubName: githubLogin,
	})
}
