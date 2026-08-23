package engine

import (
	"fmt"

	"github.com/samuelfabel/protoql/internal/catalog"
)

func evalAggregateCustomer(fn, of string, c catalog.Customer) (any, error) {
	switch fn {
	case "count":
		return aggregateCount(of, len(c.Orders))
	case "sum":
		return aggregateSumOrders(of, c.Orders)
	default:
		return nil, bindingUnsupported("aggregate fn " + fn)
	}
}

func evalAggregateProduct(fn, of string, p catalog.Product) (any, error) {
	switch fn {
	case "avg":
		return aggregateAvgReviews(of, p.Reviews)
	default:
		return nil, bindingUnsupported("aggregate fn " + fn)
	}
}

func aggregateCount(of string, n int) (int, error) {
	if of != "orders" {
		return 0, bindingUnsupported("count of " + of)
	}
	return n, nil
}

func aggregateSumOrders(of string, orders []catalog.Order) (float64, error) {
	if of != "orders.total" {
		return 0, bindingUnsupported("sum of " + of)
	}
	var sum float64
	for _, o := range orders {
		sum += o.Total
	}
	return sum, nil
}

func aggregateAvgReviews(of string, reviews []catalog.Review) (float64, error) {
	if of != "reviews.rating" {
		return 0, bindingUnsupported("avg of " + of)
	}
	if len(reviews) == 0 {
		return 0, aggregateEmpty(fmt.Sprintf("avg(%s) on empty collection", of))
	}
	var sum float64
	for _, r := range reviews {
		sum += r.Rating
	}
	return sum / float64(len(reviews)), nil
}
