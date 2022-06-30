package spreche

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bobg/mid"
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
		),
		"list", tc.doList, "list tenants", nil,
	)
}

func (tc tenantcmd) doAdd(ctx context.Context, ghinst int64, ghprivfile, ghapi, ghupload, slacktoken string, args []string) error {
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

	for _, arg := range args {
		if strings.HasPrefix(arg, "http:") || strings.HasPrefix(arg, "https:") {
			tenant.GHURLs = append(tenant.GHURLs, arg)
		} else {
			tenant.TeamIDs = append(tenant.TeamIDs, arg)
		}
	}

	err = tc.s.Tenants.Add(ctx, tenant)
	if err != nil {
		return errors.Wrap(err, "adding new tenant")
	}

	w := mid.ResponseWriter(ctx)
	fmt.Fprintf(w, "New tenant ID %d\n", tenant.TenantID)
	return nil
}

func (tc tenantcmd) doAddTo(ctx context.Context, tenantID int64, args []string) error {
	for _, arg := range args {
		if strings.HasPrefix(arg, "http:") || strings.HasPrefix(arg, "https:") {
			err := tc.s.Tenants.AddGHURL(ctx, tenantID, arg)
			if err != nil {
				return errors.Wrapf(err, "adding GH URL %s to tenant", arg)
			}
		} else {
			err := tc.s.Tenants.AddTeam(ctx, tenantID, arg)
			if err != nil {
				return errors.Wrapf(err, "adding team ID %s to tenant", arg)
			}
		}
	}
	return nil
}

func (tc tenantcmd) doList(ctx context.Context, _ []string) error {
	return tc.s.Tenants.Foreach(ctx, func(t *Tenant) error {
		w := mid.ResponseWriter(ctx)
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(t)
	})
}
