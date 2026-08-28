package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cfichtmueller/redirectr/internal/domain/iam"
	"github.com/cfichtmueller/redirectr/internal/domain/redirect"
	"github.com/cfichtmueller/redirectr/internal/ec"
	"github.com/cfichtmueller/redirectr/internal/infra/auth"
	"github.com/cfichtmueller/srv"
	"github.com/getsentry/sentry-go"
)

func getBearerToken(c *srv.Context) (string, bool) {
	authHeader := c.Header("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}
	return strings.TrimPrefix(authHeader, "Bearer "), true
}

func authenticated(c *srv.Context, next srv.Handler) *srv.Response {
	token, ok := getBearerToken(c)
	if !ok {
		return responseFromError(ec.UnauthorizedMessage("authentication required"))
	}
	p, ok, r := authenticatedSession(c, token)
	if r != nil {
		return r
	}
	if ok {
		contextSetPrincipal(c, p)
		return next(c)
	}

	return responseFromError(ec.Unauthorized)
}

func authenticatedSession(c *srv.Context, t string) (*auth.Principal, bool, *srv.Response) {
	if !strings.HasPrefix(t, "s-") {
		return nil, false, nil
	}
	sessionId := strings.TrimPrefix(t, "s-")
	s, err := iam.GetSession(c, sessionId)
	if err != nil {
		if errors.Is(err, iam.ErrSessionNotFound) {
			return nil, false, responseFromError(ec.InvalidCredentials)
		}
		return nil, false, responseFromError(err)
	}
	if s.IsExpired() {
		return nil, false, responseFromError(ec.InvalidCredentials)
	}
	contextSetSession(c, s)
	p, err := iam.GetPrincipal(c, s.User)
	if err != nil {
		return nil, false, responseFromError(err)
	}
	return p, true, nil
}

func sentryMiddleware(c *srv.Context, next srv.Handler) *srv.Response {
	r := c.Request()
	transactionName := r.URL.Path

	hub := sentry.GetHubFromContext(c)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
	}

	options := []sentry.SpanOption{
		sentry.ContinueTrace(hub, r.Header.Get(sentry.SentryTraceHeader), r.Header.Get(sentry.SentryBaggageHeader)),
		sentry.WithOpName("http.server"),
		sentry.WithTransactionSource(sentry.SourceURL),
		sentry.WithSpanOrigin(sentry.SpanOriginStdLib),
	}

	transaction := sentry.StartTransaction(
		sentry.SetHubOnContext(c, hub),
		fmt.Sprintf("%s %s", r.Method, transactionName),
		options...,
	)
	transaction.SetData("http.request.method", r.Method)

	hub.Scope().SetRequest(r)
	c.Set(contextKeySentryHub, hub)
	c.Set(contextKeySentryTransaction, transaction)

	defer recoverWithSentry(hub, r)

	res := next(c)
	status := res.StatusCode

	defer func() {
		transaction.Status = sentry.HTTPtoSpanStatus(status)
		transaction.SetData("http.response.status_code", status)
		transaction.Finish()
	}()

	return res
}

func recoverWithSentry(hub *sentry.Hub, r *http.Request) {
	if err := recover(); err != nil {
		hub.RecoverWithContext(
			context.WithValue(r.Context(), sentry.RequestContextKey, r),
			err,
		)
	}
}

func withRedirectFromPath(c *srv.Context, next srv.Handler) *srv.Response {
	redirectId := c.PathValue("redirectId")
	if redirectId == "" {
		return responseFromError(ec.NoSuchRedirect)
	}

	r, err := redirect.FindOne(c, &redirect.Filter{
		ID: redirectId,
	})
	if err != nil {
		return responseFromError(err)
	}

	contextSetRedirect(c, r)
	return next(c)
}
