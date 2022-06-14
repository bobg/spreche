package spreche

import (
	"context"

	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
)

func (s *Service) GHToSlackUsers(ctx context.Context, ghUsers []*github.User) ([]string, error) {
	var result []string

	for _, ghUser := range ghUsers {
		if ghUser == nil || ghUser.Name == nil {
			continue
		}
		u, err := s.Users.ByGithubName(ctx, *ghUser.Name)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, errors.Wrapf(err, "looking up user %s", *ghUser.Name)
		}
		result = append(result, u.SlackID)
	}

	return result, nil
}
