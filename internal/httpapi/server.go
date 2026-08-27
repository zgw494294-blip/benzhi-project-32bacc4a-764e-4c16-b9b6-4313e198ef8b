package httpapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

type Server struct{ HTTP *http.Server }

func NewServer(address string, handler http.Handler) *Server {
	return &Server{HTTP: &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 12 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}}
}

func (s *Server) Serve(listener net.Listener) error {
	err := s.HTTP.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error { return s.HTTP.Shutdown(ctx) }
