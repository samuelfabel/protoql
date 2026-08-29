package protobuf

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/samuelfabel/protoql/internal/compile"
)

const (
	fileName    = "protoql_projection.proto"
	packageName = "protoql.projection"
	rootMsgName = "Projection"
)

// BuildDescriptor builds a deterministic Protobuf message descriptor from a ProjectionPlan.
// Field numbers follow plan document order starting at 1 (binding.Index).
// No .proto source file is written; the descriptor exists only in memory.
func BuildDescriptor(plan *compile.ProjectionPlan) (protoreflect.MessageDescriptor, error) {
	desc, _, err := BuildDescriptorWithSet(plan)
	return desc, err
}

// BuildDescriptorWithSet returns the root message descriptor and a marshaled FileDescriptorSet.
func BuildDescriptorWithSet(plan *compile.ProjectionPlan) (protoreflect.MessageDescriptor, []byte, error) {
	file, err := buildFileProto(plan)
	if err != nil {
		return nil, nil, err
	}

	fds := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, nil, descriptorBuildFailed(err.Error())
	}

	fullName := protoreflect.FullName(packageName + "." + rootMsgName)
	desc, err := files.FindDescriptorByName(fullName)
	if err != nil {
		return nil, nil, descriptorBuildFailed(err.Error())
	}
	msg, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, nil, descriptorBuildFailed("root is not a message descriptor")
	}

	raw, err := proto.Marshal(fds)
	if err != nil {
		return nil, nil, descriptorBuildFailed(err.Error())
	}
	return msg, raw, nil
}

// DescriptorFromSet loads the Projection message descriptor from a marshaled FileDescriptorSet.
func DescriptorFromSet(raw []byte) (protoreflect.MessageDescriptor, error) {
	fds := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(raw, fds); err != nil {
		return nil, descriptorBuildFailed(err.Error())
	}
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, descriptorBuildFailed(err.Error())
	}
	fullName := protoreflect.FullName(packageName + "." + rootMsgName)
	desc, err := files.FindDescriptorByName(fullName)
	if err != nil {
		return nil, descriptorBuildFailed(err.Error())
	}
	msg, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, descriptorBuildFailed("root is not a message descriptor")
	}
	return msg, nil
}

func buildFileProto(plan *compile.ProjectionPlan) (*descriptorpb.FileDescriptorProto, error) {
	if plan == nil {
		return nil, descriptorBuildFailed("plan is required")
	}
	if len(plan.Bindings) == 0 {
		return nil, descriptorBuildFailed("plan has no bindings")
	}

	var messages []*descriptorpb.DescriptorProto
	root, err := buildMessage(rootMsgName, plan.Bindings, &messages)
	if err != nil {
		return nil, err
	}
	messages = append(messages, root)

	return &descriptorpb.FileDescriptorProto{
		Name:        strPtr(fileName),
		Package:     strPtr(packageName),
		Syntax:      strPtr("proto3"),
		MessageType: messages,
	}, nil
}

func buildMessage(name string, bindings []compile.FieldBinding, all *[]*descriptorpb.DescriptorProto) (*descriptorpb.DescriptorProto, error) {
	msg := &descriptorpb.DescriptorProto{Name: strPtr(name)}
	for _, b := range bindings {
		num := int32(b.Index)
		if num < 1 {
			return nil, descriptorBuildFailed(fmt.Sprintf("field %q has invalid index %d", b.Name, b.Index))
		}
		fieldName := sanitizeFieldName(b.Name)

		switch b.Kind {
		case compile.KindPath:
			if len(b.Children) == 0 {
				// @source path to a scalar leaf — encode as scalar, not nested message.
				pbType, err := scalarProtoType(b.TypeName)
				if err != nil {
					return nil, err
				}
				msg.Field = append(msg.Field, &descriptorpb.FieldDescriptorProto{
					Name:   strPtr(fieldName),
					Number: int32Ptr(num),
					Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					Type:   typePtr(pbType),
				})
				continue
			}
			childName := nestedMessageName(name, fieldName)
			child, err := buildMessage(childName, b.Children, all)
			if err != nil {
				return nil, err
			}
			*all = append(*all, child)
			msg.Field = append(msg.Field, &descriptorpb.FieldDescriptorProto{
				Name:     strPtr(fieldName),
				Number:   int32Ptr(num),
				Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
				Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
				TypeName: strPtr("." + packageName + "." + childName),
			})

		case compile.KindList:
			childName := nestedMessageName(name, fieldName)
			child, err := buildMessage(childName, b.Children, all)
			if err != nil {
				return nil, err
			}
			*all = append(*all, child)
			msg.Field = append(msg.Field, &descriptorpb.FieldDescriptorProto{
				Name:     strPtr(fieldName),
				Number:   int32Ptr(num),
				Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED),
				Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
				TypeName: strPtr("." + packageName + "." + childName),
			})

		default:
			pbType, err := scalarProtoType(b.TypeName)
			if err != nil {
				return nil, err
			}
			msg.Field = append(msg.Field, &descriptorpb.FieldDescriptorProto{
				Name:   strPtr(fieldName),
				Number: int32Ptr(num),
				Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
				Type:   typePtr(pbType),
			})
		}
	}
	return msg, nil
}

func scalarProtoType(graphqlType string) (descriptorpb.FieldDescriptorProto_Type, error) {
	switch graphqlType {
	case "String", "ID", "Date", "DateTime":
		return descriptorpb.FieldDescriptorProto_TYPE_STRING, nil
	case "Int":
		return descriptorpb.FieldDescriptorProto_TYPE_INT32, nil
	case "Boolean":
		return descriptorpb.FieldDescriptorProto_TYPE_BOOL, nil
	case "Float", "Decimal":
		return descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, nil
	case "":
		return 0, descriptorBuildFailed("missing TypeName for scalar binding")
	default:
		return 0, descriptorBuildFailed("unsupported GraphQL type for Protobuf: " + graphqlType)
	}
}

func nestedMessageName(parent, field string) string {
	return parent + "_" + field
}

func sanitizeFieldName(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	if name == "" {
		return "field"
	}
	return name
}

func strPtr(s string) *string { return &s }
func int32Ptr(v int32) *int32 { return &v }
func typePtr(t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	return &t
}
func labelPtr(l descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label {
	return &l
}
