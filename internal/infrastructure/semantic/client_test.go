package semantic

import (
	"context"
	"math"
	"net"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const testToken = "go-test-internal-token"

// testServer is a hermetic semantic service double mirroring the Python
// server's auth behavior: every RPC requires the internal token metadata;
// unimplemented methods stay UNIMPLEMENTED; GetCapabilities behavior is
// steerable for transport tests.
type testServer struct {
	semanticpb.UnimplementedSemanticServer
	capabilitiesBehavior func(ctx context.Context) (*semanticpb.GetCapabilitiesResponse, error)
	capabilitiesCalls    int
	applyCalls           int
	getBehavior          func(ctx context.Context) (*semanticpb.Operation, error)
	getCalls             int
	lastTokenValues      []string
}

func (s *testServer) GetOperation(ctx context.Context, req *semanticpb.GetOperationRequest) (*semanticpb.Operation, error) {
	s.getCalls++
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.lastTokenValues = append([]string(nil), md.Get("x-semantic-token")...)
	}
	if s.getBehavior != nil {
		return s.getBehavior(ctx)
	}
	return &semanticpb.Operation{OperationId: req.OperationId, Stage: "queued"}, nil
}

func (s *testServer) GetCapabilities(ctx context.Context, req *semanticpb.GetCapabilitiesRequest) (*semanticpb.GetCapabilitiesResponse, error) {
	s.capabilitiesCalls++
	if s.capabilitiesBehavior != nil {
		return s.capabilitiesBehavior(ctx)
	}
	return &semanticpb.GetCapabilitiesResponse{
		ProtocolVersion: "1",
		EngineVersion:   "semantica-0.6.8",
		Capabilities: []*semanticpb.Capability{{
			Mode:      "authorized_subgraph",
			Available: true,
		}},
	}, nil
}

func (s *testServer) ApplyDocumentRevision(ctx context.Context, req *semanticpb.ApplyDocumentRevisionRequest) (*semanticpb.Operation, error) {
	s.applyCalls++
	return nil, status.Error(codes.Unimplemented, "apply not implemented")
}

func authInterceptor(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		values := md.Get("x-semantic-token")
		// The client strips duplicates; exactly one value must match.
		if !ok || len(values) != 1 || values[0] != token {
			return nil, status.Error(codes.Unauthenticated, "service identity required")
		}
		return handler(ctx, req)
	}
}

// streamAuthInterceptor mirrors the deny behavior for streaming RPCs so the
// auth story stays consistent while only unary business RPCs exist.
func streamAuthInterceptor(token string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(ss.Context())
		values := md.Get("x-semantic-token")
		if !ok || len(values) != 1 || values[0] != token {
			return status.Error(codes.Unauthenticated, "service identity required")
		}
		return handler(srv, ss)
	}
}

func startTestServer(t *testing.T, srv *testServer) (address string, stop func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(authInterceptor(testToken)),
		grpc.ChainStreamInterceptor(streamAuthInterceptor(testToken)),
	)
	semanticpb.RegisterSemanticServer(server, srv)
	go func() { _ = server.Serve(listener) }()
	return listener.Addr().String(), server.Stop
}

