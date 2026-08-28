package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cfichtmueller/redirectr/internal/api"
	"github.com/cfichtmueller/redirectr/internal/config"
	"github.com/cfichtmueller/redirectr/internal/handler"
	"github.com/cfichtmueller/redirectr/internal/shell"
	"golang.org/x/sync/errgroup"
)

var Release = "redirectr@development"

func main() {
	if err := mainE(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func mainE() error {
	config.Release = Release
	s, err := shell.Configure()
	if err != nil {
		return err
	}

	defer s.Teardown()

	apiEngine := api.Configure(s.HealthEndpoint)
	handlerEngine := handler.Configure()

	apiAddr := config.ApiHost + ":" + config.ApiPort
	handlerAddr := config.HandlerHost + ":" + config.HandlerPort

	apiServer := newServer(apiAddr, apiEngine.Handler())

	handlerServer := newServer(handlerAddr, handlerEngine)
	handlerServer.WriteTimeout = 10 * time.Second

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var g errgroup.Group
	g.Go(func() error { return serve(apiServer) })
	g.Go(func() error { return serve(handlerServer) })

	slog.Info("starting API", "address", apiAddr)
	slog.Info("starting handler", "address", handlerAddr)

	<-ctx.Done()
	slog.Info("shutdown signal received, draining")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("api server shutdown failed", "error", err)
	}
	if err := handlerServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("handler server shutdown failed", "error", err)
	}

	return g.Wait()
}

// serve runs the server until it is shut down; a graceful shutdown is not an error.
func serve(s *http.Server) error {
	if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:        addr,
		Handler:     handler,
		ReadTimeout: 30 * time.Second,
	}
}
