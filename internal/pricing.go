package internal

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prathoss/hw/pkg"
)

type Pricing struct {
	Price    int    `json:"price"`
	Currency string `json:"currency"`
}

type PricedProduct struct {
	Product
	Pricing
}

type PricedAvailability struct {
	Availability
	Pricing
}

type PricedUnit struct {
	Unit
	Pricing
}

type PricedBooking struct {
	Units []PricedUnit `json:"units"`
	Booking
	Pricing
}

type PricingProcessor interface {
	GetPricedProducts(ctx context.Context, products []Product, currency string) ([]PricedProduct, error)
	GetPricedAvailabilities(ctx context.Context, availabilities []Availability, currency string) ([]PricedAvailability, error)
	GetPricedBookings(ctx context.Context, bookings []Booking, currency string) ([]PricedBooking, error)
}

var _ PricingProcessor = &PricingRepository{}

func NewPricingRepository(pool *pgxpool.Pool) *PricingRepository {
	return &PricingRepository{
		db: pool,
	}
}

type PricingRepository struct {
	db *pgxpool.Pool
}

func (p *PricingRepository) GetPricedProducts(ctx context.Context, products []Product, currency string) ([]PricedProduct, error) {
	productIDs := make([]uuid.UUID, 0, len(products))
	for _, product := range products {
		productIDs = append(productIDs, product.ID)
	}
	pricing, err := p.getPricingByProductId(ctx, productIDs, currency)
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

func (p *PricingRepository) GetPricedAvailabilities(ctx context.Context, availabilities []Availability, currency string) ([]PricedAvailability, error) {
	productIDsMap := map[uuid.UUID]struct{}{}
	for _, availability := range availabilities {
		productIDsMap[availability.ProductID] = struct{}{}
	}
	productIDs := make([]uuid.UUID, 0, len(productIDsMap))
	for productID := range productIDsMap {
		productIDs = append(productIDs, productID)
	}
	pricing, err := p.getPricingByProductId(ctx, productIDs, currency)
	if err != nil {
		return nil, err
	}
	pricedAvailabilities := make([]PricedAvailability, 0, len(availabilities))
	for _, availability := range availabilities {
		pricing, ok := pricing[availability.ProductID]
		if !ok {
			return nil, pkg.NewNotFoundError(fmt.Sprintf("could not find pricing for availability %s", availability.ID))
		}
		pricedAvailabilities = append(pricedAvailabilities, PricedAvailability{
			Availability: availability,
			Pricing:      pricing,
		})
	}
	return pricedAvailabilities, nil
}

func (p *PricingRepository) GetPricedBookings(ctx context.Context, bookings []Booking, currency string) ([]PricedBooking, error) {
	productIdsMap := map[uuid.UUID]struct{}{}
	for _, booking := range bookings {
		productIdsMap[booking.ProductID] = struct{}{}
	}
	productIDs := make([]uuid.UUID, 0, len(productIdsMap))
	for productID := range productIdsMap {
		productIDs = append(productIDs, productID)
	}

	pricing, err := p.getPricingByProductId(ctx, productIDs, currency)
	if err != nil {
		return nil, err
	}

	pricedBookings := make([]PricedBooking, 0, len(bookings))
	for _, booking := range bookings {
		pricing, ok := pricing[booking.ProductID]
		if !ok {
			return nil, pkg.NewNotFoundError(fmt.Sprintf("could not find pricing for booking %s", booking.ID))
		}

		pricedUnits := make([]PricedUnit, 0, len(booking.Units))
		for _, unit := range booking.Units {
			pricedUnits = append(pricedUnits, PricedUnit{
				Unit:    unit,
				Pricing: pricing,
			})
		}

		pricedBookings = append(pricedBookings, PricedBooking{
			Units:   pricedUnits,
			Booking: booking,
			Pricing: Pricing{
				Price:    pricing.Price * len(pricedUnits),
				Currency: pricing.Currency,
			},
		})
	}

	return pricedBookings, nil
}

func (p *PricingRepository) getPricingByProductId(ctx context.Context, productIds []uuid.UUID, currency string) (map[uuid.UUID]Pricing, error) {
	rows, err := p.db.Query(
		ctx,
		"SELECT product_id, price, currency FROM ventrata.pricing WHERE product_id = ANY($1) AND currency = $2",
		productIds,
		currency,
	)
	if err != nil {
		return nil, fmt.Errorf("querying pricing by product ids failed: %w", err)
	}
	defer rows.Close()

	pricing := map[uuid.UUID]Pricing{}
	for rows.Next() {
		var productId uuid.UUID
		var p Pricing
		err := rows.Scan(&productId, &p.Price, &p.Currency)
		if err != nil {
			return nil, err
		}
		pricing[productId] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return pricing, nil
}
