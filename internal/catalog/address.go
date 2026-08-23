package catalog

// Address is a nested value on Customer.
type Address struct {
	Street       string
	Number       int
	Neighborhood string
	City         string
	State        string
	ZipCode      string
}

// Order is one row in a Customer orders collection.
type Order struct {
	ID string
}
