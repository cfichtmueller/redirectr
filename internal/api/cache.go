package api

import (
	"github.com/cfichtmueller/redirectr/internal/domain/redirect"
	"github.com/cfichtmueller/srv"
)

func configureCacheApi(s *srv.Server) {
	g := s.Group("/api/v1/cache", authenticated)

	g.GET("", handleGetCacheStats)
	g.POST("/clear", handleClearCache)
}

func handleGetCacheStats(c *srv.Context) *srv.Response {
	stats := redirect.GetCacheStats()
	return srv.Respond().Json(stats)
}

func handleClearCache(c *srv.Context) *srv.Response {
	redirect.ClearCache()
	return srv.Respond().NoContent()
}
