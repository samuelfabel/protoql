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
	}
}

func TestProject_aggregates(t *testing.T) {
	customerPlan, err := compile.Compile(catalog.SDL, `query { customers { orderCount totalSpent } }`)
	if err != nil {
		t.Fatal(err)
	}
	productPlan, err := compile.Compile(catalog.SDL, `query { products { averageRating } }`)
	if err != nil {
		t.Fatal(err)
	}

	customers := []catalog.Customer{{
		Name: "Samuel",
		Orders: []catalog.Order{
			{ID: "o1", Total: 100.0},
			{ID: "o2", Total: 50.5},
		},
	}}
	gotCustomers, err := Project(customerPlan, customers)
	if err != nil {
		t.Fatal(err)
	}
	if gotCustomers[0]["orderCount"] != 2 {
		t.Errorf("orderCount = %v, want 2", gotCustomers[0]["orderCount"])
	}
	if gotCustomers[0]["totalSpent"] != 150.5 {
		t.Errorf("totalSpent = %v, want 150.5", gotCustomers[0]["totalSpent"])
	}

	products := []catalog.Product{{
		ID: "p1",
		Reviews: []catalog.Review{
			{Rating: 4.0},
			{Rating: 5.0},
			{Rating: 3.0},
		},
	}}
	gotProducts, err := ProjectProducts(productPlan, products)
	if err != nil {
		t.Fatal(err)
	}
	if gotProducts[0]["averageRating"] != 4.0 {
		t.Errorf("averageRating = %v, want 4.0", gotProducts[0]["averageRating"])
	}
}

func TestProject_aggregateEmptyAvg(t *testing.T) {
	plan := &compile.ProjectionPlan{
		RootField: "products",
		Bindings: []compile.FieldBinding{{
			Index:       1,
			Name:        "averageRating",
			Kind:        compile.KindAggregate,
			Aggregate:   "avg",
			AggregateOf: "reviews.rating",
		}},
	}
	_, err := ProjectProducts(plan, []catalog.Product{{ID: "p1", Reviews: nil}})
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeAggregateEmpty {
		t.Fatalf("err=%v, want %s", err, CodeAggregateEmpty)
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
	if got[0]["age"] != 36 {
		t.Errorf("age = %v, want 36", got[0]["age"])
	}
}

func TestProject_availableFromStock(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { products { available } }`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		stock int
		want  bool
	}{
		{0, false},
		{3, true},
	} {
		got, err := ProjectProducts(plan, []catalog.Product{{ID: "p1", Stock: tc.stock}})
		if err != nil {
			t.Fatalf("stock=%d: %v", tc.stock, err)
		}
		if got[0]["available"] != tc.want {
			t.Errorf("stock=%d available=%v want %v", tc.stock, got[0]["available"], tc.want)
		}
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
