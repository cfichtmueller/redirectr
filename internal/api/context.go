package api

import (
	"github.com/cfichtmueller/redirectr/internal/domain/iam"
	"github.com/cfichtmueller/redirectr/internal/domain/redirect"
	"github.com/cfichtmueller/redirectr/internal/infra/auth"
	"github.com/cfichtmueller/srv"
)

const (
	contextKeyPrincipal         = "principal"
	contextKeyRedirect          = "redirect"
	contextKeySentryHub         = "sentry_hub"
	contextKeySentryTransaction = "sentry_transaction"
	contextKeySession           = "session"
)

func contextSetPrincipal(c *srv.Context, p *auth.Principal) {
	c.Set(contextKeyPrincipal, p)
}

func contextMustGetPrincipal(c *srv.Context) *auth.Principal {
	return c.MustGet(contextKeyPrincipal).(*auth.Principal)
}

func contextSetSession(c *srv.Context, s *iam.Session) {
	c.Set(contextKeySession, s)
}

func contextMustGetSession(c *srv.Context) *iam.Session {
	return c.MustGet(contextKeySession).(*iam.Session)
}

func contextSetRedirect(c *srv.Context, r *redirect.Redirect) {
	c.Set(contextKeyRedirect, r)
}

func contextMustGetRedirect(c *srv.Context) *redirect.Redirect {
	return c.MustGet(contextKeyRedirect).(*redirect.Redirect)
}
