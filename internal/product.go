package internal

type Product struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Capacity represents max number of vacancies per 1 day (availability)
	Capacity int `json:"capacity"`
}
