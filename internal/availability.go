package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prathoss/hw/pkg"
)

type Availability struct {
	ID        uuid.UUID `json:"id"`
	LocalDate JSONTime  `json:"localDate"`
	Status    string    `json:"status"`
	// Vacancies represent number of vacancies that's available to book
	Vacancies int       `json:"vacancies"`
	Available bool      `json:"available"`
	ProductID uuid.UUID `json:"-"`
}

type AvailabilityDayRequest struct {
	ProductId uuid.UUID `json:"productId"`
	LocalDate JSONTime  `json:"localDate"`
}

type AvailabilityRangeRequest struct {
	ProductId      uuid.UUID `json:"productId"`
	LocalDateStart JSONTime  `json:"localDateStart"`
	LocalDateEnd   JSONTime  `json:"localDateEnd"`
}

const (
	AvailabilityStatusAvailable = "AVAILABLE"
	AvailabilityStatusSoldOut   = "SOLD_OUT"
)

type AvailabilityProcessor interface {
	InsertAvailabilities(ctx context.Context, availabilities []Availability) error
	GetAvailability(ctx context.Context, productID uuid.UUID, day time.Time) ([]Availability, error)
	GetAvailabilityTo(ctx context.Context, productID uuid.UUID, from time.Time, to time.Time) ([]Availability, error)
	GetAvailabilityByID(ctx context.Context, id uuid.UUID) (Availability, error)
	GetLatestAvailability(ctx context.Context, productID uuid.UUID) (*Availability, error)
}

var _ AvailabilityProcessor = &AvailabilityRepository{}

func NewAvailabilityRepository(pool *pgxpool.Pool) *AvailabilityRepository {
	return &AvailabilityRepository{
		db: pool,
	}
}

type AvailabilityRepository struct {
	db *pgxpool.Pool
}

const baseAvailabilityQuery = `SELECT a.id, a.product_id, a.date, p.capacity, (
		SELECT count(*)
		FROM ventrata.bookings b
		JOIN ventrata.tickets t ON b.id = t.booking_id
		WHERE b.availability_id = a.id
	) AS booked
FROM ventrata.availability a
JOIN ventrata.products p ON p.id = a.product_id`

func (a *AvailabilityRepository) GetLatestAvailability(ctx context.Context, productID uuid.UUID) (*Availability, error) {
	rows, err := a.db.Query(
		ctx,
		fmt.Sprintf(
			"%s WHERE a.product_id = $1 AND date = (SELECT max(date) FROM ventrata.availability WHERE product_id = $1 GROUP BY product_id)",
			baseAvailabilityQuery,
		),
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query latest availability: %w", err)
	}
	defer rows.Close()
	availabilities, err := a.scanAvailability(rows)
	if err != nil {
		return nil, fmt.Errorf("could not scan latest availability: %w", err)
	}
	if len(availabilities) == 0 {
		return nil, nil
	}
	return &availabilities[0], nil
}

func (a *AvailabilityRepository) InsertAvailabilities(ctx context.Context, availabilities []Availability) error {
	_, err := a.db.CopyFrom(
		ctx,
		pgx.Identifier{"ventrata", "availability"},
		[]string{"id", "product_id", "date"},
		pgx.CopyFromSlice(len(availabilities), func(i int) ([]interface{}, error) {
			return []any{availabilities[i].ID, availabilities[i].ProductID, time.Time(availabilities[i].LocalDate)}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("could not insert availability: %w", err)
	}
	return nil
}

func (a *AvailabilityRepository) GetAvailabilityByID(ctx context.Context, id uuid.UUID) (Availability, error) {
	rows, err := a.db.Query(
		ctx,
		fmt.Sprintf(
			"%s WHERE a.id = $1",
			baseAvailabilityQuery,
		),
		id,
	)
	if err != nil {
		return Availability{}, fmt.Errorf("querying availability by id failed: %w", err)
	}

	defer rows.Close()
	availabilities, err := a.scanAvailability(rows)
	if err != nil {
		return Availability{}, err
	}
	if len(availabilities) == 0 {
		return Availability{}, pkg.NewNotFoundError("availability was not found")
	}
	return availabilities[0], nil
}

func (a *AvailabilityRepository) GetAvailability(ctx context.Context, productID uuid.UUID, day time.Time) ([]Availability, error) {
	rows, err := a.db.Query(
		ctx,
		fmt.Sprintf(
			"%s WHERE a.product_id = $1 AND a.date = $2",
			baseAvailabilityQuery,
		),
		productID,
		day,
	)
	if err != nil {
		return nil, fmt.Errorf("querying availability failed: %w", err)
	}

	defer rows.Close()
	return a.scanAvailability(rows)
}

func (a *AvailabilityRepository) GetAvailabilityTo(ctx context.Context, productID uuid.UUID, from time.Time, to time.Time) ([]Availability, error) {
	rows, err := a.db.Query(
		ctx,
		fmt.Sprintf(
			"%s WHERE a.product_id = $1 AND $2 <= a.date AND a.date <= $3",
			baseAvailabilityQuery,
		),
		productID,
		from,
		to,
	)
	if err != nil {
		return nil, fmt.Errorf("querying availability to failed: %w", err)
	}

	defer rows.Close()
	return a.scanAvailability(rows)
}

func (a *AvailabilityRepository) scanAvailability(rows pgx.Rows) ([]Availability, error) {
	availabilities := make([]Availability, 0)
	for rows.Next() {
		var id uuid.UUID
		var productID uuid.UUID
		var date time.Time
		var capacity int
		var booked int
		if err := rows.Scan(&id, &productID, &date, &capacity, &booked); err != nil {
			return nil, fmt.Errorf("scanning availbility row failed: %w", err)
		}
		vacancies := capacity - booked
		a := Availability{
			ID:        id,
			ProductID: productID,
			LocalDate: JSONTime(date),
			Vacancies: vacancies,
		}
		if vacancies > 0 {
			a.Status = AvailabilityStatusAvailable
			a.Available = true
		} else {
			a.Status = AvailabilityStatusSoldOut
			a.Available = false
		}

		availabilities = append(availabilities, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("processing availability rows failed: %w", err)
	}
	return availabilities, nil
}
