package engine

import (
	"github.com/samuelfabel/protoql/internal/catalog"
	"github.com/samuelfabel/protoql/internal/compile"
)

// Record is one projected row in memory.
type Record map[string]any

// Project applies a ProjectionPlan to an in-memory slice of Customer.
// Only KindDirect String/ID fields are materialized in this milestone.
func Project(plan *compile.ProjectionPlan, source []catalog.Customer) ([]Record, error) {
	if plan == nil {
		return nil, unsupported("plan is required")
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
			return nil, unsupported("binding " + b.Name + " kind=" + string(b.Kind))
		}
		if b.TypeName != "String" && b.TypeName != "ID" {
			return nil, unsupported("binding " + b.Name + " type=" + b.TypeName)
		}
		v, err := directString(c, b.Name)
		if err != nil {
			return nil, err
		}
		rec[b.Name] = v
	}
	return rec, nil
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
		return "", unsupported("direct field " + graphqlName)
	}
}
