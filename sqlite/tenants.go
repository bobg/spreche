package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path"

	"github.com/bobg/sqlutil"
	"github.com/pkg/errors"

	"spreche"
)

type tenantStore struct {
	db *sql.DB
}

var _ spreche.TenantStore = tenantStore{}

func (t tenantStore) WithTenant(ctx context.Context, tenantID int64, repoURL, teamID string, f func(context.Context, *spreche.Tenant) error) error {
	const (
		qTenantID = `
			SELECT tenant_id, gh_installation_id, gh_priv_key, gh_api_url, gh_upload_url, slack_token
				FROM tenants
				WHERE tenant_id = $1
		`
		qRepo = `
			SELECT r.tenant_id, t.gh_installation_id, t.gh_priv_key, t.gh_api_url, t.gh_upload_url, t.slack_token
				FROM tenant_repos r, tenants t
				WHERE r.tenant_id = t.tenant_id AND r.repo_url = $1
		`
		qTeam = `
			SELECT tt.tenant_id, t.gh_installation_id, t.gh_priv_key, t.gh_api_url, t.gh_upload_url, t.slack_token
				FROM tenant_teams tt, tenants t
				WHERE tt.tenant_id = t.tenant_id AND tt.team_id = $1
		`
	)

	var (
		q      string
		arg    any
		isRepo bool
	)
	switch {
	case tenantID != 0:
		q, arg = qTenantID, tenantID
	case repoURL != "":
		q, arg = qRepo, repoURL
		isRepo = true
	case teamID != "":
		q, arg = qTeam, teamID
	default:
		return fmt.Errorf("WithTenant must be called with one of tenantID, repoURL, or teamID")
	}

	var tenant spreche.Tenant

	err := sqlutil.QueryRowContext(ctx, t.db, q, arg).Scan(
		&tenant.TenantID,
		&tenant.GHInstallationID,
		&tenant.GHPrivKey,
		&tenant.GHAPIURL,
		&tenant.GHUploadURL,
		&tenant.SlackToken,
	)
	if isRepo && errors.Is(err, sql.ErrNoRows) {
		u, err := url.Parse(repoURL)
		if err != nil {
			return errors.Wrapf(err, "parsing URL %s", repoURL)
		}
		u.Path = path.Dir(u.Path)
		err = sqlutil.QueryRowContext(ctx, t.db, q, u.String()).Scan(
			&tenant.TenantID,
			&tenant.GHInstallationID,
			&tenant.GHPrivKey,
			&tenant.GHAPIURL,
			&tenant.GHUploadURL,
			&tenant.SlackToken,
		)
		if errors.Is(err, sql.ErrNoRows) {
			// One more time.
			u.Path = path.Dir(u.Path)
			err = sqlutil.QueryRowContext(ctx, t.db, q, u.String()).Scan(
				&tenant.TenantID,
				&tenant.GHInstallationID,
				&tenant.GHPrivKey,
				&tenant.GHAPIURL,
				&tenant.GHUploadURL,
				&tenant.SlackToken,
			)
			// Fall through to the err check below.
		}
		// Fall through to the err check below.
	}
	if err != nil {
		return errors.Wrap(err, "getting tenant")
	}
	return f(ctx, &tenant)
}

func (t tenantStore) Add(ctx context.Context, vals *spreche.Tenant) error {
	const q = `INSERT INTO tenants (gh_installation_id, gh_priv_key, gh_api_url, gh_upload_url, slack_token) VALUES ($1, $2, $3, $4, $5)`
	res, err := t.db.ExecContext(ctx, q, vals.GHInstallationID, vals.GHPrivKey, vals.GHAPIURL, vals.GHUploadURL, vals.SlackToken)
	if err != nil {
		return errors.Wrap(err, "inserting tenant row")
	}
	vals.TenantID, err = res.LastInsertId()
	if err != nil {
		return errors.Wrap(err, "getting last insert ID")
	}

	for _, ghURL := range vals.GHURLs {
		err = t.AddGHURL(ctx, vals.TenantID, ghURL)
		if err != nil {
			return errors.Wrap(err, "adding GitHub URLs")
		}
	}

	for _, teamID := range vals.TeamIDs {
		err = t.AddTeam(ctx, vals.TenantID, teamID)
		if err != nil {
			return errors.Wrap(err, "adding team IDs")
		}
	}

	return nil
}

func (t tenantStore) AddGHURL(ctx context.Context, tenantID int64, ghURL string) error {
	const q = `INSERT INTO tenant_repos (tenant_id, gh_url) VALUES ($1, $2)`
	_, err := t.db.ExecContext(ctx, q, tenantID, ghURL)
	return err
}

func (t tenantStore) AddTeam(ctx context.Context, tenantID int64, teamID string) error {
	const q = `INSERT INTO tenant_teams (tenant_id, team_id) VALUES ($1, $2)`
	_, err := t.db.ExecContext(ctx, q, tenantID, teamID)
	return err
}

func (t tenantStore) Foreach(ctx context.Context, f func(*spreche.Tenant) error) error {
	const q = `SELECT tenant_id, gh_installation_id, gh_priv_key, gh_api_url, gh_upload_url, slack_token FROM tenants`
	return sqlutil.ForQueryRows(ctx, t.db, q, func(tenantID, ghInstallationID int64, ghPrivKey []byte, ghAPIURL, ghUploadURL, slackToken string) error {
		var tenant = &spreche.Tenant{
			TenantID:         tenantID,
			GHInstallationID: ghInstallationID,
			GHPrivKey:        ghPrivKey,
			GHAPIURL:         ghAPIURL,
			GHUploadURL:      ghUploadURL,
			SlackToken:       slackToken,
		}

		const qRepos = `SELECT gh_url FROM tenant_repos WHERE tenant_id = $1`
		err := sqlutil.ForQueryRows(ctx, t.db, qRepos, tenantID, func(ghURL string) {
			tenant.GHURLs = append(tenant.GHURLs, ghURL)
		})
		if err != nil {
			return errors.Wrap(err, "getting GitHub URLs")
		}

		const qTeams = `SELECT team_id FROM tenant_teams WHERE tenant_id = $1`
		err = sqlutil.ForQueryRows(ctx, t.db, qTeams, tenantID, func(teamID string) {
			tenant.TeamIDs = append(tenant.TeamIDs, teamID)
		})
		if err != nil {
			return errors.Wrap(err, "getting team IDs")
		}

		return f(tenant)
	})
}
