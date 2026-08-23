package engine_test

import (
	"testing"

	"github.com/samuelfabel/protoql/internal/catalog"
	"github.com/samuelfabel/protoql/internal/compile"
	"github.com/samuelfabel/protoql/internal/engine"
)

const testSchema = `
type Query {
  customer(id: ID!): Customer
}

type Customer {
  id: ID!
  name: String!
  email: String
  address: Address
  orders: [Order!]!
}

type Address {
  city: String!
  country: String!
}

type Order {
  id: ID!
}
`

func TestProject_NestedObject(t *testing.T) {
	query := `query { customer { address { city } } }`
	plan, err := compile.Compile(testSchema, query)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	source := catalog.Customer{
		ID:   "c1",
		Name: "Alice",
		Address: &catalog.Address{
			City:    "São Paulo",
			Country: "BR",
		},
	}

	result, err := engine.Project(plan, source)
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	addr, ok := result["address"].(engine.Record)
	if !ok {
		t.Fatalf("expected nested record for address, got %T", result["address"])
	}
	if addr["city"] != "São Paulo" {
		t.Errorf("city = %v, want São Paulo", addr["city"])
	}
}

func TestProject_NullableObject(t *testing.T) {
	query := `query { customer { address { city } } }`
	plan, err := compile.Compile(testSchema, query)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	source := catalog.Customer{
		ID:      "c1",
		Name:    "Alice",
		Address: nil,
	}

	result, err := engine.Project(plan, source)
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	if result["address"] != nil {
		t.Errorf("address = %v, want nil", result["address"])
	}
}

func TestProject_List(t *testing.T) {
	query := `query { customer { orders { id } } }`
	plan, err := compile.Compile(testSchema, query)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	source := catalog.Customer{
		ID:   "c1",
		Name: "Alice",
		Orders: []catalog.Order{
			{ID: "o1"},
			{ID: "o2"},
		},
	}

	result, err := engine.Project(plan, source)
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	orders, ok := result["orders"].([]engine.Record)
	if !ok {
		t.Fatalf("expected []Record for orders, got %T", result["orders"])
	}
	if len(orders) != 2 {
		t.Fatalf("len(orders) = %d, want 2", len(orders))
	}
	if orders[0]["id"] != "o1" {
		t.Errorf("orders[0].id = %v, want o1", orders[0]["id"])
	}
}

func TestProject_EmptyList(t *testing.T) {
	query := `query { customer { orders { id } } }`
	plan, err := compile.Compile(testSchema, query)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	source := catalog.Customer{
		ID:     "c1",
		Name:   "Alice",
		Orders: []catalog.Order{},
	}

	result, err := engine.Project(plan, source)
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	orders, ok := result["orders"].([]engine.Record)
	if !ok {
		t.Fatalf("expected []Record for orders, got %T", result["orders"])
	}
	if len(orders) != 0 {
		t.Errorf("len(orders) = %d, want 0", len(orders))
	}
}

func TestProject_NullViolation_List(t *testing.T) {
	query := `query { customer { orders { id } } }`
	plan, err := compile.Compile(testSchema, query)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	source := catalog.Customer{
		ID:     "c1",
		Name:   "Alice",
		Orders: nil,
	}

	_, err = engine.Project(plan, source)
	if err != engine.ErrNullViolation {
		t.Errorf("err = %v, want %v", err, engine.ErrNullViolation)
	}
}

func TestProject_FlatVsNested(t *testing.T) {
	flatQuery := `query { customer { id name } }`
	flatPlan, err := compile.Compile(testSchema, flatQuery)
	if err != nil {
		t.Fatalf("compile flat: %v", err)
	}

	nestedQuery := `query { customer { address { city } } }`
	nestedPlan, err := compile.Compile(testSchema, nestedQuery)
	if err != nil {
		t.Fatalf("compile nested: %v", err)
	}

	source := catalog.Customer{
		ID:   "c1",
		Name: "Alice",
		Address: &catalog.Address{
			City:    "São Paulo",
			Country: "BR",
		},
	}

	flatResult, err := engine.Project(flatPlan, source)
	if err != nil {
		t.Fatalf("project flat: %v", err)
	}
	if _, hasAddress := flatResult["address"]; hasAddress {
		t.Error("flat projection should not include address")
	}

	nestedResult, err := engine.Project(nestedPlan, source)
	if err != nil {
		t.Fatalf("project nested: %v", err)
	}
	if _, ok := nestedResult["address"].(engine.Record); !ok {
		t.Error("nested projection should include address as Record")
	}
}
