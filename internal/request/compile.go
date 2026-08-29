package request

import (
	"fmt"
	"strings"

	"github.com/samuelfabel/protoql/internal/compile"
	transportv1 "github.com/samuelfabel/protoql/internal/transport/v1"
)

const CodeRequestInvalid = "RPC_REQUEST_INVALID"

// Error is a request DSL validation failure.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func requestInvalid(msg string) error {
	return &Error{Code: CodeRequestInvalid, Message: msg}
}

// Compile builds a ProjectionPlan from the typed ExecuteRequest (exactly one query).
func Compile(schema string, req *transportv1.ExecuteRequest) (*compile.ProjectionPlan, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	op := req.GetQueries()[0]
	if onlyParserFields(op.GetResult()) {
		stub := fmt.Sprintf("query { %s { id } }", op.GetName())
		plan, err := compile.Compile(schema, stub)
		if err != nil {
			return nil, err
		}
		bindings, err := buildParserOnlyBindings(op.GetResult())
		if err != nil {
			return nil, err
		}
		plan.Bindings = bindings
		return plan, nil
	}

	gql := buildGraphQL(op.GetName(), op.GetResult())
	plan, err := compile.Compile(schema, gql)
	if err != nil {
		return nil, err
	}
	if plan.RootField != op.GetName() {
		return nil, requestInvalid("root field mismatch")
	}

	bindings, err := mergeResultBindings(plan.Bindings, op.GetResult())
	if err != nil {
		return nil, err
	}
	plan.Bindings = bindings
	return plan, nil
}

func validateRequest(req *transportv1.ExecuteRequest) error {
	if req == nil {
		return requestInvalid("request is required")
	}
	if len(req.GetMutations()) > 0 {
		return requestInvalid("mutations are not supported in this milestone")
	}
	if len(req.GetQueries()) != 1 {
		return requestInvalid("exactly one query operation is required")
	}
	op := req.GetQueries()[0]
	if op == nil || strings.TrimSpace(op.GetName()) == "" {
		return requestInvalid("query name is required")
	}
	if len(op.GetResult()) == 0 {
		return requestInvalid("result projection is required")
	}
	if len(op.GetParameters()) > 0 {
		return requestInvalid("parameters are not supported in this milestone")
	}
	return nil
}

func buildGraphQL(root string, result []*transportv1.ResultField) string {
	var b strings.Builder
	b.WriteString("query { ")
	b.WriteString(root)
	b.WriteString(" { ")
	writeSelection(&b, result)
	b.WriteString("} }")
	return b.String()
}

func writeSelection(b *strings.Builder, fields []*transportv1.ResultField) {
	for _, rf := range fields {
		if rf == nil {
			continue
		}
		if rf.GetParser() != "" {
			continue
		}
		b.WriteString(rf.GetName())
		if len(rf.GetChildren()) > 0 {
			b.WriteString(" { ")
			writeSelection(b, rf.GetChildren())
			b.WriteString("}")
		}
		b.WriteString(" ")
	}
}

func mergeResultBindings(compiled []compile.FieldBinding, requested []*transportv1.ResultField) ([]compile.FieldBinding, error) {
	byName := make(map[string]compile.FieldBinding, len(compiled))
	for _, b := range compiled {
		byName[b.Name] = b
	}

	out := make([]compile.FieldBinding, 0, len(requested))
	for i, rf := range requested {
		if rf == nil {
			return nil, requestInvalid("result field is required")
		}
		idx := i + 1
		if rf.GetParser() != "" {
			if len(rf.GetChildren()) > 0 {
				return nil, requestInvalid("parser fields cannot have children in this milestone")
			}
			out = append(out, compile.FieldBinding{
				Index:    idx,
				Name:     rf.GetName(),
				Kind:     compile.KindDerived,
				TypeName: "String",
				Expr:     rf.GetParser(),
			})
			continue
		}

		b, ok := byName[rf.GetName()]
		if !ok {
			return nil, requestInvalid("unknown or unsupported field: " + rf.GetName())
		}
		b.Index = idx
		if len(rf.GetChildren()) > 0 {
			if len(b.Children) == 0 {
				return nil, requestInvalid("field " + rf.GetName() + " is not a nested selection")
			}
			children, err := mergeResultBindings(b.Children, rf.GetChildren())
			if err != nil {
				return nil, err
			}
			b.Children = children
		}
		out = append(out, b)
	}
	return out, nil
}

func onlyParserFields(fields []*transportv1.ResultField) bool {
	for _, rf := range fields {
		if rf == nil {
			continue
		}
		if rf.GetParser() == "" {
			return false
		}
	}
	return len(fields) > 0
}

func buildParserOnlyBindings(fields []*transportv1.ResultField) ([]compile.FieldBinding, error) {
	out := make([]compile.FieldBinding, 0, len(fields))
	for i, rf := range fields {
		if rf == nil || rf.GetParser() == "" {
			return nil, requestInvalid("parser field required")
		}
		if len(rf.GetChildren()) > 0 {
			return nil, requestInvalid("parser fields cannot have children in this milestone")
		}
		out = append(out, compile.FieldBinding{
			Index:    i + 1,
			Name:     rf.GetName(),
			Kind:     compile.KindDerived,
			TypeName: "String",
			Expr:     rf.GetParser(),
		})
	}
	return out, nil
}
