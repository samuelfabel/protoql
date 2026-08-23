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
	transportv1 "github.com/samuelfabel/protoql/internal/transport/v1"
)

const (
	CodeRPCCompileFailed = "RPC_COMPILE_FAILED"
	CodeRPCEngineFailed  = "RPC_ENGINE_FAILED"
)

// Server is the POC gRPC service: GraphQL query in, projection bytes out.
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

// Execute compiles the query, projects fixtures, and returns wire bytes plus descriptor set.
func (s *Server) Execute(ctx context.Context, req *transportv1.ExecuteRequest) (*transportv1.ExecuteResponse, error) {
	if req == nil || req.GetQuery() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s: query is required", CodeRPCCompileFailed)
	}

	plan, err := compile.Compile(s.Schema, req.GetQuery())
	if err != nil {
		return nil, mapCompileError(err)
	}

	rows, err := engine.Project(plan, s.Source)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s: %v", CodeRPCEngineFailed, err)
	}
	if len(rows) == 0 {
		return nil, status.Errorf(codes.Internal, "%s: empty projection", CodeRPCEngineFailed)
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
		Payload:            payload,
		FileDescriptorSet: fds,
	}, nil
}

func mapCompileError(err error) error {
	var cErr *compile.Error
	if errors.As(err, &cErr) {
		return status.Errorf(codes.InvalidArgument, "%s: %s: %s", CodeRPCCompileFailed, cErr.Code, cErr.Message)
	}
	return status.Errorf(codes.InvalidArgument, "%s: %v", CodeRPCCompileFailed, err)
}
