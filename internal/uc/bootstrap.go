package uc

import (
	"context"
	"log/slog"

	"github.com/cfichtmueller/redirectr/internal/domain/iam"
	"github.com/cfichtmueller/srv"
)

type BootstrapCommand struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (c *BootstrapCommand) Validate() error {
	v := RequireEmail("email", c.Email, nil)
	v = RequireUserPassword("password", c.Password, v)
	return srv.Validate(v)
}

func Bootstrap(c context.Context, cmd BootstrapCommand) error {
	userCount, err := iam.CountUsers(c)
	if err != nil {
		return err
	}
	if userCount > 0 {
		slog.Warn("ignoring bootstrap request - app is already bootstrapped")
		return nil
	}

	slog.Info("bootstrapping app")

	if _, err := iam.CreateUser(c, iam.CreateUserCommand{
		Email:    cmd.Email,
		Password: cmd.Password,
	}); err != nil {
		return err
	}

	return nil
}
