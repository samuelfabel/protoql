package catalog

// Customer is an in-memory source row for projection.
type Customer struct {
	ID            string
	Name          string
	Email         string
	LoyaltyPoints int
	Active        bool
	CreditScore   float64
	Address       *Address
	Orders        []Order
}
