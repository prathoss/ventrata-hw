package internal

import "time"

type Availability struct {
	ID        string    `json:"id"`
	LocalDate time.Time `json:"localDate"`
	Status    string    `json:"status"`
	// Vacancies represent number of vacancies that's available to book
	Vacancies int  `json:"vacancies"`
	Available bool `json:"available"`
}

const (
	StatusAvailable = "AVAILABLE"
	StatusSoldOut   = "SOLD_OUT"
)
