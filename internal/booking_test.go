package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestServer_createBooking_Concurrency(t *testing.T) {
	pgConn, cleanup, err := setupPgAndMigrations()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(cleanup)
	s, err := NewServer(Config{
		DatabaseDSN: pgConn,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, pgConn)
	if err != nil {
		t.Fatal(err)
	}
	productID := uuid.New()
	availabilityID := uuid.New()
	date := time.Now().UTC().Truncate(time.Hour * 24)

	_, err = pool.Exec(ctx, "INSERT INTO ventrata.products(id, name, capacity) VALUES ($1, 'product', 10)", productID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, "INSERT INTO ventrata.availability(id, product_id, date) VALUES ($1, $2, $3)", availabilityID, productID, date)
	if err != nil {
		t.Fatal(err)
	}

	concurrencyDegree := 200
	wg := sync.WaitGroup{}
	for range concurrencyDegree {
		w := httptest.NewRecorder()
		buff := &bytes.Buffer{}
		br := BookingRequest{
			ProductID:      productID,
			AvailabilityID: availabilityID,
			Units:          1,
		}
		if err := json.NewEncoder(buff).Encode(br); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", buff)
		wg.Add(1)
		go func() {
			_, _ = s.createBooking(w, req)
			wg.Done()
		}()
	}
	wg.Wait()
	availability, err := s.availabilityProcessor.GetAvailabilityByID(ctx, availabilityID)
	if err != nil {
		t.Fatal(err)
	}
	if availability.Vacancies < 0 {
		t.Fatalf("created more bookings than availabile, resulting vacancies: %d", availability.Vacancies)
	}
}
