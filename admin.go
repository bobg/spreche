package crocs

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bobg/mid"
)

type AdminCmd struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

func (s *Service) OnAdmin(httpServer *http.Server, ch chan struct{}) func(context.Context, AdminCmd) error {
	return func(ctx context.Context, cmd AdminCmd) error {
		if cmd.Key != s.AdminKey {
			return mid.CodeErr{C: http.StatusUnauthorized}
		}
		switch cmd.Name {
		case "shutdown":
			err := httpServer.Shutdown(ctx)
			close(ch)
			return err
		}

		return mid.CodeErr{
			C:   http.StatusBadRequest,
			Err: fmt.Errorf("unknown admin command %s", cmd.Name),
		}
	}
}
