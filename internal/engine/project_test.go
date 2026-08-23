package engine

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/samuelfabel/protoql/internal/catalog"
	"github.com/samuelfabel/protoql/internal/compile"
)

func TestProject_customerName(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { name } }`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Project(plan, []catalog.Customer{
		{ID: "1", Name: "Samuel", Email: "s@example.com"},
		{ID: "2", Name: "Ana", Email: "a@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []Record{
		{"name": "Samuel"},
		{"name": "Ana"},
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i]["name"] != want[i]["name"] {
			t.Errorf("[%d] name=%v want %v", i, got[i]["name"], want[i]["name"])
		}
		if _, ok := got[i]["email"]; ok {
			t.Errorf("[%d] did not request email: %+v", i, got[i])
		}
	}
}

func TestProject_idAndName(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { id name } }`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Project(plan, []catalog.Customer{{ID: "c1", Name: "Samuel"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["id"] != "c1" || got[0]["name"] != "Samuel" {
		t.Fatalf("got %+v", got)
	}
}

func TestProject_basicTypes(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query {
		customers {
			id
			name
			email
			loyaltyPoints
			active
			creditScore
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	source := []catalog.Customer{{
		ID:            "c1",
		Name:          "Samuel",
		Email:         "s@example.com",
		LoyaltyPoints: 120,
		Active:        true,
		CreditScore:   98.5,
	}}
	got, err := Project(plan, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
	rec := got[0]
	cases := []struct {
		field string
		want  any
	}{
		{"id", "c1"},
		{"name", "Samuel"},
		{"email", "s@example.com"},
		{"loyaltyPoints", 120},
		{"active", true},
		{"creditScore", 98.5},
	}
	for _, tc := range cases {
		if rec[tc.field] != tc.want {
			t.Errorf("%s = %v (%T), want %v (%T)", tc.field, rec[tc.field], rec[tc.field], tc.want, tc.want)
		}
	}
}

func TestProject_ageFromBirthDate(t *testing.T) {
	ref := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	NowFunc = func() time.Time { return ref }
	t.Cleanup(func() { NowFunc = time.Now })

	plan, err := compile.Compile(catalog.SDL, `query { customers { age } }`)
	if err != nil {
		t.Fatal(err)
	}
	birthDate := time.Date(1990, 6, 15, 0, 0, 0, 0, time.UTC)
	got, err := Project(plan, []catalog.Customer{{
		Name:      "Samuel",
		BirthDate: birthDate,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0]["age"] != 36 {
		t.Errorf("age = %v, want 36", got[0]["age"])
	}
}

func TestProject_availableFromStock(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { products { available } }`)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		stock int
		want  bool
	}{
		{0, false},
		{3, true},
	}
	for _, tc := range cases {
		got, err := ProjectProducts(plan, []catalog.Product{{ID: "p1", Stock: tc.stock}})
		if err != nil {
			t.Fatalf("stock=%d: %v", tc.stock, err)
		}
		if got[0]["available"] != tc.want {
			t.Errorf("stock=%d available=%v want %v", tc.stock, got[0]["available"], tc.want)
		}
	}
}

func TestProject_derivedEvalError(t *testing.T) {
	plan := &compile.ProjectionPlan{
		RootField: "customers",
		Bindings: []compile.FieldBinding{{
			Index: 1,
			Name:  "age",
			Kind:  compile.KindDerived,
			Expr:  "age(unknownField)",
		}},
	}
	_, err := Project(plan, []catalog.Customer{{Name: "Samuel"}})
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeDerivedEvalError {
		t.Fatalf("err=%v", err)
	}
}

func TestProject_birthDateTypeUnsupported(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { birthDate } }`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Project(plan, []catalog.Customer{{Name: "Samuel"}})
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeTypeUnsupported {
		t.Fatalf("err=%v", err)
	}
}

func TestNoGRPCOrProtobufImports(t *testing.T) {
	dir := "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range src.Imports {
			path := strings.Trim(im.Path.Value, `"`)
			if strings.Contains(path, "grpc") || strings.Contains(path, "protobuf") || strings.Contains(path, "protodesc") {
				t.Errorf("%s imports %s", e.Name(), path)
			}
		}
	}
}
