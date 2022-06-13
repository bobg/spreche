package spreche

import (
	"context"

	"github.com/google/go-github/v45/github"
)

func (s *Service) GHToSlackUsers(ctx context.Context, ghUsers []*github.User) ([]string, error) {
	var result []string

	for _, ghUser := range ghUsers {
		if ghUser == nil {
			continue
		}
		// xxx
	}

	return result, nil
}
