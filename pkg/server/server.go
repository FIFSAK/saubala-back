package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/FIFSAK/saubala-back/pkg/log"
)

type Option func(*Servers) error

type Servers struct {
	httpServer *http.Server
	httpListen net.Listener
	router     chi.Router
}

func NewServer(opts ...Option) (*Servers, error) {
	s := &Servers{}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return s, nil
}

func (s *Servers) Run(logger *zap.Logger) error {
	if logger == nil {
		logger = zap.NewNop()
	}

	if s.httpServer == nil {
		return fmt.Errorf("no servers configured to run")
	}

	addr := s.httpListen.Addr().String()
	go func() {
		logger.Info("starting http server", zap.String("addr", addr))
		if err := s.httpServer.Serve(s.httpListen); err != nil && err != http.ErrServerClosed {
			logger.Error("http serve failed", zap.String("addr", addr), zap.Error(err))
		}
		logger.Info("http server stopped", zap.String("addr", addr))
	}()

	return nil
}

func (s *Servers) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}

func (s *Servers) Router() chi.Router {
	return s.router
}

func WithHTTP(addr string) Option {
	return func(s *Servers) error {
		if s.httpServer != nil {
			return fmt.Errorf("http server already configured")
		}
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("listen http %s: %w", addr, err)
		}

		r := chi.NewRouter()
		r.Use(middleware.RequestID)
		r.Use(middleware.RealIP)
		r.Use(requestLogger)
		r.Use(middleware.Recoverer)

		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		})

		s.router = r
		s.httpListen = l
		s.httpServer = &http.Server{
			Handler:           r,
			ReadHeaderTimeout: 10 * time.Second,
		}
		return nil
	}
}

// requestLogger logs each HTTP request using the application's zap logger.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		defer func() {
			log.GetLogger().Info("http request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Int("bytes", ww.BytesWritten()),
				zap.Duration("duration", time.Since(start)),
				zap.String("request_id", middleware.GetReqID(r.Context())),
			)
		}()
		next.ServeHTTP(ww, r)
	})
}
