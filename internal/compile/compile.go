package compile

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// Compile compiles a GraphQL query string into an ExecutionPlan.
func Compile(schema string, query string) (*ExecutionPlan, error) {
	schemaDoc, err := gqlparser.LoadSchema(&ast.Source{Name: "schema.graphql", Input: schema})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
	}

	queryDoc, err := gqlparser.LoadQuery(schemaDoc, &ast.Source{Name: "query.graphql", Input: query})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueryInvalid, err)
	}

	if len(queryDoc.Operations) != 1 {
		return nil, fmt.Errorf("%w: expected exactly one operation", ErrQueryInvalid)
	}

	op := queryDoc.Operations[0]
	if op.Operation != ast.Query {
		return nil, fmt.Errorf("%w: only query operations are supported", ErrQueryInvalid)
	}

	rootType := schemaDoc.Query
	if rootType == nil {
		return nil, fmt.Errorf("%w: schema has no Query type", ErrSchemaInvalid)
	}

	fields, err := compileSelectionSet(schemaDoc, rootType, op.SelectionSet, "")
	if err != nil {
		return nil, err
	}

	return &ExecutionPlan{
		RootType: rootType.Name,
		Fields:   fields,
	}, nil
}

func compileSelectionSet(schema *ast.Schema, parentType *ast.Definition, set ast.SelectionSet, prefix string) ([]FieldBinding, error) {
	var bindings []FieldBinding

	for _, sel := range set {
		switch node := sel.(type) {
		case *ast.Field:
			fieldDef := parentType.Fields.ForName(node.Name)
			if fieldDef == nil {
				return nil, fmt.Errorf("%w: field %q not found on type %q", ErrFieldNotFound, node.Name, parentType.Name)
			}

			if len(node.Arguments) > 0 {
				return nil, fmt.Errorf("%w: field %q has arguments (not supported)", ErrQueryInvalid, node.Name)
			}

			outputName := node.Alias
			if outputName == "" {
				outputName = node.Name
			}

			path := outputName
			if prefix != "" {
				path = prefix + "." + outputName
			}

			binding := FieldBinding{
				GraphQLName: node.Name,
				OutputName:  outputName,
				Path:        path,
				Nullable:    fieldNullable(fieldDef.Type),
			}

			if len(node.SelectionSet) > 0 {
				childType, err := resolveNamedType(schema, fieldDef.Type)
				if err != nil {
					return nil, err
				}
				if childType.Kind != ast.Object {
					return nil, fmt.Errorf("%w: field %q is not an object type", ErrQueryInvalid, node.Name)
				}

				children, err := compileSelectionSet(schema, childType, node.SelectionSet, path)
				if err != nil {
					return nil, err
				}
				binding.Children = children
				binding.Kind = KindPath
			} else if isListType(fieldDef.Type) {
				binding.Kind = KindList
			} else {
				binding.Kind = KindScalar
			}

			bindings = append(bindings, binding)

		case *ast.InlineFragment, *ast.FragmentSpread:
			return nil, fmt.Errorf("%w: fragments are not supported", ErrQueryInvalid)
		}
	}

	return bindings, nil
}

func resolveNamedType(schema *ast.Schema, t *ast.Type) (*ast.Definition, error) {
	for t.NamedType == "" {
		t = t.Elem
	}
	def := schema.Types[t.NamedType]
	if def == nil {
		return nil, fmt.Errorf("%w: type %q not found", ErrSchemaInvalid, t.NamedType)
	}
	return def, nil
}

func isListType(t *ast.Type) bool {
	for t.Elem != nil {
		if t.Elem.NamedType == "" {
			t = t.Elem
			continue
		}
		return t.Elem.NamedType == ""
	}
	return false
}

func fieldNullable(t *ast.Type) bool {
	if t == nil {
		return false
	}
	if t.NonNull {
		return false
	}
	if t.NamedType != "" {
		return true
	}
	return fieldNullable(t.Elem)
}

// FieldPath returns the dot-separated path for a binding (used by the engine).
func FieldPath(b FieldBinding) string {
	return b.Path
}

// StructFieldName converts a GraphQL field name to the expected Go struct field name (exported).
func StructFieldName(graphQLName string) string {
	if graphQLName == "" {
		return ""
	}
	return strings.ToUpper(graphQLName[:1]) + graphQLName[1:]
}

// GoFieldByGraphQLName looks up a struct field by GraphQL name using reflection.
func GoFieldByGraphQLName(v reflect.Value, graphQLName string) (reflect.Value, bool) {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	name := StructFieldName(graphQLName)
	f := v.FieldByName(name)
	if !f.IsValid() {
		return reflect.Value{}, false
	}
	return f, true
}
