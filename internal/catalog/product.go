package catalog

// Product is an in-memory source row for product projection.
type Product struct {
	ID      string
	Name    string
	Stock   int
	Reviews []Review
}
