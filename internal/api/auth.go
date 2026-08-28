package api

import (
	"errors"
	"strings"

	"github.com/cfichtmueller/redirectr/internal/domain/iam"
	"github.com/cfichtmueller/srv"
)

func configureAuthApi(s *srv.Server) {
	s.POST("/api/v1/login", handleLogin)
	s.POST("/api/v1/logout", handleLogout, authenticated)

	s.GET("/api/v1/me", handleGetMe)
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r *LoginRequest) Validate() error {
	v := srv.RequireNotEmpty("username", r.Username, nil)
	v = srv.RequireNotEmpty("password", r.Password, v)
	return srv.Validate(v)
}

func handleLogin(c *srv.Context) *srv.Response {
	var cmd LoginRequest
	if r := c.BindJSON(&cmd); r != nil {
		return r
	}
	p, err := iam.AuthenticateUser(c, cmd.Username, cmd.Password)
	if err != nil {
		return responseFromError(err)
	}
	s, err := iam.CreateSession(c, p.ID)
	if err != nil {
		return responseFromError(err)
	}
	return srv.Respond().Json(map[string]string{"access_token": "s-" + s.ID})
}

func handleLogout(c *srv.Context) *srv.Response {
	s := contextMustGetSession(c)
	if err := iam.DeleteSession(c, s.ID); err != nil {
		return responseFromError(err)
	}
	return srv.Respond().NoContent()
}

type MeResponse struct {
	Authenticated bool `json:"authenticated"`
}

var unauthenticatedResponse = srv.Respond().Json(MeResponse{
	Authenticated: false,
})

func handleGetMe(c *srv.Context) *srv.Response {
	token, ok := getBearerToken(c)
	if !ok || !strings.HasPrefix(token, "s-") {
		return unauthenticatedResponse
	}

	sessionId := strings.TrimPrefix(token, "s-")
	s, err := iam.GetSession(c, sessionId)
	if err != nil {
		if errors.Is(err, iam.ErrSessionNotFound) {
			return unauthenticatedResponse
		}
		return responseFromError(err)
	}
	if s.IsExpired() {
		return unauthenticatedResponse
	}

	return srv.Respond().Json(MeResponse{
		Authenticated: true,
	})
}
