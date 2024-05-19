package internal

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prathoss/hw/pkg"
)

func NewServer(config Config) (*Server, error) {
	pool, err := pgxpool.New(context.Background(), config.DatabaseDSN)
	if err != nil {
		return nil, err
	}
	return &Server{
		db: pool,
	}, nil
}

type Server struct {
	db *pgxpool.Pool
}

func (s *Server) handleHealth(_ http.ResponseWriter, r *http.Request) (any, error) {
	err := s.db.Ping(r.Context())
	if err != nil {
		return nil, pkg.NewServiceUnavailableError(err)
	}
	return nil, nil
}

func (s *Server) Run() error {
	mux := http.NewServeMux()

	mux.Handle("GET /api/v1/health", pkg.HttpHandler(s.handleHealth))

	server := &http.Server{
		Addr: ":8080",
		Handler: pkg.CorrelationHandler(
			pkg.LoggingHandler(
				mux,
			),
		),
		ReadTimeout:       100 * time.Millisecond,
		ReadHeaderTimeout: 50 * time.Millisecond,
		WriteTimeout:      100 * time.Millisecond,
		IdleTimeout:       10 * time.Second,
		ErrorLog:          slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	}

	return pkg.ServeWithShutdown(server)
}
