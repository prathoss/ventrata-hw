package internal

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductRepository_GetProduct(t *testing.T) {
	pgConn, cleanup, err := setupPgAndMigrations()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, pgConn)
	if err != nil {
		t.Fatal(err)
	}
	productRepository := NewProductRepository(pool)
	products, err := productRepository.ListProducts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) > 0 {
		t.Fatalf("expected number of products returned to be 0, but got %d", len(products))
	}
}
