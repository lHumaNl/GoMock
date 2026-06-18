package server

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const shutdownTimeout = 5 * time.Second

type HTTPServer struct {
	server *http.Server
	logger *slog.Logger
	tls    *TLSConfig
}

type TLSConfig struct {
	CertFile   string
	KeyFile    string
	MinVersion uint16
}

func NewHTTPServer(address string, handler http.Handler, logger *slog.Logger) *HTTPServer {
	return &HTTPServer{server: &http.Server{Addr: address, Handler: handler}, logger: logger}
}

func NewHTTPSServer(address string, handler http.Handler, logger *slog.Logger, config TLSConfig) *HTTPServer {
	tlsConfig := &tls.Config{MinVersion: config.MinVersion}
	server := &http.Server{Addr: address, Handler: handler, TLSConfig: tlsConfig}
	return &HTTPServer{server: server, logger: logger, tls: &config}
}

func (s *HTTPServer) Run(ctx context.Context) error {
	errorsChan := make(chan error, 1)
	go s.listen(errorsChan)

	select {
	case err := <-errorsChan:
		return err
	case <-ctx.Done():
		return s.shutdown(errorsChan)
	}
}

func (s *HTTPServer) listen(errorsChan chan<- error) {
	s.logger.Info("server listening", "address", s.server.Addr, "scheme", s.scheme())
	err := s.listenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		errorsChan <- nil
		return
	}
	errorsChan <- err
}

func (s *HTTPServer) listenAndServe() error {
	if s.tls == nil {
		return s.server.ListenAndServe()
	}
	return s.server.ListenAndServeTLS(s.tls.CertFile, s.tls.KeyFile)
}

func (s *HTTPServer) scheme() string {
	if s.tls == nil {
		return "http"
	}
	return "https"
}

func (s *HTTPServer) shutdown(errorsChan <-chan error) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	s.logger.Info("http server shutting down")
	if err := s.server.Shutdown(ctx); err != nil {
		return err
	}
	return <-errorsChan
}
