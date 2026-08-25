package engine

import (
	"testing"

	"github.com/samuelfabel/protoql/internal/catalog"
)

func TestEvalConcatCustomer(t *testing.T) {
	c := catalog.Customer{Name: "Samuel", Email: "s@example.com"}
	got, err := evalConcatCustomer(`concat(name, " <", email, ">")`, c)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Samuel <s@example.com>" {
		t.Errorf("got %q", got)
	}
}
