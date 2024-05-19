package internal

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prathoss/hw/pkg"
)

const CapabilityPricing = "pricing"

//go:embed openapi.yaml
var openApi []byte

func NewServer(config Config) (*Server, error) {
	pool, err := pgxpool.New(context.Background(), config.DatabaseDSN)
	if err != nil {
		return nil, err
	}
	return &Server{
		db:           pool,
		config:       config,
		productStore: NewProductRepository(pool),
		pricingStore: NewPricingRepository(pool),
	}, nil
}

type Server struct {
	db           *pgxpool.Pool
	config       Config
	productStore ProductStorer
	pricingStore PricingStorer
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

	products, err := s.productStore.ListProducts(r.Context())
	if err != nil {
		return nil, err
	}

	if capability == CapabilityPricing {
		productIDs := make([]uuid.UUID, 0, len(products))
		for _, product := range products {
			productIDs = append(productIDs, product.ID)
		}
		pricing, err := s.pricingStore.GetPricingByProductId(r.Context(), productIDs, "EUR")
		if err != nil {
			return nil, err
		}
		pricedProducts := make([]PricedProduct, 0, len(products))
		for _, product := range products {
			pricing, ok := pricing[product.ID]
			if !ok {
				return nil, pkg.NewNotFoundError(fmt.Sprintf("could not find pricing for product %s", product.ID))
			}
			pricedProducts = append(pricedProducts, PricedProduct{
				Product: product,
				Pricing: pricing,
			})
		}
		return pricedProducts, nil
	}
	return products, nil
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

	product, err := s.productStore.GetProduct(r.Context(), id)
	if err != nil {
		return nil, err
	}

	if capability == CapabilityPricing {
		pricing, err := s.pricingStore.GetPricingByProductId(r.Context(), []uuid.UUID{product.ID}, "EUR")
		if err != nil {
			return nil, err
		}
		p, ok := pricing[product.ID]
		if !ok {
			return nil, pkg.NewNotFoundError(fmt.Sprintf("could not find pricing for product %s", product.ID))
		}
		pricedProduct := PricedProduct{
			Product: product,
			Pricing: p,
		}
		return pricedProduct, nil
	}
	return product, nil
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
	if capability != "" && capability != CapabilityPricing {
		return []pkg.InvalidParam{
			{
				Name:   "Capability",
				Reason: fmt.Sprintf("Capability header contains unexpected value, allowed values are: %s", CapabilityPricing),
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

	mux.HandleFunc("GET /api/v1/open-api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yml")
		if _, err := w.Write(openApi); err != nil {
			slog.ErrorContext(r.Context(), "failed to write open api", pkg.Err(err))
		}
	})

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