func newTestClient(t *testing.T, address, token string) *grpcClient {
	t.Helper()
	client, err := NewClient(SemanticClientConfig{
		Address:       address,
		InternalToken: token,
		CallTimeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client.(*grpcClient)
}

func TestSemanticClientUnauthenticated(t *testing.T) {
	srv := &testServer{}
	address, stop := startTestServer(t, srv)
	defer stop()
	client := newTestClient(t, address, "wrong-token")
	_, err := client.Capabilities(context.Background())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want UNAUTHENTICATED, got %v", err)
	}
}

func TestSemanticClientUnimplementedIsNotFakeSuccess(t *testing.T) {
	srv := &testServer{}
	address, stop := startTestServer(t, srv)
	defer stop()
	client := newTestClient(t, address, testToken)
	_, err := client.Apply(context.Background(), types.SemanticApplyRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("want UNIMPLEMENTED, got %v", err)
	}
}

func TestSemanticClientCapabilitiesRoundTrip(t *testing.T) {
	srv := &testServer{}
	address, stop := startTestServer(t, srv)
	defer stop()
	client := newTestClient(t, address, testToken)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if capabilities.ProtocolVersion != "1" || capabilities.EngineVersion != "semantica-0.6.8" {
		t.Fatalf("capabilities not mapped: %+v", capabilities)
	}
	if len(capabilities.Capabilities) != 1 || !capabilities.Capabilities[0].Available {
		t.Fatalf("capability lost: %+v", capabilities)
	}
}

func TestSemanticClientDeadlineExceeded(t *testing.T) {
	srv := &testServer{capabilitiesBehavior: func(ctx context.Context) (*semanticpb.GetCapabilitiesResponse, error) {
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	}}
	address, stop := startTestServer(t, srv)
	defer stop()
	client := newTestClient(t, address, testToken)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_, err := client.Capabilities(ctx)
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("want DEADLINE_EXCEEDED, got %v", err)
	}
}

func TestSemanticClientCanceled(t *testing.T) {
	srv := &testServer{capabilitiesBehavior: func(ctx context.Context) (*semanticpb.GetCapabilitiesResponse, error) {
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	}}
	address, stop := startTestServer(t, srv)
	defer stop()
	client := newTestClient(t, address, testToken)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(40 * time.Millisecond)
		cancel()
	}()
	_, err := client.Capabilities(ctx)
	if status.Code(err) != codes.Canceled {
		t.Fatalf("want CANCELED, got %v", err)
	}
}

