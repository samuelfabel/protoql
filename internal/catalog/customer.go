package catalog

import "time"

// Customer is an in-memory source row for direct scalar projection.
type Customer struct {
	ID            string
	Name          string
	BirthDate     time.Time
	Email         string
	LoyaltyPoints int
	Active        bool
	CreditScore   float64
}
