package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/logr"
)

type Option func(*Server)

func WithGracefulShutdownTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.gracefulShutdownTimeout = timeout
	}
}

type Server struct {
	httpServer              *http.Server
	logger                  *logr.Logger
	gracefulShutdownTimeout time.Duration
}

func NewServer(addr string, handler http.Handler, logger *logr.Logger, opts ...Option) *Server {
	srv := &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
		logger:                  logger,
		gracefulShutdownTimeout: 30 * time.Second,
	}
	for _, setOpt := range opts {
		setOpt(srv)
	}
	return srv
}

func (s *Server) Start(ctx context.Context) error {
	serverContext, stopServer := context.WithCancel(ctx)
	errChan := make(chan error)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		<-sig
		defer stopServer()

		shutdownCtx, cancel := context.WithTimeout(serverContext, s.gracefulShutdownTimeout)
		defer cancel()
		go func() {
			<-shutdownCtx.Done()
			s.logger.Info("Graceful shutdown timed out")
		}()

		s.logger.Info("Shutting down server")
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			errChan <- fmt.Errorf("error shutting down server: %v", err)
		}
	}()

	go func() {
		s.logger.Info("Server listening", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- fmt.Errorf("Error starting server: %v", err)
		}
	}()

	select {
	case <-serverContext.Done():
		return nil
	case err := <-errChan:
		return err
	}
}
