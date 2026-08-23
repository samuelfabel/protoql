package protobuf

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/samuelfabel/protoql/internal/engine"
)

// Encode marshals a projection Record into Protobuf wire bytes using desc.
func Encode(desc protoreflect.MessageDescriptor, rec engine.Record) ([]byte, error) {
	if desc == nil {
		return nil, descriptorBuildFailed("descriptor is required")
	}
	msg := dynamicpb.NewMessage(desc)
	if err := setRecord(msg, rec); err != nil {
		return nil, err
	}
	return proto.Marshal(msg)
}

// Decode unmarshals Protobuf wire bytes into a projection Record using desc.
func Decode(desc protoreflect.MessageDescriptor, data []byte) (engine.Record, error) {
	if desc == nil {
		return nil, descriptorBuildFailed("descriptor is required")
	}
	msg := dynamicpb.NewMessage(desc)
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, descriptorBuildFailed(err.Error())
	}
	return getRecord(msg), nil
}

func setRecord(msg *dynamicpb.Message, rec engine.Record) error {
	fields := msg.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		name := string(fd.Name())
		val, ok := rec[name]
		if !ok || val == nil {
			continue
		}
		if err := setField(msg, fd, val); err != nil {
			return err
		}
	}
	return nil
}

func setField(msg *dynamicpb.Message, fd protoreflect.FieldDescriptor, val any) error {
	switch {
	case fd.IsList():
		listVal, ok := val.([]engine.Record)
		if !ok {
			return descriptorBuildFailed(fmt.Sprintf("field %s: expected []Record, got %T", fd.Name(), val))
		}
		list := msg.Mutable(fd).List()
		for _, item := range listVal {
			child := dynamicpb.NewMessage(fd.Message())
			if err := setRecord(child, item); err != nil {
				return err
			}
			list.Append(protoreflect.ValueOfMessage(child))
		}
		return nil

	case fd.Kind() == protoreflect.MessageKind:
		childRec, ok := val.(engine.Record)
		if !ok {
			return descriptorBuildFailed(fmt.Sprintf("field %s: expected Record, got %T", fd.Name(), val))
		}
		child := dynamicpb.NewMessage(fd.Message())
		if err := setRecord(child, childRec); err != nil {
			return err
		}
		msg.Set(fd, protoreflect.ValueOfMessage(child))
		return nil

	default:
		pv, err := toProtoValue(fd, val)
		if err != nil {
			return err
		}
		msg.Set(fd, pv)
		return nil
	}
}

func toProtoValue(fd protoreflect.FieldDescriptor, val any) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.StringKind:
		s, ok := val.(string)
		if !ok {
			return protoreflect.Value{}, descriptorBuildFailed(fmt.Sprintf("field %s: want string, got %T", fd.Name(), val))
		}
		return protoreflect.ValueOfString(s), nil
	case protoreflect.Int32Kind:
		switch v := val.(type) {
		case int:
			return protoreflect.ValueOfInt32(int32(v)), nil
		case int32:
			return protoreflect.ValueOfInt32(v), nil
		case int64:
			return protoreflect.ValueOfInt32(int32(v)), nil
		default:
			return protoreflect.Value{}, descriptorBuildFailed(fmt.Sprintf("field %s: want int, got %T", fd.Name(), val))
		}
	case protoreflect.BoolKind:
		b, ok := val.(bool)
		if !ok {
			return protoreflect.Value{}, descriptorBuildFailed(fmt.Sprintf("field %s: want bool, got %T", fd.Name(), val))
		}
		return protoreflect.ValueOfBool(b), nil
	case protoreflect.DoubleKind:
		switch v := val.(type) {
		case float64:
			return protoreflect.ValueOfFloat64(v), nil
		case float32:
			return protoreflect.ValueOfFloat64(float64(v)), nil
		default:
			return protoreflect.Value{}, descriptorBuildFailed(fmt.Sprintf("field %s: want float64, got %T", fd.Name(), val))
		}
	default:
		return protoreflect.Value{}, descriptorBuildFailed(fmt.Sprintf("unsupported protobuf kind %v for field %s", fd.Kind(), fd.Name()))
	}
}

func getRecord(msg protoreflect.Message) engine.Record {
	rec := make(engine.Record)
	fields := msg.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		name := string(fd.Name())
		if !msg.Has(fd) && !fd.IsList() {
			continue
		}
		rec[name] = getField(msg, fd)
	}
	return rec
}

func getField(msg protoreflect.Message, fd protoreflect.FieldDescriptor) any {
	switch {
	case fd.IsList():
		list := msg.Get(fd).List()
		out := make([]engine.Record, list.Len())
		for i := 0; i < list.Len(); i++ {
			out[i] = getRecord(list.Get(i).Message())
		}
		return out

	case fd.Kind() == protoreflect.MessageKind:
		if !msg.Has(fd) {
			return nil
		}
		return getRecord(msg.Get(fd).Message())

	default:
		return fromProtoValue(fd, msg.Get(fd))
	}
}

func fromProtoValue(fd protoreflect.FieldDescriptor, v protoreflect.Value) any {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return v.String()
	case protoreflect.Int32Kind:
		return int(v.Int())
	case protoreflect.BoolKind:
		return v.Bool()
	case protoreflect.DoubleKind:
		return v.Float()
	default:
		return nil
	}
}
