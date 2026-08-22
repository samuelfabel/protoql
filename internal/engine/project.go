package engine

import (
	"github.com/samuelfabel/protoql/internal/catalog"
	"github.com/samuelfabel/protoql/internal/compile"
)

// Record is one projected row in memory.
type Record map[string]any

// Project applies a ProjectionPlan to an in-memory slice of Customer.
// Direct bindings for String, ID, Int, Boolean, and Float are materialized.
func Project(plan *compile.ProjectionPlan, source []catalog.Customer) ([]Record, error) {
	if plan == nil {
		return nil, bindingUnsupported("plan is required")
	}

	out := make([]Record, 0, len(source))
	for i := range source {
		rec, err := projectCustomer(plan.Bindings, source[i])
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func projectCustomer(bindings []compile.FieldBinding, c catalog.Customer) (Record, error) {
	rec := make(Record, len(bindings))
	for _, b := range bindings {
		if b.Kind != compile.KindDirect {
			return nil, bindingUnsupported("binding " + b.Name + " kind=" + string(b.Kind))
		}
		v, err := directValue(c, b.Name, b.TypeName)
		if err != nil {
			return nil, err
		}
		rec[b.Name] = v
	}
	return rec, nil
}

func directValue(c catalog.Customer, graphqlName, typeName string) (any, error) {
	switch typeName {
	case "String", "ID":
		return directString(c, graphqlName)
	case "Int":
		return directInt(c, graphqlName)
	case "Boolean":
		return directBool(c, graphqlName)
	case "Float":
		return directFloat(c, graphqlName)
	default:
		return nil, typeUnsupported("type " + typeName + " for field " + graphqlName)
	}
}

func directString(c catalog.Customer, graphqlName string) (string, error) {
	switch graphqlName {
	case "id":
		return c.ID, nil
	case "name":
		return c.Name, nil
	case "email":
		return c.Email, nil
	default:
		return "", bindingUnsupported("direct field " + graphqlName)
	}
}

func directInt(c catalog.Customer, graphqlName string) (int, error) {
	switch graphqlName {
	case "loyaltyPoints":
		return c.LoyaltyPoints, nil
	default:
		return 0, bindingUnsupported("direct field " + graphqlName)
	}
}

func directBool(c catalog.Customer, graphqlName string) (bool, error) {
	switch graphqlName {
	case "active":
		return c.Active, nil
	default:
		return false, bindingUnsupported("direct field " + graphqlName)
	}
}

func directFloat(c catalog.Customer, graphqlName string) (float64, error) {
	switch graphqlName {
	case "creditScore":
		return c.CreditScore, nil
	default:
		return 0, bindingUnsupported("direct field " + graphqlName)
	}
}