func TestSemanticClientConnectionFailure(t *testing.T) {
	// A bound-but-closed port: dialing fails with Unavailable, not a hang.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	address := listener.Addr().String()
	listener.Close()

	client, err := NewClient(SemanticClientConfig{Address: address, InternalToken: testToken, CallTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer func() { _ = client.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = client.Capabilities(ctx)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("want UNAVAILABLE, got %v", err)
	}
}

func TestSemanticClientRetriesReadOnlyButNotApply(t *testing.T) {
	var calls int
	srv := &testServer{capabilitiesBehavior: func(ctx context.Context) (*semanticpb.GetCapabilitiesResponse, error) {
		calls++
		if calls == 1 {
			return nil, status.Error(codes.Unavailable, "transient")
		}
		return &semanticpb.GetCapabilitiesResponse{ProtocolVersion: "1"}, nil
	}}
	address, stop := startTestServer(t, srv)
	defer stop()
	client := newTestClient(t, address, testToken)

	if _, err := client.Capabilities(context.Background()); err != nil {
		t.Fatalf("capabilities with transient failure: %v", err)
	}
	if calls != 2 {
		t.Fatalf("read-only retry expected 2 calls, got %d", calls)
	}

	// Apply must NOT be transparently retried: a failing Apply surfaces the
	// error; the business layer owns retry via idempotency keys.
	failServer := &testServer{}
	failAddr, failStop := startTestServer(t, failServer)
	defer failStop()
	failServer.capabilitiesBehavior = func(ctx context.Context) (*semanticpb.GetCapabilitiesResponse, error) {
		return nil, status.Error(codes.Unavailable, "transient")
	}
	failClient := newTestClient(t, failAddr, testToken)
	_, err := failClient.Apply(context.Background(), types.SemanticApplyRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("apply error must surface verbatim, got %v", err)
	}
	if failServer.applyCalls != 1 {
		t.Fatalf("apply must not be retried, got %d calls", failServer.applyCalls)
	}
}

// TestSemanticClientStripsStaleCallerToken pins the single-value semantic:
// a caller context already carrying x-semantic-token must not produce
// duplicate metadata - the client's own identity is the only value sent.
func TestSemanticClientStripsStaleCallerToken(t *testing.T) {
	srv := &testServer{}
	address, stop := startTestServer(t, srv)
	defer stop()
	client := newTestClient(t, address, testToken)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-semantic-token", "stale-caller-value")
	if _, err := client.Get(ctx, types.SemanticScopeKey{TenantID: 1, KBID: "kb"}, "op-1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(srv.lastTokenValues) != 1 || srv.lastTokenValues[0] != testToken {
		t.Fatalf("client must send exactly its own token, got %v", srv.lastTokenValues)
	}
}

// TestSemanticClientGetRetriesTransientFailure exercises the read-only
// retry path end-to-end (Get is a read operation).
func TestSemanticClientGetRetriesTransientFailure(t *testing.T) {
	calls := 0
	srv := &testServer{getBehavior: func(ctx context.Context) (*semanticpb.Operation, error) {
		calls++
		if calls == 1 {
			return nil, status.Error(codes.Unavailable, "transient")
		}
		return &semanticpb.Operation{OperationId: "op-1", Stage: "running"}, nil
	}}
	address, stop := startTestServer(t, srv)
	defer stop()
	client := newTestClient(t, address, testToken)
	operation, err := client.Get(context.Background(), types.SemanticScopeKey{TenantID: 1, KBID: "kb"}, "op-1")
	if err != nil {
		t.Fatalf("get with transient failure: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected retry, got %d calls", calls)
	}
	if operation.OperationID != "op-1" || operation.Stage != "running" {
		t.Fatalf("operation not mapped: %+v", operation)
	}
}

// TestSemanticClientRetrySurfacesContextError pins that a deadline firing
// during retry backoff surfaces DEADLINE_EXCEEDED, not a stale Unavailable.
func TestSemanticClientRetrySurfacesContextError(t *testing.T) {
	srv := &testServer{getBehavior: func(ctx context.Context) (*semanticpb.Operation, error) {
		return nil, status.Error(codes.Unavailable, "transient")
	}}
	address, stop := startTestServer(t, srv)
	defer stop()
	client, err := NewClient(SemanticClientConfig{
		Address: address, InternalToken: testToken,
		CallTimeout: 30 * time.Second, ReadOnlyRetryAttempts: 50,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer func() { _ = client.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	_, err = client.Get(ctx, types.SemanticScopeKey{TenantID: 1, KBID: "kb"}, "op-1")
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("want DEADLINE_EXCEEDED during retry backoff, got %v", err)
	}
}

// TestScopeKeyWireUint64RoundTrip keeps the C01 max-uint64 guarantee alive.
// TestSemanticClientNegativeRetryStillInvokesOnce pins the documented
// "negative disables transparent retries" semantic: the RPC must still run
// EXACTLY ONCE and surface its real error - never silently skip the call.
func TestSemanticClientNegativeRetryStillInvokesOnce(t *testing.T) {
	srv := &testServer{}
	address, stop := startTestServer(t, srv)
	defer stop()
	client, err := NewClient(SemanticClientConfig{
		Address: address, InternalToken: testToken,
		ReadOnlyRetryAttempts: -1,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer func() { _ = client.Close() }()
	if _, err := client.Capabilities(context.Background()); err != nil {
		t.Fatalf("single-attempt capabilities must succeed, got %v", err)
	}
	if srv.capabilitiesCalls != 1 {
		t.Fatalf("negative retries must still invoke once, got %d calls", srv.capabilitiesCalls)
	}
}

func TestScopeKeyWireUint64RoundTrip(t *testing.T) {
	scope := types.SemanticScopeKey{TenantID: math.MaxUint64, KBID: "kb"}
	wire := ScopeKeyToWire(scope)
	parsed := &semanticpb.ScopeKey{}
	if err := unmarshalWire(wire, parsed); err != nil {
		t.Fatalf("wire: %v", err)
	}
	restored, err := ScopeKeyFromWire(parsed)
	if err != nil {
		t.Fatalf("from wire: %v", err)
	}
	if restored.TenantID != math.MaxUint64 {
		t.Fatalf("uint64 lost: %d", restored.TenantID)
	}
}
