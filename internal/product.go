package internal

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Product struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	// Capacity represents max number of vacancies per 1 day (availability)
	Capacity int `json:"capacity"`
}

type ProductStorer interface {
	GetProduct(ctx context.Context, id uuid.UUID) (Product, error)
	ListProducts(ctx context.Context) ([]Product, error)
}

var _ ProductStorer = &ProductRepository{}

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{
		db: pool,
	}
}

type ProductRepository struct {
	db *pgxpool.Pool
}

func (p *ProductRepository) GetProduct(ctx context.Context, id uuid.UUID) (Product, error) {
	rows, err := p.db.Query(ctx, "SELECT id, name, capacity FROM ventrata.products WHERE id = $1", id)
	if err != nil {
		return Product{}, fmt.Errorf("querying product by id failed: %w", err)
	}
	defer rows.Close()

	product, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Product])
	if err != nil {
		return Product{}, fmt.Errorf("scanning product row failed: %w", err)
	}

	return product, nil
}

func (p *ProductRepository) ListProducts(ctx context.Context) ([]Product, error) {
	rows, err := p.db.Query(ctx, "SELECT id, name, capacity FROM ventrata.products")
	if err != nil {
		return nil, fmt.Errorf("querying products failed: %w", err)
	}
	defer rows.Close()

	product, err := pgx.CollectRows(rows, pgx.RowToStructByName[Product])
	if err != nil {
		return nil, fmt.Errorf("scanning product rows failed: %w", err)
	}

	return product, nil
}
