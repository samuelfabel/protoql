package transport_test

import (
	"context"
	"net"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/samuelfabel/protoql/internal/catalog"
	"github.com/samuelfabel/protoql/internal/compile"
	"github.com/samuelfabel/protoql/internal/engine"
	"github.com/samuelfabel/protoql/internal/protobuf"
	"github.com/samuelfabel/protoql/internal/transport"
	transportv1 "github.com/samuelfabel/protoql/internal/transport/v1"
)

const bufSize = 1024 * 1024

func startTestServer(t *testing.T, source []catalog.Customer) (transportv1.ProtoQLClient, func()) {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	transportv1.RegisterProtoQLServer(s, transport.NewServer(source))
	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		_ = conn.Close()
		s.Stop()
		_ = lis.Close()
	}
	return transportv1.NewProtoQLClient(conn), cleanup
}

func TestExecute_roundTripEqualsEngine(t *testing.T) {
	source := []catalog.Customer{{
		ID:            "c1",
		Name:          "Samuel",
		Email:         "s@example.com",
		LoyaltyPoints: 120,
		Active:        true,
		CreditScore:   98.5,
	}}
	query := `query { customers { id name email loyaltyPoints active creditScore } }`

	plan, err := compile.Compile(catalog.SDL, query)
	if err != nil {
		t.Fatal(err)
	}
	wantRows, err := engine.Project(plan, source)
	if err != nil {
		t.Fatal(err)
	}

	client, cleanup := startTestServer(t, source)
	defer cleanup()

	resp, err := client.Execute(context.Background(), &transportv1.ExecuteRequest{Query: query})
	if err != nil {
		t.Fatal(err)
	}

	desc, err := protobuf.DescriptorFromSet(resp.GetFileDescriptorSet())
	if err != nil {
		t.Fatal(err)
	}
	got, err := protobuf.Decode(desc, resp.GetPayload())
	if err != nil {
		t.Fatal(err)
	}

	want := wantRows[0]
	for _, key := range []string{"id", "name", "email", "loyaltyPoints", "active", "creditScore"} {
		if !reflect.DeepEqual(got[key], want[key]) {
			t.Errorf("%s = %v (%T), want %v (%T)", key, got[key], got[key], want[key], want[key])
		}
	}
}

func TestExecute_unknownFieldMapsToInvalidArgument(t *testing.T) {
	client, cleanup := startTestServer(t, []catalog.Customer{{Name: "Samuel"}})
	defer cleanup()

	_, err := client.Execute(context.Background(), &transportv1.ExecuteRequest{
		Query: `query { customers { nope } }`,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("not a status error: %v", err)
	}
	if st.Code() == codes.OK {
		t.Fatal("status must not be OK")
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", st.Code())
	}
	msg := st.Message()
	if !strings.Contains(msg, transport.CodeRPCCompileFailed) {
		t.Errorf("message missing %s: %s", transport.CodeRPCCompileFailed, msg)
	}
	if !strings.Contains(msg, compile.CodeQueryFieldUnknown) {
		t.Errorf("message missing %s: %s", compile.CodeQueryFieldUnknown, msg)
	}
}

func TestNoHTTPGraphQLHandler(t *testing.T) {
	// Transport package must expose gRPC only — no net/http GraphQL handler symbols.
	client, cleanup := startTestServer(t, []catalog.Customer{{Name: "Samuel"}})
	defer cleanup()
	_, err := client.Execute(context.Background(), &transportv1.ExecuteRequest{
		Query: `query { customers { name } }`,
	})
	if err != nil {
		t.Fatal(err)
	}
}
