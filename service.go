package spreche

import (
	"log"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
)

type Service struct {
	AdminKey           string
	GHSecret           string
	SlackSigningSecret string

	Channels ChannelStore
	Comments CommentStore
	Tenants  TenantStore
	Users    UserStore
}

var ErrNotFound = errors.New("not found")

const ghAppID = 207677 // https://github.com/settings/apps/spreche

func (t *Tenant) GHClient() (*github.Client, error) {
	itr, err := ghinstallation.New(http.DefaultTransport, ghAppID, t.GHInstallationID, t.GHPrivKey)
	if err != nil {
		return nil, errors.Wrap(err, "creating transport for GitHub client")
	}
	itr.BaseURL = t.GHAPIURL
	return github.NewEnterpriseClient(t.GHAPIURL, t.GHUploadURL, &http.Client{Transport: itr})
}

func debugf(format string, args ...any) {
	log.Printf(format, args...)
}
