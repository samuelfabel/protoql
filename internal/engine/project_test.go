package engine

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestProject_derivedUnsupported(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { age } }`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Project(plan, []catalog.Customer{{Name: "Samuel"}})
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeEngineBindingUnsupported {
		t.Fatalf("err=%v", err)
	}
}

func TestProject_birthDateUnsupported(t *testing.T) {
	plan, err := compile.Compile(catalog.SDL, `query { customers { birthDate } }`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Project(plan, []catalog.Customer{{Name: "Samuel"}})
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeEngineBindingUnsupported {
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
