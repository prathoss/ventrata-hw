package internal

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prathoss/hw/pkg"
)

func NewServer(config Config) (*Server, error) {
	pool, err := pgxpool.New(context.Background(), config.DatabaseDSN)
	if err != nil {
		return nil, err
	}
	productStore := NewProductRepository(pool)
	return &Server{
		db:           pool,
		config:       config,
		productStore: productStore,
	}, nil
}

type Server struct {
	db           *pgxpool.Pool
	config       Config
	productStore ProductStorer
}

func (s *Server) handleHealth(_ http.ResponseWriter, r *http.Request) (any, error) {
	err := s.db.Ping(r.Context())
	if err != nil {
		return nil, pkg.NewServiceUnavailableError(err)
	}
	return nil, nil
}

func (s *Server) listProducts(_ http.ResponseWriter, r *http.Request) (any, error) {
	invalidParams := make([]pkg.InvalidParam, 0, 10)

	capability := getCapabilityHeader(r)
	validationErrors := validateCapability(capability)
	invalidParams = append(invalidParams, validationErrors...)

	if len(invalidParams) > 0 {
		return nil, pkg.NewBadRequestError(invalidParams...)
	}

	// TODO: handle capability
	return s.productStore.ListProducts(r.Context())
}

func (s *Server) getProductDetail(_ http.ResponseWriter, r *http.Request) (any, error) {
	invalidParams := make([]pkg.InvalidParam, 0, 10)

	idStr := r.PathValue("id")
	id, validationErrors := validateID(idStr)
	invalidParams = append(invalidParams, validationErrors...)

	capability := getCapabilityHeader(r)
	validationErrors = validateCapability(capability)
	invalidParams = append(invalidParams, validationErrors...)

	if len(invalidParams) > 0 {
		return nil, pkg.NewBadRequestError(invalidParams...)
	}

	// TODO: handle capability
	return s.productStore.GetProduct(r.Context(), id)
}

func (s *Server) listAvailability(w http.ResponseWriter, r *http.Request) (any, error) {
	// TODO: implement
	panic("not implemented")
}

func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) (any, error) {
	// TODO: implement
	panic("not implemented")
}

func (s *Server) getBookingDetail(w http.ResponseWriter, r *http.Request) (any, error) {
	// TODO: implement
	panic("not implemented")
}

func (s *Server) confirmBooking(w http.ResponseWriter, r *http.Request) (any, error) {
	// TODO: implement
	panic("not implemented")
}

func validateCapability(capability string) []pkg.InvalidParam {
	if capability != "" && capability != "pricing" {
		return []pkg.InvalidParam{
			{
				Name:   "Capability",
				Reason: "Capability header contains unexpected value, allowed values are: pricing",
			},
		}
	}
	return nil
}

func validateID(id string) (uuid.UUID, []pkg.InvalidParam) {
	if id == "" {
		return uuid.UUID{}, []pkg.InvalidParam{
			{
				Name:   "ID",
				Reason: "path variable ID is missing",
			},
		}
	}
	typedId, err := uuid.Parse(id)
	if err != nil {
		return uuid.UUID{}, []pkg.InvalidParam{
			{
				Name:   "ID",
				Reason: "path variable ID is malformed",
			},
		}
	}
	return typedId, nil
}

func getCapabilityHeader(r *http.Request) string {
	capability := r.Header.Get("Capability")
	return capability
}

func (s *Server) Run() error {
	mux := http.NewServeMux()

	mux.Handle("GET /api/v1/health", pkg.HttpHandler(s.handleHealth))
	mux.Handle("GET /api/v1/products", pkg.HttpHandler(s.listProducts))
	mux.Handle("GET /api/v1/products/{id}", pkg.HttpHandler(s.getProductDetail))
	mux.Handle("POST /api/v1/availability", pkg.HttpHandler(s.listAvailability))
	mux.Handle("POST /api/v1/bookings", pkg.HttpHandler(s.createBooking))
	mux.Handle("GET /api/v1/bookings/{id}", pkg.HttpHandler(s.getBookingDetail))
	mux.Handle("POST /api/v1/bookings/{id}/confirm", pkg.HttpHandler(s.confirmBooking))

	server := &http.Server{
		Addr: s.config.ServerAddress,
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
