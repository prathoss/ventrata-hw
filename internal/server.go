package internal

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prathoss/hw/pkg"
	"github.com/robfig/cron/v3"
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
		db:                pool,
		config:            config,
		productStore:      NewProductRepository(pool),
		pricingStore:      NewPricingRepository(pool),
		availabilityStore: NewAvailabilityRepository(pool),
		bookingStore:      NewBookingRepository(pool),
	}, nil
}

type Server struct {
	db                *pgxpool.Pool
	config            Config
	productStore      ProductStorer
	pricingStore      PricingStorer
	availabilityStore AvailabilityStorer
	bookingStore      BookingStorer
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
		return s.pricingStore.GetPricedProducts(r.Context(), products, getCurrency())
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
		pricedProducts, err := s.pricingStore.GetPricedProducts(r.Context(), []Product{product}, getCurrency())
		if err != nil {
			return nil, err
		}
		return pricedProducts[0], nil
	}
	return product, nil
}

func (s *Server) listAvailability(_ http.ResponseWriter, r *http.Request) (any, error) {
	capability := getCapabilityHeader(r)
	invalidParams := validateCapability(capability)
	if len(invalidParams) > 0 {
		return nil, pkg.NewBadRequestError(invalidParams...)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var rawMessage map[string]any
	if err := json.Unmarshal(body, &rawMessage); err != nil {
		return nil, pkg.NewBadRequestError(pkg.InvalidParam{
			Name:   "Body",
			Reason: err.Error(),
		})
	}
	_, isRange := rawMessage["localDateStart"]

	var availabilities []Availability
	if isRange {
		var request AvailabilityRangeRequest
		if err := json.Unmarshal(body, &request); err != nil {
			return nil, pkg.NewBadRequestError(pkg.InvalidParam{
				Name:   "Body",
				Reason: err.Error(),
			})
		}
		availabilities, err = s.availabilityStore.GetAvailabilityTo(r.Context(), request.ProductId, time.Time(request.LocalDateStart).UTC(), time.Time(request.LocalDateEnd).UTC())
		if err != nil {
			return nil, err
		}
	} else {
		var request AvailabilityDayRequest
		if err := json.Unmarshal(body, &request); err != nil {
			return nil, pkg.NewBadRequestError(pkg.InvalidParam{
				Name:   "Body",
				Reason: err.Error(),
			})
		}
		availabilities, err = s.availabilityStore.GetAvailability(r.Context(), request.ProductId, time.Time(request.LocalDate).UTC())
		if err != nil {
			return nil, err
		}
	}

	if capability == CapabilityPricing {
		return s.pricingStore.GetPricedAvailabilities(r.Context(), availabilities, getCurrency())
	}

	return availabilities, nil
}

func (s *Server) createBooking(_ http.ResponseWriter, r *http.Request) (any, error) {
	invalidParams := make([]pkg.InvalidParam, 0, 10)

	var bookingRequest BookingRequest
	if err := json.NewDecoder(r.Body).Decode(&bookingRequest); err != nil {
		return nil, pkg.NewBadRequestError(pkg.InvalidParam{
			Name:   "Body",
			Reason: err.Error(),
		})
	}
	if bookingRequest.Units <= 0 {
		invalidParams = append(invalidParams, pkg.InvalidParam{
			Name:   "units",
			Reason: "Must be greater than zero",
		})
	}
	if len(invalidParams) > 0 {
		return nil, pkg.NewBadRequestError(invalidParams...)
	}

	availability, err := s.availabilityStore.GetAvailabilityByID(r.Context(), bookingRequest.AvailabilityID)
	if err != nil {
		return nil, err
	}

	if availability.ProductID != bookingRequest.ProductID {
		invalidParams = append(invalidParams, pkg.InvalidParam{
			Name:   "productID",
			Reason: "product availability mismatch",
		})
	}
	if len(invalidParams) > 0 {
		return nil, pkg.NewBadRequestError(invalidParams...)
	}

	return s.bookingStore.CreateBooking(r.Context(), availability, bookingRequest.Units)
}

func (s *Server) getBookingDetail(_ http.ResponseWriter, r *http.Request) (any, error) {
	invalidParams := make([]pkg.InvalidParam, 0, 10)

	capability := getCapabilityHeader(r)
	validationErrors := validateCapability(capability)
	invalidParams = append(invalidParams, validationErrors...)

	idStr := r.PathValue("id")
	id, validationErrors := validateID(idStr)
	invalidParams = append(invalidParams, validationErrors...)

	if len(invalidParams) > 0 {
		return nil, pkg.NewBadRequestError(invalidParams...)
	}

	booking, err := s.bookingStore.GetBooking(r.Context(), id)
	if err != nil {
		return nil, err
	}

	if capability == CapabilityPricing {
		bookings, err := s.pricingStore.GetPricedBookings(r.Context(), []Booking{booking}, getCurrency())
		if err != nil {
			return nil, err
		}
		return bookings[0], nil
	}

	return booking, nil
}

func (s *Server) confirmBooking(_ http.ResponseWriter, r *http.Request) (any, error) {
	idStr := r.PathValue("id")
	id, validationErrors := validateID(idStr)
	if len(validationErrors) > 0 {
		return nil, pkg.NewBadRequestError(validationErrors...)
	}

	return s.bookingStore.ConfirmBooking(r.Context(), id)
}

func getCurrency() string {
	return "EUR"
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

	mux.HandleFunc("POST /dev/v1/availability", func(w http.ResponseWriter, r *http.Request) {
		s.CreateAvailabilities()
		w.WriteHeader(http.StatusCreated)
	})

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

	// everyday at midnight create additional availabilities so that at least a year of availabilities data is ready
	c := cron.New()
	_, err := c.AddFunc("@daily", s.CreateAvailabilities)
	if err != nil {
		return err
	}
	c.Start()

	return pkg.ServeWithShutdown(server)
}

func (s *Server) CreateAvailabilities() {
	ctx, cFunc := context.WithTimeout(context.Background(), 10*time.Second)
	defer cFunc()
	products, err := s.productStore.ListProducts(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list products", pkg.Err(err))
		return
	}
	// yes, n+1 but ok for this use case
	for _, product := range products {
		latestAvailability, err := s.availabilityStore.GetLatestAvailability(ctx, product.ID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get latest availability", pkg.Err(err))
			return
		}
		endDate := time.Now().UTC().AddDate(1, 0, 0).Truncate(24 * time.Hour)
		startDate := time.Now().AddDate(0, 0, -1).UTC().Truncate(24 * time.Hour)
		if latestAvailability != nil {
			startDate = time.Time(latestAvailability.LocalDate)
		}
		daysDiff := int(endDate.Sub(startDate).Hours() / 24)
		availabilities := make([]Availability, 0, daysDiff)
		for i := range daysDiff {
			availabilities = append(availabilities, Availability{
				ID:        uuid.New(),
				ProductID: product.ID,
				LocalDate: JSONTime(startDate.AddDate(0, 0, i+1)),
			})
		}
		if err := s.availabilityStore.InsertAvailabilities(ctx, availabilities); err != nil {
			slog.ErrorContext(ctx, "failed to insert availabilities", pkg.Err(err))
			return
		}
	}
}
