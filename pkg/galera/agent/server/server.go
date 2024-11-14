package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
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

func WithTLSEnabled(tlsEnabled bool) Option {
	return func(s *Server) {
		s.tlsEnabled = tlsEnabled
	}
}

func WithTLSCAPath(tlsCACertPath string) Option {
	return func(s *Server) {
		s.tlsCACertPath = tlsCACertPath
	}
}

func WithTLSCertPath(tlsCertPath string) Option {
	return func(s *Server) {
		s.tlsCertPath = tlsCertPath
	}
}

func WithTLSKeyPath(tlsKeyPath string) Option {
	return func(s *Server) {
		s.tlsKeyPath = tlsKeyPath
	}
}

type Server struct {
	httpServer              *http.Server
	logger                  *logr.Logger
	gracefulShutdownTimeout time.Duration

	tlsEnabled    bool
	tlsCACertPath string
	tlsCertPath   string
	tlsKeyPath    string
}

func NewServer(addr string, handler http.Handler, logger *logr.Logger, opts ...Option) (*Server, error) {
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

	if srv.tlsEnabled {
		srv.logger.Info("Configuring TLS")
		tlsConfig, err := srv.getTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("error getting TLS config: %v", err)
		}
		srv.httpServer.TLSConfig = tlsConfig
	}
	return srv, nil
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
		logger := s.logger.WithValues("addr", s.httpServer.Addr, "tls", s.tlsEnabled)
		listenFn := func() error {
			if s.tlsEnabled {
				return s.httpServer.ListenAndServeTLS("", "")
			}
			return s.httpServer.ListenAndServe()
		}

		logger.Info("Server listening")
		if err := listenFn(); err != http.ErrServerClosed {
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

func (s *Server) getTLSConfig() (*tls.Config, error) {
	if !s.tlsEnabled {
		return nil, errors.New("TLS must be enabled")
	}
	caCert, err := os.ReadFile(s.tlsCACertPath)
	if err != nil {
		return nil, fmt.Errorf("error reading CA cert: %v", err)
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		return nil, errors.New("unable to add CA cert to pool")
	}

	cert, err := tls.LoadX509KeyPair(s.tlsCertPath, s.tlsKeyPath)
	if err != nil {
		return nil, fmt.Errorf("error loading x509 keypair: %v", err)
	}

	return &tls.Config{
		ClientCAs:          caCertPool,
		ClientAuth:         tls.RequireAndVerifyClientCert,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: false,
	}, nil
}
