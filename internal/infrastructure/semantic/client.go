package semantic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type SemanticClientConfig struct {
	Address      string
	ServerName   string
	RootCA       []byte
	ServiceToken string
	Audience     string
}

type SemanticRPCError struct {
	Code    codes.Code
	Message string
}

func (e *SemanticRPCError) Error() string {
	return fmt.Sprintf("semantic RPC %s: %s", e.Code, e.Message)
}

type serviceIdentityCredentials struct{ token, audience string }

func (c serviceIdentityCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + c.token, "x-weknora-audience": c.audience}, nil
}
func (serviceIdentityCredentials) RequireTransportSecurity() bool { return true }

type semanticClient struct {
	connection *grpc.ClientConn
	wire       semanticpb.SemanticServiceClient
}

var _ interfaces.SemanticClient = (*semanticClient)(nil)

func NewClient(config SemanticClientConfig) (interfaces.SemanticClient, error) {
	if strings.TrimSpace(config.Address) == "" || strings.TrimSpace(config.ServerName) == "" || len(config.RootCA) == 0 || config.ServiceToken == "" || config.Audience == "" {
		return nil, fmt.Errorf("semantic client requires address, TLS server name, root CA, service token, and audience")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(config.RootCA) {
		return nil, fmt.Errorf("semantic client root CA bundle contains no certificates")
	}
	transport := credentials.NewTLS(&tls.Config{RootCAs: roots, ServerName: config.ServerName, MinVersion: tls.VersionTLS12})
	connection, err := grpc.NewClient(config.Address,
		grpc.WithTransportCredentials(transport),
		grpc.WithPerRPCCredentials(serviceIdentityCredentials{token: config.ServiceToken, audience: config.Audience}),
		grpc.WithChainUnaryInterceptor(traceMetadataInterceptor),
	)
	if err != nil {
		return nil, fmt.Errorf("create semantic gRPC client: %w", err)
	}
	return &semanticClient{connection: connection, wire: semanticpb.NewSemanticServiceClient(connection)}, nil
}

func traceMetadataInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var outgoing metadata.MD
	if existing, ok := metadata.FromOutgoingContext(ctx); ok {
		outgoing = existing.Copy()
	} else {
		outgoing = metadata.MD{}
	}
	if incoming, ok := metadata.FromIncomingContext(ctx); ok {
		for _, key := range []string{"traceparent", "tracestate", "baggage"} {
			if values := incoming.Get(key); len(values) > 0 {
				outgoing.Set(key, values...)
			}
		}
	}
	return invoker(metadata.NewOutgoingContext(ctx, outgoing), method, req, reply, cc, opts...)
}

func mapRPCError(err error) error {
	if err == nil {
		return nil
	}
	if rpcStatus, ok := status.FromError(err); ok {
		return &SemanticRPCError{Code: rpcStatus.Code(), Message: rpcStatus.Message()}
	}
	return err
}

func (c *semanticClient) Close() error { return c.connection.Close() }

func (c *semanticClient) GetCapabilities(ctx context.Context) (*types.SemanticCapabilities, error) {
	response, err := c.wire.GetCapabilities(ctx, &semanticpb.Empty{})
	if err != nil {
		return nil, mapRPCError(err)
	}
	result, err := types.SemanticCapabilitiesFromWire(response)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *semanticClient) ApplyDocumentRevision(ctx context.Context, request types.SemanticApplyRequest) (*types.SemanticOperation, error) {
	wireRequest, err := types.SemanticApplyRequestToWire(request)
	if err != nil {
		return nil, err
	}
	response, err := c.wire.ApplyDocumentRevision(ctx, wireRequest)
	if err != nil {
		return nil, mapRPCError(err)
	}
	result, err := types.SemanticOperationFromWire(response)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *semanticClient) DeleteDocument(ctx context.Context, revision types.SemanticDocumentRevision) (*types.SemanticOperation, error) {
	wireRequest, err := types.SemanticDeleteDocumentRevisionToWire(revision)
	if err != nil {
		return nil, err
	}
	response, err := c.wire.DeleteDocument(ctx, wireRequest)
	if err != nil {
		return nil, mapRPCError(err)
	}
	result, err := types.SemanticOperationFromWire(response)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *semanticClient) GetOperation(ctx context.Context, reference types.SemanticOperationRef) (*types.SemanticOperation, error) {
	wireRequest, err := types.SemanticOperationRefToWire(reference)
	if err != nil {
		return nil, err
	}
	response, err := c.wire.GetOperation(ctx, wireRequest)
	if err != nil {
		return nil, mapRPCError(err)
	}
	result, err := types.SemanticOperationFromWire(response)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *semanticClient) CancelOperation(ctx context.Context, reference types.SemanticOperationRef) (*types.SemanticOperation, error) {
	wireRequest, err := types.SemanticOperationRefToWire(reference)
	if err != nil {
		return nil, err
	}
	response, err := c.wire.CancelOperation(ctx, wireRequest)
	if err != nil {
		return nil, mapRPCError(err)
	}
	result, err := types.SemanticOperationFromWire(response)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *semanticClient) Search(ctx context.Context, request types.SemanticSearchRequest) (*types.SemanticSearchResponse, error) {
	wireRequest, err := types.SemanticSearchRequestToWire(request)
	if err != nil {
		return nil, err
	}
	response, err := c.wire.Search(ctx, wireRequest)
	if err != nil {
		return nil, mapRPCError(err)
	}
	result, err := types.SemanticSearchResponseFromWire(response)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *semanticClient) Reason(ctx context.Context, request types.SemanticReasonRequest) (*types.SemanticReasonResponse, error) {
	wireRequest, err := types.SemanticReasonRequestToWire(request)
	if err != nil {
		return nil, err
	}
	response, err := c.wire.Reason(ctx, wireRequest)
	if err != nil {
		return nil, mapRPCError(err)
	}
	result, err := types.SemanticReasonResponseFromWire(response)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
