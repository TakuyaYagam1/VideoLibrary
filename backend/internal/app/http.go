package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	logkit "github.com/wahrwelt-kit/go-logkit"
	"golang.org/x/sync/errgroup"
)

type HTTPServer struct {
	server          *http.Server
	shutdownTimeout time.Duration
	logger          logkit.Logger
}

func NewHTTPServer(cfg config.HTTP, handler http.Handler, logger logkit.Logger) *HTTPServer {
	return &HTTPServer{
		server: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			MaxHeaderBytes:    cfg.MaxHeaderBytes,
		},
		shutdownTimeout: cfg.ShutdownTimeout,
		logger:          logger,
	}
}

func (s *HTTPServer) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("listen http: %w", err)
	}

	return s.serve(ctx, listener)
}

func (s *HTTPServer) serve(ctx context.Context, listener net.Listener) error {
	logger := s.logger
	if logger == nil {
		logger = logkit.Noop()
	}

	s.server.BaseContext = func(net.Listener) context.Context {
		return ctx
	}

	group, groupCtx := errgroup.WithContext(ctx)
	serveDone := make(chan struct{})
	group.Go(func() error {
		defer close(serveDone)
		logger.InfoContext(groupCtx, "http server started", logkit.Component("http"), logkit.Fields{
			"addr": listener.Addr().String(),
		})
		if err := s.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve http: %w", err)
		}

		return nil
	})
	group.Go(func() error {
		select {
		case <-groupCtx.Done():
		case <-serveDone:
			return nil
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http: %w", err)
		}

		logger.InfoContext(context.Background(), "http server stopped", logkit.Component("http"))
		return nil
	})

	if err := group.Wait(); err != nil {
		return err
	}

	return nil
}
