package internal

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Pricing struct {
	Price    int    `json:"price"`
	Currency string `json:"currency"`
}

type PricingStorer interface {
	GetPricingByProductId(ctx context.Context, productIds []uuid.UUID, currency string) (map[uuid.UUID]Pricing, error)
}

var _ PricingStorer = &PricingRepository{}

func NewPricingRepository(pool *pgxpool.Pool) *PricingRepository {
	return &PricingRepository{
		db: pool,
	}
}

type PricingRepository struct {
	db *pgxpool.Pool
}

func (p *PricingRepository) GetPricingByProductId(ctx context.Context, productIds []uuid.UUID, currency string) (map[uuid.UUID]Pricing, error) {
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
