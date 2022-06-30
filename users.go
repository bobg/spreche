package spreche

import (
	"context"

	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
)

type UserStore interface {
	BySlackID(context.Context, int64, string) (*User, error)
	ByGHLogin(context.Context, int64, string) (*User, error)
	Add(context.Context, int64, *User) error
}

type User struct {
	SlackID string
	GHLogin string
}

func (s *Service) GHToSlackUsers(ctx context.Context, tenantID int64, ghUsers []*github.User) ([]string, error) {
	var result []string

	for _, ghUser := range ghUsers {
		if ghUser == nil || ghUser.Login == nil {
			continue
		}
		u, err := s.Users.ByGHLogin(ctx, tenantID, *ghUser.Login)
		if errors.Is(err, ErrNotFound) {
			debugf("No Slack user found for GitHub user %s", *ghUser.Login)
			continue
		}
		if err != nil {
			return nil, errors.Wrapf(err, "looking up user %s", *ghUser.Login)
		}
		debugf("Found Slack user %s for GitHub user %s", u.SlackID, *ghUser.Login)
		result = append(result, u.SlackID)
	}

	return result, nil
}
