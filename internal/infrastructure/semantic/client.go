package semantic

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// SemanticClientConfig configures the Go transport adapter for the semantic
// service. The client dials lazily: a successfully constructed client does
// NOT imply the service is reachable or ready - readiness must be observed
// through Capabilities or health probing.
type SemanticClientConfig struct {
	// Address is the semantic service host:port.
	Address string
	// InternalToken is the approved internal service identity sent on every
	// RPC (x-semantic-token metadata).
	InternalToken string
	// TLSServerName enables TLS verification against the given server name.
	// Empty means plaintext - only acceptable for local tests.
	TLSServerName string
	// RootCAPEM optionally pins a private CA (PEM). With TLSServerName set
	// and RootCAPEM empty, system roots are used.
	RootCAPEM []byte
	// CallTimeout bounds each RPC when the caller context carries no
	// deadline. Zero defaults to 30s.
	CallTimeout time.Duration
	// ReadOnlyRetryAttempts bounds transparent retries for read-only RPCs
	// (Capabilities, Get) on transient Unavailable errors. Zero defaults to
	// 2; a negative value disables transparent retries entirely.
	// Apply/Delete/Search/Reason are NEVER retried here - the business
	// layer owns Apply retry via idempotency keys.
	ReadOnlyRetryAttempts int
}

const tokenMetadataKey = "x-semantic-token"

type grpcClient struct {
	conn          *grpc.ClientConn
	stub          semanticpb.SemanticClient
	token         string
	callTimeout   time.Duration
	retryAttempts int
}

// NewClient builds the transport adapter. It validates the mandatory
// identity fields eagerly and dials lazily.
func NewClient(config SemanticClientConfig) (interfaces.SemanticClient, error) {
	if config.Address == "" {
		return nil, fmt.Errorf("semantic client requires an address")
	}
	if config.InternalToken == "" {
		return nil, fmt.Errorf("semantic client requires an internal token (service identity)")
	}
	var transportCredentials credentials.TransportCredentials
	if config.TLSServerName != "" {
		if len(config.RootCAPEM) > 0 {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(config.RootCAPEM) {
				return nil, fmt.Errorf("semantic client: invalid root CA PEM")
			}
			transportCredentials = credentials.NewClientTLSFromCert(pool, config.TLSServerName)
		} else {
			transportCredentials = credentials.NewClientTLSFromCert(nil, config.TLSServerName)
		}
	} else {
		transportCredentials = insecure.NewCredentials()
	}
	conn, err := grpc.NewClient(config.Address, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		return nil, fmt.Errorf("semantic client dial: %w", err)
	}
	callTimeout := config.CallTimeout
	if callTimeout <= 0 {
		callTimeout = 30 * time.Second
	}
	retries := config.ReadOnlyRetryAttempts
	if retries == 0 {
		retries = 2
	}
	if retries < 0 {
		// "Disabled" means exactly one attempt - never zero invocations.
		retries = 0
	}
	return &grpcClient{
		conn:          conn,
		stub:          semanticpb.NewSemanticClient(conn),
		token:         config.InternalToken,
		callTimeout:   callTimeout,
		retryAttempts: retries,
	}, nil
}

// Close releases the transport. Calling it twice is safe: the second call
// is a no-op (grpc surfaces ErrClientConnClosing which we swallow).
func (c *grpcClient) Close() error {
	if err := c.conn.Close(); err != nil && !errors.Is(err, grpc.ErrClientConnClosing) {
		return err
	}
	return nil
}

// callContext attaches the service identity and a default deadline when the
// caller provided none. Any caller-supplied x-semantic-token values are
// stripped first: the client's own identity is the single value sent, so
// both server implementations can rely on exactly one presented token.
func (c *grpcClient) callContext(ctx context.Context) (context.Context, context.CancelFunc) {
	var outgoing metadata.MD
	if existing, ok := metadata.FromOutgoingContext(ctx); ok {
		outgoing = existing.Copy()
	} else {
		outgoing = metadata.New(nil)
	}
	outgoing.Set(tokenMetadataKey, c.token)
	ctx = metadata.NewOutgoingContext(ctx, outgoing)
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.callTimeout)
}

