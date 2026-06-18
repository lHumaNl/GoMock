package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const shutdownTimeout = 5 * time.Second

type HTTPServer struct {
	server *http.Server
	logger *slog.Logger
}

func NewHTTPServer(address string, handler http.Handler, logger *slog.Logger) *HTTPServer {
	return &HTTPServer{server: &http.Server{Addr: address, Handler: handler}, logger: logger}
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
	s.logger.Info("http server listening", "address", s.server.Addr)
	err := s.server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		errorsChan <- nil
		return
	}
	errorsChan <- err
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
