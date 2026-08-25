package engine

import (
	"github.com/samuelfabel/protoql/internal/catalog"
	"github.com/samuelfabel/protoql/internal/compile"
)

// Record is one projected row in memory.
type Record map[string]any

// Project applies a ProjectionPlan to an in-memory slice of Customer.
func Project(plan *compile.ProjectionPlan, source []catalog.Customer) ([]Record, error) {
	if plan == nil {
		return nil, bindingUnsupported("plan is required")
	}

	out := make([]Record, 0, len(source))
	for i := range source {
		rec, err := projectBindings(plan.Bindings, source[i])
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func projectBindings(bindings []compile.FieldBinding, entity any) (Record, error) {
	rec := make(Record, len(bindings))
	for _, b := range bindings {
		v, err := projectBinding(entity, b)
		if err != nil {
			return nil, err
		}
		rec[b.Name] = v
	}
	return rec, nil
}

func projectBinding(entity any, b compile.FieldBinding) (any, error) {
	switch e := entity.(type) {
	case catalog.Customer:
		return projectCustomerBinding(e, b)
	case catalog.Address:
		return projectAddressBinding(e, b)
	case catalog.Order:
		return projectOrderBinding(e, b)
	case catalog.Product:
		return projectProductBinding(e, b)
	default:
		return nil, bindingUnsupported("unsupported entity type")
	}
}

func projectCustomerBinding(c catalog.Customer, b compile.FieldBinding) (any, error) {
	switch b.Kind {
	case compile.KindDirect:
		return directValue(c, b.Name, b.TypeName)
	case compile.KindDerived:
		return evalDerivedCustomer(b.Expr, c)
	case compile.KindAggregate:
		return evalAggregateCustomer(b.Aggregate, b.AggregateOf, c)
	case compile.KindPath:
		return projectCustomerPath(c, b)
	case compile.KindList:
		return projectCustomerList(c, b)
	default:
		return nil, bindingUnsupported("binding " + b.Name + " kind=" + string(b.Kind))
	}
}

func projectCustomerPath(c catalog.Customer, b compile.FieldBinding) (any, error) {
	if len(b.Children) > 0 {
		child, isNull, err := resolveCustomerNested(c, b.Name)
		if err != nil {
			return nil, err
		}
		if isNull {
			if b.Nullable {
				return nil, nil
			}
			return nil, nullViolation("field " + b.Name + " is non-null but source is null")
		}
		return projectBindings(b.Children, child)
	}
	if len(b.Path) > 0 {
		v, isNull, err := walkCustomerPath(c, b.Path)
		if err != nil {
			return nil, err
		}
		if isNull {
			if b.Nullable {
				return nil, nil
			}
			return nil, nullViolation("field " + b.Name + " is non-null but source is null")
		}
		return v, nil
	}
	return nil, bindingUnsupported("path binding " + b.Name + " has neither children nor path")
}

func projectCustomerList(c catalog.Customer, b compile.FieldBinding) (any, error) {
	items, err := resolveCustomerList(c, b.Name)
	if err != nil {
		return nil, err
	}
	if items == nil {
		if b.Nullable {
			return nil, nil
		}
		return nil, nullViolation("field " + b.Name + " is non-null but source is null")
	}
	out := make([]Record, 0, len(items))
	for i := range items {
		rec, err := projectBindings(b.Children, items[i])
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func resolveCustomerNested(c catalog.Customer, name string) (any, bool, error) {
	switch name {
	case "address":
		if c.Address == nil {
			return nil, true, nil
		}
		return *c.Address, false, nil
	default:
		return nil, false, bindingUnsupported("nested field " + name)
	}
}

func resolveCustomerList(c catalog.Customer, name string) ([]catalog.Order, error) {
	switch name {
	case "orders":
		return c.Orders, nil
	default:
		return nil, bindingUnsupported("list field " + name)
	}
}

func walkCustomerPath(c catalog.Customer, path []string) (any, bool, error) {
	if len(path) == 0 {
		return nil, false, bindingUnsupported("empty path")
	}
	switch path[0] {
	case "address":
		if c.Address == nil {
			return nil, true, nil
		}
		return walkAddressPath(*c.Address, path[1:])
	default:
		return nil, false, bindingUnsupported("path segment " + path[0])
	}
}

func walkAddressPath(a catalog.Address, path []string) (any, bool, error) {
	if len(path) == 0 {
		return a, false, nil
	}
	if len(path) != 1 {
		return nil, false, bindingUnsupported("unsupported address path " + path[0])
	}
	switch path[0] {
	case "street":
		return a.Street, false, nil
	case "number":
		return a.Number, false, nil
	case "neighborhood":
		return a.Neighborhood, false, nil
	case "city":
		return a.City, false, nil
	case "state":
		return a.State, false, nil
	case "zipCode":
		return a.ZipCode, false, nil
	default:
		return nil, false, bindingUnsupported("address field " + path[0])
	}
}

func projectAddressBinding(a catalog.Address, b compile.FieldBinding) (any, error) {
	switch b.Kind {
	case compile.KindDirect:
		return directAddressValue(a, b.Name, b.TypeName)
	default:
		return nil, bindingUnsupported("address binding " + b.Name + " kind=" + string(b.Kind))
	}
}

func directAddressValue(a catalog.Address, graphqlName, typeName string) (any, error) {
	switch typeName {
	case "String", "ID":
		switch graphqlName {
		case "street":
			return a.Street, nil
		case "neighborhood":
			return a.Neighborhood, nil
		case "city":
			return a.City, nil
		case "state":
			return a.State, nil
		case "zipCode":
			return a.ZipCode, nil
		default:
			return "", bindingUnsupported("direct field " + graphqlName)
		}
	case "Int":
		switch graphqlName {
		case "number":
			return a.Number, nil
		default:
			return 0, bindingUnsupported("direct field " + graphqlName)
		}
	default:
		return nil, typeUnsupported("type " + typeName + " for field " + graphqlName)
	}
}

func projectOrderBinding(o catalog.Order, b compile.FieldBinding) (any, error) {
	switch b.Kind {
	case compile.KindDirect:
		return directOrderValue(o, b.Name, b.TypeName)
	default:
		return nil, bindingUnsupported("order binding " + b.Name + " kind=" + string(b.Kind))
	}
}

func directOrderValue(o catalog.Order, graphqlName, typeName string) (any, error) {
	switch typeName {
	case "String", "ID":
		switch graphqlName {
		case "id":
			return o.ID, nil
		default:
			return "", bindingUnsupported("direct field " + graphqlName)
		}
	case "Float", "Decimal":
		switch graphqlName {
		case "total":
			return o.Total, nil
		default:
			return 0.0, bindingUnsupported("direct field " + graphqlName)
		}
	default:
		return nil, typeUnsupported("type " + typeName + " for field " + graphqlName)
	}
}

// ProjectProducts applies a ProjectionPlan to an in-memory slice of Product.
func ProjectProducts(plan *compile.ProjectionPlan, source []catalog.Product) ([]Record, error) {
	if plan == nil {
		return nil, bindingUnsupported("plan is required")
	}

	out := make([]Record, 0, len(source))
	for i := range source {
		rec, err := projectBindings(plan.Bindings, source[i])
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func projectProductBinding(p catalog.Product, b compile.FieldBinding) (any, error) {
	switch b.Kind {
	case compile.KindDirect:
		return directProductValue(p, b.Name, b.TypeName)
	case compile.KindDerived:
		return evalDerivedProduct(b.Expr, p)
	case compile.KindAggregate:
		return evalAggregateProduct(b.Aggregate, b.AggregateOf, p)
	default:
		return nil, bindingUnsupported("binding " + b.Name + " kind=" + string(b.Kind))
	}
}

func directProductValue(p catalog.Product, graphqlName, typeName string) (any, error) {
	switch typeName {
	case "String", "ID":
		switch graphqlName {
		case "id":
			return p.ID, nil
		case "name":
			return p.Name, nil
		default:
			return "", bindingUnsupported("direct field " + graphqlName)
		}
	case "Int":
		switch graphqlName {
		case "stock":
			return p.Stock, nil
		default:
			return 0, bindingUnsupported("direct field " + graphqlName)
		}
	default:
		return nil, typeUnsupported("type " + typeName + " for field " + graphqlName)
	}
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
