package request

import (
	"testing"

	"github.com/samuelfabel/protoql/internal/catalog"
	"github.com/samuelfabel/protoql/internal/compile"
	transportv1 "github.com/samuelfabel/protoql/internal/transport/v1"
)

func TestCompile_singleQueryProjection(t *testing.T) {
	req := &transportv1.ExecuteRequest{
		Queries: []*transportv1.Operation{{
			Name: "customers",
			Result: []*transportv1.ResultField{
				{Name: "id"},
				{Name: "name"},
			},
		}},
	}
	plan, err := Compile(catalog.SDL, req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.RootField != "customers" || len(plan.Bindings) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Bindings[0].Name != "id" || plan.Bindings[1].Name != "name" {
		t.Fatalf("bindings = %+v", plan.Bindings)
	}
}

func TestCompile_parserField(t *testing.T) {
	parser := `concat(name, " ", email)`
	req := &transportv1.ExecuteRequest{
		Queries: []*transportv1.Operation{{
			Name: "customers",
			Result: []*transportv1.ResultField{{
				Name:   "displayName",
				Parser: &parser,
			}},
		}},
	}
	plan, err := Compile(catalog.SDL, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Bindings) != 1 {
		t.Fatalf("bindings = %+v", plan.Bindings)
	}
	b := plan.Bindings[0]
	if b.Kind != compile.KindDerived || b.Expr != parser || b.TypeName != "String" {
		t.Fatalf("binding = %+v", b)
	}
}

func TestCompile_nestedResult(t *testing.T) {
	req := &transportv1.ExecuteRequest{
		Queries: []*transportv1.Operation{{
			Name: "customers",
			Result: []*transportv1.ResultField{{
				Name: "address",
				Children: []*transportv1.ResultField{
					{Name: "city"},
				},
			}},
		}},
	}
	plan, err := Compile(catalog.SDL, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Bindings) != 1 || plan.Bindings[0].Kind != compile.KindPath {
		t.Fatalf("binding = %+v", plan.Bindings[0])
	}
	if len(plan.Bindings[0].Children) != 1 || plan.Bindings[0].Children[0].Name != "city" {
		t.Fatalf("children = %+v", plan.Bindings[0].Children)
	}
}

func TestCompile_rejectsBatchAndMutations(t *testing.T) {
	_, err := Compile(catalog.SDL, &transportv1.ExecuteRequest{
		Queries: []*transportv1.Operation{{Name: "customers", Result: []*transportv1.ResultField{{Name: "id"}}}, {Name: "products", Result: []*transportv1.ResultField{{Name: "id"}}}},
	})
	if err == nil {
		t.Fatal("expected error for batch")
	}
	_, err = Compile(catalog.SDL, &transportv1.ExecuteRequest{
		Mutations: []*transportv1.Operation{{Name: "createUser", Result: []*transportv1.ResultField{{Name: "id"}}}},
	})
	if err == nil {
		t.Fatal("expected error for mutations")
	}
}
