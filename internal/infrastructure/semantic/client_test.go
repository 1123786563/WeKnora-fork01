package semantic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type testSemanticServer struct {
	semanticpb.UnimplementedSemanticServiceServer
	searchStarted chan struct{}
	seenMetadata  chan metadata.MD
}

func (s *testSemanticServer) GetCapabilities(ctx context.Context, _ *semanticpb.Empty) (*semanticpb.Capabilities, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.seenMetadata <- md
	return &semanticpb.Capabilities{ProtocolVersion: "v1", EngineVersion: "test", Limits: &semanticpb.QueryLimits{}}, nil
}

func (s *testSemanticServer) Search(ctx context.Context, _ *semanticpb.SearchRequest) (*semanticpb.SearchResponse, error) {
	s.searchStarted <- struct{}{}
	<-ctx.Done()
	return nil, status.FromContextError(ctx.Err()).Err()
}

func testClientConfig(t *testing.T) (SemanticClientConfig, *testSemanticServer, func()) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{Raw: der}
	pool := x509.NewCertPool()
	pool.AddCert(certificate)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	serverTLS := credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serverImpl := &testSemanticServer{searchStarted: make(chan struct{}, 1), seenMetadata: make(chan metadata.MD, 1)}
	server := grpc.NewServer(grpc.Creds(serverTLS))
	semanticpb.RegisterSemanticServiceServer(server, serverImpl)
	go func() { _ = server.Serve(listener) }()
	config := SemanticClientConfig{Address: listener.Addr().String(), ServerName: "localhost", RootCA: certPEM, ServiceToken: "test-token", Audience: "weknora-semantic"}
	return config, serverImpl, func() { server.Stop(); _ = listener.Close() }
}

func TestClientInjectsApplicationTraceAndRequestID(t *testing.T) {
	config, server, stop := testClientConfig(t)
	defer stop()
	client, err := NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	tracerProvider := sdktrace.NewTracerProvider()
	defer func() { _ = tracerProvider.Shutdown(context.Background()) }()
	ctx, span := tracerProvider.Tracer("semantic-client-test").Start(context.Background(), "application-call")
	defer span.End()
	expectedTraceparent := "00-" + span.SpanContext().TraceID().String() + "-" + span.SpanContext().SpanID().String() + "-01"
	ctx = context.WithValue(ctx, types.RequestIDContextKey, "request-123")
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-existing", "preserved"))
	capabilities, err := client.GetCapabilities(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if capabilities.EngineVersion != "test" {
		t.Fatalf("engine version = %q", capabilities.EngineVersion)
	}
	md := <-server.seenMetadata
	if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer test-token" {
		t.Fatalf("authorization metadata = %v", got)
	}
	if got := md.Get("x-weknora-audience"); len(got) != 1 || got[0] != "weknora-semantic" {
		t.Fatalf("audience metadata = %v", got)
	}
	if got := md.Get("traceparent"); len(got) != 1 || got[0] != expectedTraceparent {
		t.Fatalf("trace metadata = %v", got)
	}
	if got := md.Get("x-request-id"); len(got) != 1 || got[0] != "request-123" {
		t.Fatalf("request ID metadata = %v", got)
	}
	if got := md.Get("x-existing"); len(got) != 1 || got[0] != "preserved" {
		t.Fatalf("existing outgoing metadata = %v", got)
	}
}

func TestClientMapsUnimplementedStatus(t *testing.T) {
	config, _, stop := testClientConfig(t)
	defer stop()
	client, err := NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	scope := types.SemanticAccessScope{Scope: types.SemanticScopeKey{TenantID: 1, KBID: "kb"}, SubjectID: "u", ScopeRef: "r", ScopeHash: "h", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Audience: "weknora-semantic", Purpose: types.SemanticAccessPurposeReason, BudgetRef: "b"}
	request := types.SemanticReasonRequest{Search: types.SemanticSearchRequest{QueryID: "q", Query: "q", AccessScope: scope, RequestedMode: types.SemanticRetrievalModeReason}, ReasoningMode: types.SemanticReasoningModeRules, RuleSetVersion: "v1"}
	_, err = client.Reason(context.Background(), request)
	if rpcErr, ok := err.(*SemanticRPCError); !ok || rpcErr.Code != codes.Unimplemented {
		t.Fatalf("reason error = %#v", err)
	}
}

func TestClientSearchHonorsDeadline(t *testing.T) {
	config, _, stop := testClientConfig(t)
	defer stop()
	client, err := NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	scope := types.SemanticAccessScope{Scope: types.SemanticScopeKey{TenantID: 1, KBID: "kb"}, SubjectID: "u", ScopeRef: "r", ScopeHash: "h", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Audience: "weknora-semantic", Purpose: types.SemanticAccessPurposeSearch, BudgetRef: "b"}
	_, err = client.Search(ctx, types.SemanticSearchRequest{QueryID: "q", Query: "q", AccessScope: scope, RequestedMode: types.SemanticRetrievalModeGraphRAG})
	if rpcErr, ok := err.(*SemanticRPCError); !ok || rpcErr.Code != codes.DeadlineExceeded {
		t.Fatalf("search error = %#v", err)
	}
}

func TestClientSearchHonorsCallerCancellation(t *testing.T) {
	config, serverImpl, stop := testClientConfig(t)
	defer stop()
	client, err := NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	scope := types.SemanticAccessScope{Scope: types.SemanticScopeKey{TenantID: 1, KBID: "kb"}, SubjectID: "u", ScopeRef: "r", ScopeHash: "h", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Audience: "weknora-semantic", Purpose: types.SemanticAccessPurposeSearch, BudgetRef: "b"}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, callErr := client.Search(ctx, types.SemanticSearchRequest{QueryID: "q", Query: "q", AccessScope: scope, RequestedMode: types.SemanticRetrievalModeGraphRAG})
		result <- callErr
	}()
	<-serverImpl.searchStarted
	cancel()
	if callErr := <-result; callErr == nil {
		t.Fatal("expected cancellation error")
	} else if rpcErr, ok := callErr.(*SemanticRPCError); !ok || rpcErr.Code != codes.Canceled {
		t.Fatalf("search error = %#v", callErr)
	}
}

func TestNewClientRequiresAuthenticatedTLSConfiguration(t *testing.T) {
	if _, err := NewClient(SemanticClientConfig{}); err == nil {
		t.Fatal("expected incomplete client config rejection")
	}
}

func TestClientMapsConnectionFailure(t *testing.T) {
	config, _, stop := testClientConfig(t)
	stop()
	config.Address = "127.0.0.1:1"
	client, err := NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = client.GetCapabilities(ctx)
	if rpcErr, ok := err.(*SemanticRPCError); !ok || (rpcErr.Code != codes.Unavailable && rpcErr.Code != codes.DeadlineExceeded) {
		t.Fatalf("connection error = %#v", err)
	}
}
