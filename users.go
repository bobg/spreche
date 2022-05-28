package crocs

import (
	"context"

	"github.com/google/go-github/v44/github"
)

type UserMapping struct {
	User string
	Err  error
}

func (s *Service) GHToSlackUsers(ctx context.Context, ghUsers []*github.User) (map[string]UserMapping, error) {
	result := make(map[string]UserMapping)
	for _, ghUser := range ghUsers {
		if ghUser == nil {
			continue
		}
		// xxx
	}

	return result, nil
}
