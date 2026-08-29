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
	"github.com/samuelfabel/protoql/internal/engine"
	"github.com/samuelfabel/protoql/internal/protobuf"
	"github.com/samuelfabel/protoql/internal/request"
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

func sampleCustomer() catalog.Customer {
	return catalog.Customer{
		ID:            "c1",
		Name:          "Samuel",
		Email:         "s@example.com",
		LoyaltyPoints: 120,
		Active:        true,
		CreditScore:   98.5,
		Address:       &catalog.Address{City: "Curitiba"},
	}
}

func TestExecute_roundTripEqualsEngine(t *testing.T) {
	source := []catalog.Customer{sampleCustomer()}
	req := &transportv1.ExecuteRequest{
		Queries: []*transportv1.Operation{{
			Name: "customers",
			Result: []*transportv1.ResultField{
				{Name: "id"},
				{Name: "name"},
				{Name: "email"},
				{Name: "loyaltyPoints"},
				{Name: "active"},
				{Name: "creditScore"},
			},
		}},
	}

	plan, err := request.Compile(catalog.SDL, req)
	if err != nil {
		t.Fatal(err)
	}
	wantRows, err := engine.Project(plan, source)
	if err != nil {
		t.Fatal(err)
	}

	client, cleanup := startTestServer(t, source)
	defer cleanup()

	resp, err := client.Execute(context.Background(), req)
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

func TestExecute_parserFieldRoundTrip(t *testing.T) {
	source := []catalog.Customer{sampleCustomer()}
	parser := `concat(name, " <", email, ">")`
	req := &transportv1.ExecuteRequest{
		Queries: []*transportv1.Operation{{
			Name: "customers",
			Result: []*transportv1.ResultField{{
				Name:   "displayName",
				Parser: &parser,
			}},
		}},
	}

	client, cleanup := startTestServer(t, source)
	defer cleanup()

	resp, err := client.Execute(context.Background(), req)
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
	if got["displayName"] != "Samuel <s@example.com>" {
		t.Errorf("displayName = %v", got["displayName"])
	}
}

func TestExecute_nestedResultRoundTrip(t *testing.T) {
	source := []catalog.Customer{sampleCustomer()}
	req := &transportv1.ExecuteRequest{
		Queries: []*transportv1.Operation{{
			Name: "customers",
			Result: []*transportv1.ResultField{{
				Name: "address",
				Children: []*transportv1.ResultField{
					{Name: "city"},
				},
			}},
		}},
	}

	client, cleanup := startTestServer(t, source)
	defer cleanup()

	resp, err := client.Execute(context.Background(), req)
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
	addr, ok := got["address"].(engine.Record)
	if !ok || addr["city"] != "Curitiba" {
		t.Fatalf("address = %v", got["address"])
	}
}

func TestExecute_unknownFieldMapsToInvalidArgument(t *testing.T) {
	client, cleanup := startTestServer(t, []catalog.Customer{{Name: "Samuel"}})
	defer cleanup()

	_, err := client.Execute(context.Background(), &transportv1.ExecuteRequest{
		Queries: []*transportv1.Operation{{
			Name:   "customers",
			Result: []*transportv1.ResultField{{Name: "nope"}},
		}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", st.Code())
	}
	if !strings.Contains(st.Message(), transport.CodeRPCCompileFailed) &&
		!strings.Contains(st.Message(), request.CodeRequestInvalid) {
		t.Fatalf("message = %s", st.Message())
	}
}

func TestExecute_rejectsInvalidRequest(t *testing.T) {
	client, cleanup := startTestServer(t, []catalog.Customer{{Name: "Samuel"}})
	defer cleanup()

	_, err := client.Execute(context.Background(), &transportv1.ExecuteRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	st, _ := status.FromError(err)
	if !strings.Contains(st.Message(), transport.CodeRPCRequestInvalid) {
		t.Fatalf("message = %s", st.Message())
	}
}

func TestNoHTTPGraphQLHandler(t *testing.T) {
	client, cleanup := startTestServer(t, []catalog.Customer{{Name: "Samuel"}})
	defer cleanup()
	_, err := client.Execute(context.Background(), &transportv1.ExecuteRequest{
		Queries: []*transportv1.Operation{{
			Name:   "customers",
			Result: []*transportv1.ResultField{{Name: "name"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
}