// retryReadOnly retries transient Unavailable errors for read-only RPCs.
func (c *grpcClient) retryReadOnly(ctx context.Context, invoke func(context.Context) error) error {
	var err error
	for attempt := 0; attempt <= c.retryAttempts; attempt++ {
		err = invoke(ctx)
		if err == nil || status.Code(err) != codes.Unavailable {
			return err
		}
		select {
		case <-ctx.Done():
			// The deadline/cancellation firing during backoff is the real
			// outcome - never mask it with the stale Unavailable.
			return status.FromContextError(ctx.Err()).Err()
		case <-time.After(time.Duration(attempt+1) * 20 * time.Millisecond):
		}
	}
	return err
}

func (c *grpcClient) Capabilities(ctx context.Context) (types.SemanticCapabilities, error) {
	callCtx, cancel := c.callContext(ctx)
	defer cancel()
	var response *semanticpb.GetCapabilitiesResponse
	err := c.retryReadOnly(callCtx, func(ctx context.Context) error {
		wire, callErr := c.stub.GetCapabilities(ctx, &semanticpb.GetCapabilitiesRequest{})
		if callErr != nil {
			return callErr
		}
		response = wire
		return nil
	})
	if err != nil {
		return types.SemanticCapabilities{}, err
	}
	return CapabilitiesFromWire(response)
}

func (c *grpcClient) Apply(ctx context.Context, request types.SemanticApplyRequest) (types.SemanticOperation, error) {
	callCtx, cancel := c.callContext(ctx)
	defer cancel()
	wire, err := ApplyRequestToWire(request)
	if err != nil {
		return types.SemanticOperation{}, err
	}
	response, err := c.stub.ApplyDocumentRevision(callCtx, &semanticpb.ApplyDocumentRevisionRequest{Apply: wire})
	if err != nil {
		return types.SemanticOperation{}, err
	}
	return OperationFromWire(response)
}

func (c *grpcClient) Delete(ctx context.Context, revision types.SemanticDocumentRevision) (types.SemanticOperation, error) {
	callCtx, cancel := c.callContext(ctx)
	defer cancel()
	response, err := c.stub.DeleteDocument(callCtx, &semanticpb.DeleteDocumentRequest{
		Document: DocumentRevisionToWire(revision),
	})
	if err != nil {
		return types.SemanticOperation{}, err
	}
	return OperationFromWire(response)
}

func (c *grpcClient) Get(ctx context.Context, scope types.SemanticScopeKey, operationID string) (types.SemanticOperation, error) {
	callCtx, cancel := c.callContext(ctx)
	defer cancel()
	var response *semanticpb.Operation
	err := c.retryReadOnly(callCtx, func(ctx context.Context) error {
		wire, callErr := c.stub.GetOperation(ctx, &semanticpb.GetOperationRequest{
			Scope:       ScopeKeyToWire(scope),
			OperationId: operationID,
		})
		if callErr != nil {
			return callErr
		}
		response = wire
		return nil
	})
	if err != nil {
		return types.SemanticOperation{}, err
	}
	return OperationFromWire(response)
}

func (c *grpcClient) Cancel(ctx context.Context, scope types.SemanticScopeKey, operationID string) (types.SemanticOperation, error) {
	callCtx, cancel := c.callContext(ctx)
	defer cancel()
	response, err := c.stub.CancelOperation(callCtx, &semanticpb.CancelOperationRequest{
		Scope:       ScopeKeyToWire(scope),
		OperationId: operationID,
	})
	if err != nil {
		return types.SemanticOperation{}, err
	}
	return OperationFromWire(response)
}

func (c *grpcClient) Search(ctx context.Context, request types.SemanticSearchRequest) (types.SemanticSearchResponse, error) {
	callCtx, cancel := c.callContext(ctx)
	defer cancel()
	wire, err := SearchRequestToWire(request, nil)
	if err != nil {
		return types.SemanticSearchResponse{}, err
	}
	response, err := c.stub.Search(callCtx, wire)
	if err != nil {
		return types.SemanticSearchResponse{}, err
	}
	return SearchResponseFromWire(response)
}

func (c *grpcClient) Reason(ctx context.Context, request types.SemanticReasonRequest) (types.SemanticReasonResponse, error) {
	callCtx, cancel := c.callContext(ctx)
	defer cancel()
	wire, err := ReasonRequestToWire(request, nil)
	if err != nil {
		return types.SemanticReasonResponse{}, err
	}
	response, err := c.stub.Reason(callCtx, wire)
	if err != nil {
		return types.SemanticReasonResponse{}, err
	}
	return ReasonResponseFromWire(response)
}
