package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/samuelfabel/protoql/internal/catalog"
)

// NowFunc supplies the reference date for derived age calculations.
var NowFunc = time.Now

func ageFromBirthDate(birthDate, ref time.Time) int {
	years := ref.Year() - birthDate.Year()
	if ref.Month() < birthDate.Month() ||
		(ref.Month() == birthDate.Month() && ref.Day() < birthDate.Day()) {
		years--
	}
	return years
}

func evalDerivedCustomer(expr string, c catalog.Customer) (any, error) {
	if strings.HasPrefix(expr, "concat(") && strings.HasSuffix(expr, ")") {
		return evalConcatCustomer(expr, c)
	}
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

func evalConcatCustomer(expr string, c catalog.Customer) (string, error) {
	args, err := parseConcatArgs(expr)
	if err != nil {
		return "", derivedEvalError(err.Error())
	}
	var b strings.Builder
	for _, arg := range args {
		if strings.HasPrefix(arg, `"`) {
			unquoted, err := strconv.Unquote(arg)
			if err != nil {
				return "", derivedEvalError("invalid string literal: " + arg)
			}
			b.WriteString(unquoted)
			continue
		}
		v, err := directString(c, arg)
		if err != nil {
			return "", err
		}
		b.WriteString(v)
	}
	return b.String(), nil
}

func parseConcatArgs(expr string) ([]string, error) {
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(expr, "concat("), ")"))
	if inner == "" {
		return nil, fmt.Errorf("concat requires at least one argument")
	}
	var args []string
	var cur strings.Builder
	inString := false
	for i := 0; i < len(inner); i++ {
		ch := inner[i]
		switch {
		case ch == '"':
			inString = !inString
			cur.WriteByte(ch)
		case ch == ',' && !inString:
			args = append(args, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(ch)
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated string in concat")
	}
	if tail := strings.TrimSpace(cur.String()); tail != "" {
		args = append(args, tail)
	}
	return args, nil
}
