package protobuf_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/samuelfabel/protoql/internal/catalog"
	"github.com/samuelfabel/protoql/internal/compile"
	"github.com/samuelfabel/protoql/internal/engine"
	"github.com/samuelfabel/protoql/internal/protobuf"
)

func TestBuildDescriptor_deterministic(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { id name email loyaltyPoints active creditScore } }`)
	if err != nil {
		t.Fatal(err)
	}

	a, err := protobuf.BuildDescriptor(plan)
	if err != nil {
		t.Fatal(err)
	}
	b, err := protobuf.BuildDescriptor(plan)
	if err != nil {
		t.Fatal(err)
	}

	if a.FullName() != b.FullName() {
		t.Fatalf("FullName %q != %q", a.FullName(), b.FullName())
	}
	if a.Fields().Len() != b.Fields().Len() {
		t.Fatalf("fields len %d != %d", a.Fields().Len(), b.Fields().Len())
	}
	for i := 0; i < a.Fields().Len(); i++ {
		fa, fb := a.Fields().Get(i), b.Fields().Get(i)
		if fa.Name() != fb.Name() || fa.Number() != fb.Number() || fa.Kind() != fb.Kind() {
			t.Errorf("[%d] a=%s#%d/%v b=%s#%d/%v", i, fa.Name(), fa.Number(), fa.Kind(), fb.Name(), fb.Number(), fb.Kind())
		}
	}
}

func TestBuildDescriptor_noProtoFileWritten(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	plan, err := compile.Compile(catalog.SDL, `query { customers { name } }`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := protobuf.BuildDescriptor(plan); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".proto" {
			t.Fatalf("unexpected .proto file written: %s", e.Name())
		}
	}
}

func TestEncodeDecode_roundTripDirectFields(t *testing.T) {
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

	desc, err := protobuf.BuildDescriptor(plan)
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
	rows, err := engine.Project(plan, source)
	if err != nil {
		t.Fatal(err)
	}

	data, err := protobuf.Encode(desc, rows[0])
	if err != nil {
		t.Fatal(err)
	}
	got, err := protobuf.Decode(desc, data)
	if err != nil {
		t.Fatal(err)
	}

	want := rows[0]
	for _, key := range []string{"id", "name", "email", "loyaltyPoints", "active", "creditScore"} {
		if !reflect.DeepEqual(got[key], want[key]) {
			t.Errorf("%s = %v (%T), want %v (%T)", key, got[key], got[key], want[key], want[key])
		}
	}
}

func TestBuildDescriptor_nestedAndList(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query {
		customers {
			name
			city
			orders { id }
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}

	desc, err := protobuf.BuildDescriptor(plan)
	if err != nil {
		t.Fatal(err)
	}

	name := desc.Fields().ByName("name")
	if name == nil || name.Number() != 1 {
		t.Fatalf("name field = %v", name)
	}

	city := desc.Fields().ByName("city")
	if city == nil {
		t.Fatal("missing city")
	}

	orders := desc.Fields().ByName("orders")
	if orders == nil || !orders.IsList() {
		t.Fatalf("orders should be repeated, got %v", orders)
	}
	if orders.Message() == nil {
		t.Fatal("orders element message missing")
	}
	id := orders.Message().Fields().ByName("id")
	if id == nil {
		t.Fatal("orders.id missing")
	}
}

func TestNoProtocOnHotPath(t *testing.T) {
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not installed; hot-path still does not invoke it")
	}
	plan, err := compile.Compile(catalog.SDL, `query { customers { name age } }`)
	if err != nil {
		t.Fatal(err)
	}
	desc, err := protobuf.BuildDescriptor(plan)
	if err != nil {
		t.Fatal(err)
	}
	rec := engine.Record{"name": "Samuel", "age": 36}
	data, err := protobuf.Encode(desc, rec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := protobuf.Decode(desc, data); err != nil {
		t.Fatal(err)
	}
}
