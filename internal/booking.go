package internal

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prathoss/hw/pkg"
)

type Unit struct {
	ID     uuid.UUID `json:"id"`
	Ticket *string   `json:"ticket"`
}

type Booking struct {
	ID             uuid.UUID `json:"id"`
	Status         string    `json:"status"`
	ProductID      uuid.UUID `json:"productId"`
	AvailabilityID uuid.UUID `json:"availabilityId"`
	Units          []Unit    `json:"units"`
}

const (
	BookingStatusReserved  = "RESERVED"
	BookingStatusConfirmed = "CONFIRMED"
)

type BookingRequest struct {
	ProductID      uuid.UUID `json:"productId"`
	AvailabilityID uuid.UUID `json:"availabilityId"`
	Units          int       `json:"units"`
}

type Ticket struct {
	ID        uuid.UUID
	BookingID uuid.UUID
	Content   string
}

type BookingProcessor interface {
	CreateBooking(ctx context.Context, availability Availability, units int) (Booking, error)
	GetBooking(ctx context.Context, bookingID uuid.UUID) (Booking, error)
	ConfirmBooking(ctx context.Context, bookingID uuid.UUID) (Booking, error)
}

var _ BookingProcessor = &BookingRepository{}

func NewBookingRepository(pool *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{
		db: pool,
	}
}

type BookingRepository struct {
	db *pgxpool.Pool
}

func (b *BookingRepository) CreateBooking(ctx context.Context, availability Availability, units int) (Booking, error) {
	// this may want concurrency protection to avoid booking over capacity
	if availability.Vacancies < units {
		return Booking{}, pkg.NewBadRequestError(pkg.InvalidParam{
			Name:   "units",
			Reason: "units is greater than availability vacancies",
		})
	}

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return Booking{}, fmt.Errorf("begin booking creation transaction failed: %w", err)
	}

	commitedTx := false
	defer func() {
		if commitedTx {
			return
		}
		if err := tx.Rollback(ctx); err != nil {
			slog.ErrorContext(ctx, "rolling back booking creation transaction failed", pkg.Err(err))
			panic(err)
		}
	}()

	bookingID := uuid.New()
	tickets := make([]Ticket, 0, units)
	for range units {
		tickets = append(tickets, Ticket{
			ID:        uuid.New(),
			BookingID: bookingID,
			// ticket content will be available after booking confirmation
			Content: "",
		})
	}
	_, err = tx.Exec(
		ctx,
		"INSERT INTO ventrata.bookings (id, availability_id, confirmed) VALUES ($1, $2, FALSE)",
		bookingID,
		availability.ID,
	)
	if err != nil {
		return Booking{}, fmt.Errorf("insert booking failed: %w", err)
	}
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"ventrata", "tickets"},
		[]string{"id", "booking_id", "content"},
		pgx.CopyFromSlice(len(tickets), func(i int) ([]any, error) {
			ticket := tickets[i]
			return []any{ticket.ID, ticket.BookingID, ticket.Content}, nil
		}),
	)
	if err != nil {
		return Booking{}, fmt.Errorf("insert booking tickets failed: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, fmt.Errorf("commit booking creation transaction failed: %w", err)
	}
	commitedTx = true

	return b.GetBooking(ctx, bookingID)
}

func (b *BookingRepository) GetBooking(ctx context.Context, bookingID uuid.UUID) (Booking, error) {
	rows, err := b.db.Query(
		ctx,
		`SELECT b.id, b.availability_id, b.confirmed, t.id AS ticket_id, t.content AS ticket_content, a.product_id
FROM ventrata.bookings b
JOIN ventrata.tickets t ON b.id = t.booking_id
JOIN ventrata.availability a ON a.id = b.availability_id
WHERE b.id = $1`,
		bookingID,
	)
	if err != nil {
		return Booking{}, fmt.Errorf("querying booking failed: %w", err)
	}

	defer rows.Close()
	bookings, err := b.scanBookings(rows)
	if err != nil {
		return Booking{}, err
	}
	if len(bookings) == 0 {
		return Booking{}, pkg.NewNotFoundError(fmt.Sprintf("booking %s not found", bookingID))
	}
	return bookings[0], nil
}

func (b *BookingRepository) ConfirmBooking(ctx context.Context, bookingID uuid.UUID) (Booking, error) {
	booking, err := b.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if booking.Status == BookingStatusConfirmed {
		return Booking{}, pkg.NewBadRequestError(pkg.InvalidParam{
			Name:   "bookingId",
			Reason: "booking already confirmed",
		})
	}
	tx, err := b.db.Begin(ctx)
	if err != nil {
		return Booking{}, fmt.Errorf("begin booking confirmation transaction failed: %w", err)
	}

	commitedTx := false
	defer func() {
		if commitedTx {
			return
		}
		if err := tx.Rollback(ctx); err != nil {
			slog.ErrorContext(ctx, "rolling back booking confirmation transaction failed", pkg.Err(err))
		}
	}()

	_, err = tx.Exec(ctx, "UPDATE ventrata.bookings SET confirmed = TRUE WHERE id = $1", bookingID)
	if err != nil {
		return Booking{}, fmt.Errorf("updating booking status failed: %w", err)
	}
	_, err = tx.Exec(ctx, "UPDATE ventrata.tickets SET content = 'my awesome ticket' WHERE booking_id = $1", bookingID)
	if err != nil {
		return Booking{}, fmt.Errorf("updating booking tickets failed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Booking{}, fmt.Errorf("commit booking confirmation transaction failed: %w", err)
	}
	commitedTx = true

	booking, err = b.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	return booking, err
}

func (b *BookingRepository) scanBookings(rows pgx.Rows) ([]Booking, error) {
	bookingsMap := make(map[uuid.UUID]*Booking)
	for rows.Next() {
		var id uuid.UUID
		var availabilityID uuid.UUID
		var confirmed bool
		var ticketID uuid.UUID
		var ticketContent string
		var productID uuid.UUID
		if err := rows.Scan(&id, &availabilityID, &confirmed, &ticketID, &ticketContent, &productID); err != nil {
			return nil, fmt.Errorf("scanning bookings failed: %w", err)
		}

		var nullableTicketContent *string
		if confirmed {
			nullableTicketContent = &ticketContent
		}
		if booking, ok := bookingsMap[id]; ok {
			booking.Units = append(booking.Units, Unit{
				ID:     ticketID,
				Ticket: nullableTicketContent,
			})
		} else {
			status := BookingStatusReserved
			if confirmed {
				status = BookingStatusConfirmed
			}
			booking := Booking{
				ID:             id,
				ProductID:      productID,
				AvailabilityID: availabilityID,
				Status:         status,
				Units: []Unit{
					{
						ID:     ticketID,
						Ticket: nullableTicketContent,
					},
				},
			}
			bookingsMap[id] = &booking
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("proccessing booking rows failed: %w", err)
	}
	bookings := make([]Booking, 0, len(bookingsMap))
	for _, booking := range bookingsMap {
		bookings = append(bookings, *booking)
	}
	return bookings, nil
}
