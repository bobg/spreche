package sqlite

import (
	"context"
	"database/sql"
	"fmt"

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
			SELECT gh_installation_id, gh_priv_key, gh_api_url, gh_upload_url, slack_token
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
		q   string
		arg any
	)
	switch {
	case tenantID != 0:
		q, arg = qTenantID, tenantID
	case repoURL != "":
		q, arg = qRepo, repoURL
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

	for _, repoURL := range vals.RepoURLs {
		err = t.AddRepo(ctx, vals.TenantID, repoURL)
		if err != nil {
			return errors.Wrap(err, "adding repo URLs")
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

func (t tenantStore) AddRepo(ctx context.Context, tenantID int64, repoURL string) error {
	const q = `INSERT INTO tenant_repos (tenant_id, repo_url) VALUES ($1, $2)`
	_, err := t.db.ExecContext(ctx, q, tenantID, repoURL)
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

		const qRepos = `SELECT repo_url FROM tenant_repos WHERE tenant_id = $1`
		err := sqlutil.ForQueryRows(ctx, t.db, qRepos, tenantID, func(repoURL string) {
			tenant.RepoURLs = append(tenant.RepoURLs, repoURL)
		})
		if err != nil {
			return errors.Wrap(err, "getting repo URLs")
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
