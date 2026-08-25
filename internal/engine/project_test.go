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

func TestProject_NestedObject(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { address { city } } }`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Project(plan, []catalog.Customer{{
		Name: "Alice",
		Address: &catalog.Address{
			City: "São Paulo",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	addr, ok := got[0]["address"].(Record)
	if !ok {
		t.Fatalf("address type = %T, want Record", got[0]["address"])
	}
	if addr["city"] != "São Paulo" {
		t.Errorf("city = %v, want São Paulo", addr["city"])
	}
	if _, flat := got[0]["city"]; flat {
		t.Error("nested projection must not flatten city onto the root")
	}
}

func TestProject_List(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { orders { id total } } }`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Project(plan, []catalog.Customer{{
		Name: "Alice",
		Orders: []catalog.Order{
			{ID: "o1", Total: 10},
			{ID: "o2", Total: 20.5},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	orders, ok := got[0]["orders"].([]Record)
	if !ok {
		t.Fatalf("orders type = %T, want []Record", got[0]["orders"])
	}
	if len(orders) != 2 {
		t.Fatalf("len(orders) = %d, want 2", len(orders))
	}
	if orders[0]["id"] != "o1" || orders[0]["total"] != 10.0 {
		t.Errorf("orders[0] = %+v", orders[0])
	}
	if orders[1]["id"] != "o2" || orders[1]["total"] != 20.5 {
		t.Errorf("orders[1] = %+v", orders[1])
	}
}

func TestProject_NullableObject(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { address { city } } }`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Project(plan, []catalog.Customer{{
		Name:    "Alice",
		Address: nil,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0]["address"] != nil {
		t.Errorf("address = %v, want nil", got[0]["address"])
	}
}

func TestProject_SourcePathCity(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { city } }`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Project(plan, []catalog.Customer{{
		Name:    "Alice",
		Address: &catalog.Address{City: "Curitiba"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0]["city"] != "Curitiba" {
		t.Errorf("city = %v, want Curitiba", got[0]["city"])
	}

	gotNil, err := Project(plan, []catalog.Customer{{Name: "Bob", Address: nil}})
	if err != nil {
		t.Fatal(err)
	}
	if gotNil[0]["city"] != nil {
		t.Errorf("city with nil address = %v, want nil", gotNil[0]["city"])
	}
}

func TestProject_EmptyList(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { orders { id } } }`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Project(plan, []catalog.Customer{{
		Name:   "Alice",
		Orders: []catalog.Order{},
	}})
	if err != nil {
		t.Fatal(err)
	}
	orders, ok := got[0]["orders"].([]Record)
	if !ok {
		t.Fatalf("orders type = %T, want []Record", got[0]["orders"])
	}
	if orders == nil {
		t.Fatal("empty list must not be nil")
	}
	if len(orders) != 0 {
		t.Errorf("len(orders) = %d, want 0", len(orders))
	}
}

func TestProject_NullViolation_List(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { orders { id } } }`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Project(plan, []catalog.Customer{{
		Name:   "Alice",
		Orders: nil,
	}})
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeEngineNullViolation {
		t.Fatalf("err=%v, want %s", err, CodeEngineNullViolation)
	}
}

func TestProject_FlatVsNested(t *testing.T) {
	flatPlan, err := compile.Compile(catalog.SDL, `query { customers { id name } }`)
	if err != nil {
		t.Fatal(err)
	}
	nestedPlan, err := compile.Compile(catalog.SDL, `query { customers { address { city } } }`)
	if err != nil {
		t.Fatal(err)
	}
	source := []catalog.Customer{{
		ID:   "c1",
		Name: "Alice",
		Address: &catalog.Address{
			City: "São Paulo",
		},
	}}

	flat, err := Project(flatPlan, source)
	if err != nil {
		t.Fatal(err)
	}
	if _, has := flat[0]["address"]; has {
		t.Error("flat projection should not include address")
	}

	nested, err := Project(nestedPlan, source)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := nested[0]["address"].(Record); !ok {
		t.Errorf("nested address type = %T, want Record", nested[0]["address"])
	}
}

