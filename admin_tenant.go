package spreche

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bobg/subcmd/v2"
	"github.com/pkg/errors"
)

func (a admincmd) doTenant(ctx context.Context, args []string) error {
	return subcmd.Run(ctx, tenantcmd{s: a.s}, args)
}

type tenantcmd struct{ s *Service }

func (tc tenantcmd) Subcmds() subcmd.Map {
	return subcmd.Commands(
		"add", tc.doAdd, "add a tenant", subcmd.Params(
			"-ghinst", subcmd.Int64, 0, "GitHub installation ID",
			"-ghpriv", subcmd.String, "", "path to file containing GitHub private key",
			"-ghapi", subcmd.String, "", "GitHub API URL",
			"-ghupload", subcmd.String, "", "GitHub upload URL",
			"-slacktoken", subcmd.String, "", "Slack token",
		),
		"addto", tc.doAddTo, "add a GitHub repo and/or a Slack team to a tenant", subcmd.Params(
			"-tenant", subcmd.Int64, 0, "tenant ID",
			"-repo", subcmd.String, "", "GitHub repo URL",
			"-team", subcmd.String, "", "Slack team ID",
		),
		"list", tc.doList, "list tenants", nil,
	)
}

func (tc tenantcmd) doAdd(ctx context.Context, ghinst int64, ghprivfile, ghapi, ghupload, slacktoken string, _ []string) error {
	ghpriv, err := os.ReadFile(ghprivfile)
	if err != nil {
		return errors.Wrap(err, "reading privkey file")
	}
	tenant := &Tenant{
		GHInstallationID: ghinst,
		GHPrivKey:        ghpriv,
		GHAPIURL:         ghapi,
		GHUploadURL:      ghupload,
		SlackToken:       slacktoken,
	}
	err = tc.s.Tenants.Add(ctx, tenant)
	if err != nil {
		return errors.Wrap(err, "adding new tenant")
	}
	fmt.Printf("New tenant ID %d\n", tenant.TenantID)
	return nil
}

func (tc tenantcmd) doAddTo(ctx context.Context, tenantID int64, repoURL, teamID string, _ []string) error {
	if repoURL != "" {
		err := tc.s.Tenants.AddRepo(ctx, tenantID, repoURL)
		if err != nil {
			return errors.Wrap(err, "adding repo to tenant")
		}
	}
	if teamID != "" {
		err := tc.s.Tenants.AddTeam(ctx, tenantID, teamID)
		if err != nil {
			return errors.Wrap(err, "adding team to tenant")
		}
	}
	return nil
}

func (tc tenantcmd) doList(ctx context.Context, _ []string) error {
	return tc.s.Tenants.Foreach(ctx, func(t *Tenant) error {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(t)
	})
}
