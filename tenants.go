package spreche

import (
	"context"

	"github.com/slack-go/slack"
)

// TenantStore is a persistent store for information about tenants of the other types of store.
type TenantStore interface {
	// WithTenant finds a suitable tenant and runs the given callback with it.
	// If tenantID is non-zero, that's identifies the tenant to use.
	// Otherwise, one of repoURL and teamID must be specified, and the associated tenant is found.
	// If specified, repoURL must be the HTML URL of a GitHub repo.
	// The tenant may be associated with it directly,
	// or with its parent (the containing GitHub user or org),
	// or _its_ parent.
	// See Tenant.GHURLs.
	WithTenant(ctx context.Context, tenantID int64, repoURL, teamID string, f func(context.Context, *Tenant) error) error

	// Add adds a new tenant to the store.
	// The values for the new tenant are in the given *Tenant object.
	// On a successful return, the TenantID field of the object is populated with the new ID.
	Add(context.Context, *Tenant) error

	AddGHURL(context.Context, int64, string) error
	AddTeam(context.Context, int64, string) error
	Foreach(context.Context, func(*Tenant) error) error
}

type Tenant struct {
	TenantID         int64  `json:"tenant_id"`
	GHInstallationID int64  `json:"gh_installation_id"`
	GHPrivKey        []byte `json:"-"`
	GHAPIURL         string `json:"gh_api_url"`
	GHUploadURL      string `json:"gh_upload_url"`
	SlackToken       string `json:"-"`

	// GHURLs is a list of GitHub URLs associated with this tenant.
	// Each URL may be a repo's HTML URL,
	// or its parent (to cover all the repos for a user or org),
	// or _its_ parent (to cover all the repos for a server).
	GHURLs []string `json:"gh_urls,omitempty"`

	TeamIDs []string `json:"team_ids,omitempty"`
}

func (t *Tenant) SlackClient() *slack.Client {
	return slack.New(t.SlackToken)
}
