package compile

import (
	"errors"
	"reflect"
	"testing"

	"github.com/samuelfabel/protoql/internal/catalog"
)

func TestCompile_customersName_directBinding(t *testing.T) {
	plan, err := Compile(catalog.SDL, `query { customers { name } }`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if plan.RootField != "customers" {
		t.Fatalf("RootField = %q", plan.RootField)
	}
	if len(plan.Bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(plan.Bindings))
	}
	b := plan.Bindings[0]
	if b.Index != 1 || b.Name != "name" || b.Kind != KindDirect {
		t.Fatalf("binding = %+v", b)
	}
}

func TestCompile_deterministic(t *testing.T) {
	q := `query { customers { name age city } }`
	a, err := Compile(catalog.SDL, q)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Compile(catalog.SDL, q)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("plans differ:\n%#v\n%#v", a, b)
	}
}

func TestCompile_unknownField(t *testing.T) {
	_, err := Compile(catalog.SDL, `query { customers { nope } }`)
	var cErr *Error
	if !errors.As(err, &cErr) {
		t.Fatalf("err type %T: %v", err, err)
	}
	if cErr.Code != CodeQueryFieldUnknown {
		t.Fatalf("code = %q, want %s (%v)", cErr.Code, CodeQueryFieldUnknown, cErr)
	}
}

func TestCompile_invalidDocument(t *testing.T) {
	_, err := Compile(catalog.SDL, `query {`)
	var cErr *Error
	if !errors.As(err, &cErr) {
		t.Fatalf("err type %T: %v", err, err)
	}
	if cErr.Code != CodeQueryInvalid {
		t.Fatalf("code = %q, want %s (%v)", cErr.Code, CodeQueryInvalid, cErr)
	}
}

func TestCompile_distinguishesDirectDerivedPathAggregate(t *testing.T) {
	plan, err := Compile(catalog.SDL, `query {
		customers {
			name
			age
			city
			orderCount
			orders { id }
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name string
		kind BindingKind
	}{
		{"name", KindDirect},
		{"age", KindDerived},
		{"city", KindPath},
		{"orderCount", KindAggregate},
		{"orders", KindList},
	}
	if len(plan.Bindings) != len(want) {
		t.Fatalf("len=%d plan=%+v", len(plan.Bindings), plan.Bindings)
	}
	for i, w := range want {
		b := plan.Bindings[i]
		if b.Name != w.name || b.Kind != w.kind || b.Index != i+1 {
			t.Errorf("[%d] %+v want name=%s kind=%s index=%d", i, b, w.name, w.kind, i+1)
		}
	}
	if plan.Bindings[1].Expr == "" {
		t.Error("age missing expr")
	}
	if len(plan.Bindings[2].Path) != 2 {
		t.Errorf("city path = %v", plan.Bindings[2].Path)
	}
	orders := plan.Bindings[4]
	if len(orders.Children) != 1 || orders.Children[0].Name != "id" || orders.Children[0].Kind != KindDirect {
		t.Errorf("orders children = %+v", orders.Children)
	}
}

func TestCompile_nullableFlags(t *testing.T) {
	plan, err := Compile(catalog.SDL, `query { customers { name address { city } orders { id } } }`)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]FieldBinding{}
	for _, b := range plan.Bindings {
		byName[b.Name] = b
	}
	if byName["name"].Nullable {
		t.Error("name should be non-null")
	}
	if !byName["address"].Nullable {
		t.Error("address should be nullable")
	}
	if byName["orders"].Nullable {
		t.Error("orders should be non-null")
	}
}
