package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/samuelfabel/protoql/internal/catalog"
)

// NowFunc supplies the reference date for derived age calculations.
// Tests may override it for deterministic fixtures.
var NowFunc = time.Now

// ageFromBirthDate returns full years between birthDate and ref.
// Rule: age = referenceDate - birthDate (calendar years, birthday not yet reached subtracts one).
func ageFromBirthDate(birthDate, ref time.Time) int {
	years := ref.Year() - birthDate.Year()
	if ref.Month() < birthDate.Month() ||
		(ref.Month() == birthDate.Month() && ref.Day() < birthDate.Day()) {
		years--
	}
	return years
}

func evalDerivedCustomer(expr string, c catalog.Customer) (any, error) {
	if strings.HasPrefix(expr, "age(") && strings.HasSuffix(expr, ")") {
		field := strings.TrimSuffix(strings.TrimPrefix(expr, "age("), ")")
		if field != "birthDate" {
			return nil, derivedEvalError(fmt.Sprintf("age() only supports birthDate, got %q", field))
		}
		return ageFromBirthDate(c.BirthDate, NowFunc()), nil
	}
	return nil, derivedEvalError("unsupported expression: " + expr)
}

func evalDerivedProduct(expr string, p catalog.Product) (any, error) {
	parts := strings.Split(strings.TrimSpace(expr), " ")
	if len(parts) == 3 && parts[0] == "stock" && parts[1] == ">" {
		threshold, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, derivedEvalError("invalid comparison literal: " + parts[2])
		}
		return p.Stock > threshold, nil
	}
	return nil, derivedEvalError("unsupported expression: " + expr)
}
