package api

import (
	"github.com/cfichtmueller/redirectr/internal/infra/health"
	"github.com/cfichtmueller/srv"
)

func Configure(he *health.Endpoint) *srv.Server {
	s := srv.NewServer().Use(sentryMiddleware, srv.LoggingMiddleware())

	s.GET("/internal/readiness", func(c *srv.Context) *srv.Response {
		return srv.Respond().Text("OK\n")
	})
	s.GET("/internal/liveness", func(c *srv.Context) *srv.Response {
		return srv.Respond().Text("OK\n")
	})
	s.GET("/internal/health", func(c *srv.Context) *srv.Response {
		s, err := he.GetHealth(c)
		if err != nil {
			return responseFromError(err)
		}
		return srv.Respond().Json(s)
	})

	configureBootstrapApi(s)
	configureAuthApi(s)
	configureCacheApi(s)
	configureRedirectApi(s)

	return s
}
