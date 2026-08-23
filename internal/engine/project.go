package engine

import (
	"reflect"

	"github.com/samuelfabel/protoql/internal/compile"
)

// Record is a dynamic projection result (map of field name to value).
type Record map[string]any

// Project applies an ExecutionPlan to a source value and returns a Record.
func Project(plan *compile.ExecutionPlan, source any) (Record, error) {
	if plan == nil {
		return nil, ErrPlanNil
	}

	src := reflect.ValueOf(source)
	if src.Kind() == reflect.Ptr {
		if src.IsNil() {
			return nil, ErrSourceNil
		}
		src = src.Elem()
	}

	if src.Kind() != reflect.Struct {
		return nil, ErrSourceNotStruct
	}

	out := make(Record, len(plan.Fields))
	for _, binding := range plan.Fields {
		val, err := projectField(src, binding)
		if err != nil {
			return nil, err
		}
		out[binding.OutputName] = val
	}
	return out, nil
}

func projectField(src reflect.Value, binding compile.FieldBinding) (any, error) {
	switch binding.Kind {
	case compile.KindScalar:
		return projectScalar(src, binding)
	case compile.KindPath:
		return projectPath(src, binding)
	case compile.KindList:
		return projectList(src, binding)
	default:
		return projectScalar(src, binding)
	}
}

func projectScalar(src reflect.Value, binding compile.FieldBinding) (any, error) {
	field, ok := compile.GoFieldByGraphQLName(src, binding.GraphQLName)
	if !ok {
		return nil, ErrFieldNotFound
	}

	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			if binding.Nullable {
				return nil, nil
			}
			return nil, ErrNullViolation
		}
		field = field.Elem()
	}

	return field.Interface(), nil
}

func projectPath(src reflect.Value, binding compile.FieldBinding) (any, error) {
	field, ok := compile.GoFieldByGraphQLName(src, binding.GraphQLName)
	if !ok {
		return nil, ErrFieldNotFound
	}

	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			if binding.Nullable {
				return nil, nil
			}
			return nil, ErrNullViolation
		}
		field = field.Elem()
	}

	if field.Kind() != reflect.Struct {
		return nil, ErrSourceNotStruct
	}

	out := make(Record, len(binding.Children))
	for _, child := range binding.Children {
		val, err := projectField(field, child)
		if err != nil {
			return nil, err
		}
		out[child.OutputName] = val
	}
	return out, nil
}

func projectList(src reflect.Value, binding compile.FieldBinding) (any, error) {
	field, ok := compile.GoFieldByGraphQLName(src, binding.GraphQLName)
	if !ok {
		return nil, ErrFieldNotFound
	}

	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			if binding.Nullable {
				return nil, nil
			}
			return nil, ErrNullViolation
		}
		field = field.Elem()
	}

	if field.Kind() != reflect.Slice && field.Kind() != reflect.Array {
		return nil, ErrSourceNotSlice
	}

	if field.IsNil() {
		if binding.Nullable {
			return nil, nil
		}
		return nil, ErrNullViolation
	}

	if len(binding.Children) == 0 {
		return nil, ErrListNeedsSelection
	}

	elemBinding := binding.Children[0]
	result := make([]Record, field.Len())
	for i := 0; i < field.Len(); i++ {
		elem := field.Index(i)
		val, err := projectField(elem, elemBinding)
		if err != nil {
			return nil, err
		}
		rec, ok := val.(Record)
		if !ok {
			return nil, ErrSourceNotStruct
		}
		result[i] = rec
	}
	return result, nil
}
