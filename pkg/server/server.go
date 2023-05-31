package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/go-logr/logr"
)

// server is a general purpose HTTP server Runnable for a manager
// to serve some internal handlers such as health probes, metrics and profiling.
type server struct {
	kind     string
	log      logr.Logger
	server   *http.Server
	listener net.Listener
}

// NewServer returns a new server with sane defaults.
func NewServer(kind string) *server {
	return &server{
		kind: kind,
	}
}

func (s *server) Start(ctx context.Context) error {
	log := s.log.WithValues("kind", s.kind, "addr", s.listener.Addr())

	serverShutdown := make(chan struct{})
	go func() {
		<-ctx.Done()
		log.Info("shutting down server")
		if err := s.server.Shutdown(context.Background()); err != nil {
			log.Error(err, "error shutting down server")
		}
		close(serverShutdown)
	}()

	log.Info("starting server")
	if err := s.server.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	<-serverShutdown
	return nil
}

func (s *server) SetupWithManager(mgr ctrl.Manager) error {
	mux := http.NewServeMux()
	srv := &http.Server{
		Handler:           mux,
		MaxHeaderBytes:    1 << 20,
		IdleTimeout:       90 * time.Second, // matches http.DefaultTransport keep-alive timeout
		ReadHeaderTimeout: 32 * time.Second,
	}

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	ln, err := net.Listen("tcp", ":9099")
	if err != nil {
		return fmt.Errorf("error listening on 9099: %w", err)
	}

	s.server = srv
	s.listener = ln
	s.log = mgr.GetLogger()

	return mgr.Add(s)
}
