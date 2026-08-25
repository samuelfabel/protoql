package transport

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/samuelfabel/protoql/internal/catalog"
	"github.com/samuelfabel/protoql/internal/compile"
	"github.com/samuelfabel/protoql/internal/engine"
	"github.com/samuelfabel/protoql/internal/protobuf"
	"github.com/samuelfabel/protoql/internal/request"
	transportv1 "github.com/samuelfabel/protoql/internal/transport/v1"
)

const (
	CodeRPCRequestInvalid = "RPC_REQUEST_INVALID"
	CodeRPCCompileFailed  = "RPC_COMPILE_FAILED"
	CodeRPCEngineFailed   = "RPC_ENGINE_FAILED"
)

// Server is the POC gRPC service: typed request DSL in, projection bytes out.
type Server struct {
	transportv1.UnimplementedProtoQLServer
	Schema string
	Source []catalog.Customer
}

// NewServer returns a ProtoQL server bound to the catalog SDL and in-memory customers.
func NewServer(source []catalog.Customer) *Server {
	return &Server{
		Schema: catalog.SDL,
		Source: source,
	}
}

// Execute compiles the request DSL, projects fixtures, and returns wire bytes plus descriptor set.
func (s *Server) Execute(ctx context.Context, req *transportv1.ExecuteRequest) (*transportv1.ExecuteResponse, error) {
	plan, err := request.Compile(s.Schema, req)
	if err != nil {
		return nil, mapRequestError(err)
	}

	rows, err := engine.Project(plan, s.Source)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s: %v", CodeRPCEngineFailed, err)
	}
	if len(rows) == 0 {
		return nil, status.Errorf(codes.Internal, "%s: %v", CodeRPCEngineFailed, err)
	}

	desc, fds, err := protobuf.BuildDescriptorWithSet(plan)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s: %v", CodeRPCEngineFailed, err)
	}

	payload, err := protobuf.Encode(desc, rows[0])
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s: %v", CodeRPCEngineFailed, err)
	}

	return &transportv1.ExecuteResponse{
		Payload:           payload,
		FileDescriptorSet: fds,
	}, nil
}

func mapRequestError(err error) error {
	var rErr *request.Error
	if errors.As(err, &rErr) {
		return status.Errorf(codes.InvalidArgument, "%s: %s", rErr.Code, rErr.Message)
	}
	var cErr *compile.Error
	if errors.As(err, &cErr) {
		return status.Errorf(codes.InvalidArgument, "%s: %s: %s", CodeRPCCompileFailed, cErr.Code, cErr.Message)
	}
	return status.Errorf(codes.InvalidArgument, "%s: %v", CodeRPCCompileFailed, err)
}
