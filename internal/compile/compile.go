package compile

import (
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Compile turns a GraphQL document and schema SDL into a ProjectionPlan.
func Compile(schemaSDL, query string) (*ProjectionPlan, error) {
	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "schema.graphql", Input: schemaSDL})
	if err != nil {
		return nil, &Error{Code: CodeQueryInvalid, Message: err.Error()}
	}

	doc, errs := gqlparser.LoadQuery(schema, query)
	if len(errs) > 0 {
		return nil, mapGQLErrors(errs)
	}

	op, cerr := selectedOperation(doc)
	if cerr != nil {
		return nil, cerr
	}

	if len(op.SelectionSet) != 1 {
		return nil, &Error{Code: CodeQueryInvalid, Message: "query must request exactly one root field"}
	}

	field, ok := op.SelectionSet[0].(*ast.Field)
	if !ok {
		return nil, &Error{Code: CodeQueryInvalid, Message: "root fragments are not supported in this milestone"}
	}

	bindings, cerr := bindSelectionSet(field.SelectionSet)
	if cerr != nil {
		return nil, cerr
	}

	return &ProjectionPlan{RootField: field.Name, Bindings: bindings}, nil
}

func selectedOperation(doc *ast.QueryDocument) (*ast.OperationDefinition, error) {
	if len(doc.Operations) == 0 {
		return nil, &Error{Code: CodeQueryInvalid, Message: "document has no operation"}
	}
	if len(doc.Operations) > 1 {
		return nil, &Error{Code: CodeQueryInvalid, Message: "only one operation per document is supported in this milestone"}
	}
	op := doc.Operations[0]
	if op.Operation != ast.Query {
		return nil, &Error{Code: CodeQueryInvalid, Message: "only query operations are supported in this milestone"}
	}
	return op, nil
}

func bindSelectionSet(set ast.SelectionSet) ([]FieldBinding, error) {
	out := make([]FieldBinding, 0, len(set))
	index := 1
	for _, sel := range set {
		field, ok := sel.(*ast.Field)
		if !ok {
			return nil, &Error{Code: CodeQueryInvalid, Message: "only fields are supported (no fragments) in this milestone"}
		}
		b, err := bindField(field, index)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
		index++
	}
	return out, nil
}

func bindField(field *ast.Field, index int) (FieldBinding, error) {
	def := field.Definition
	if def == nil {
		return FieldBinding{}, &Error{Code: CodeQueryFieldUnknown, Message: field.Name}
	}

	name := field.Alias
	if name == "" {
		name = field.Name
	}

	b := FieldBinding{
		Index:    index,
		Name:     name,
		TypeName: namedType(def.Type),
		Kind:     classify(def, len(field.SelectionSet) > 0),
	}

	if d := def.Directives.ForName("derived"); d != nil {
		if arg := d.Arguments.ForName("expr"); arg != nil && arg.Value != nil {
			b.Expr = arg.Value.Raw
		}
	}
	if d := def.Directives.ForName("source"); d != nil {
		if arg := d.Arguments.ForName("path"); arg != nil && arg.Value != nil {
			b.Path = stringList(arg.Value)
		}
	}
	if d := def.Directives.ForName("aggregate"); d != nil {
		if fn := d.Arguments.ForName("fn"); fn != nil && fn.Value != nil {
			b.Aggregate = fn.Value.Raw
		}
		if of := d.Arguments.ForName("of"); of != nil && of.Value != nil {
			b.AggregateOf = of.Value.Raw
		}
	}

	if len(field.SelectionSet) > 0 {
		children, err := bindSelectionSet(field.SelectionSet)
		if err != nil {
			return FieldBinding{}, err
		}
		b.Children = children
	}

	return b, nil
}

func classify(def *ast.FieldDefinition, hasChildren bool) BindingKind {
	if def.Directives.ForName("aggregate") != nil {
		return KindAggregate
	}
	if def.Directives.ForName("derived") != nil {
		return KindDerived
	}
	if def.Directives.ForName("source") != nil {
		return KindPath
	}
	if isList(def.Type) {
		return KindList
	}
	if hasChildren {
		return KindPath
	}
	return KindDirect
}

func isList(t *ast.Type) bool {
	for t != nil {
		if t.NamedType == "" && t.Elem != nil {
			return true
		}
		t = t.Elem
	}
	return false
}

func namedType(t *ast.Type) string {
	for t != nil {
		if t.NamedType != "" {
			return t.NamedType
		}
		t = t.Elem
	}
	return ""
}

func stringList(v *ast.Value) []string {
	if v == nil {
		return nil
	}
	if v.Kind == ast.ListValue {
		out := make([]string, 0, len(v.Children))
		for _, c := range v.Children {
			if c.Value != nil {
				out = append(out, c.Value.Raw)
			}
		}
		return out
	}
	if v.Raw != "" {
		return []string{v.Raw}
	}
	return nil
}

func mapGQLErrors(errs gqlerror.List) error {
	for _, e := range errs {
		if e == nil {
			continue
		}
		if isUnknownField(e) {
			return &Error{Code: CodeQueryFieldUnknown, Message: e.Error()}
		}
	}
	return &Error{Code: CodeQueryInvalid, Message: errs.Error()}
}

func isUnknownField(e *gqlerror.Error) bool {
	if e.Rule == "FieldsOnCorrectType" {
		return true
	}
	m := strings.ToLower(e.Message)
	return strings.Contains(m, "cannot query field")
}
