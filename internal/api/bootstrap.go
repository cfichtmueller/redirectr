package api

import (
	"github.com/cfichtmueller/redirectr/internal/uc"
	"github.com/cfichtmueller/srv"
)

func configureBootstrapApi(s *srv.Server) {
	s.POST("/api/v1/bootstrap", handleBootstrap)
}

func handleBootstrap(c *srv.Context) *srv.Response {
	var cmd uc.BootstrapCommand
	if r := c.BindJson(&cmd); r != nil {
		return r
	}

	if err := uc.Bootstrap(c, cmd); err != nil {
		return responseFromError(err)
	}

	return srv.Respond().NoContent()
}
