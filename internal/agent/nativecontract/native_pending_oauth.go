package nativecontract

import (
	"context"
	"time"
)

// PendingOAuthBinding is server-owned authority, never a client request DTO.
// Session ownership and OAuth principal are deliberately separate identities.
type PendingOAuthBinding struct {
	Key                                                                     PendingKey
	Principal                                                               Principal
	SessionOwnerID, ServiceID, InstallationID, PendingRevision, RedirectURI string
	ExpiresAt                                                               time.Time
}

// PendingOAuthAttempt is private authority data. Never serialize it as pending
// detail, events, or logs: State is a one-shot secret, not an OAuth status.
type PendingOAuthAttempt struct {
	Binding                     PendingOAuthBinding
	AuthorizationAttempt, State string
}

type PendingOAuthCallback struct {
	AuthorizationAttempt, State, Receipt string
}

// PendingOAuthVerificationResult is safe callback metadata. It does not expose
// raw state/proof, hashes, principal credentials, or an authorization decision.
type PendingOAuthVerificationResult struct {
	PendingID            string    `json:"pending_id"`
	AuthorizationAttempt string    `json:"authorization_attempt"`
	ServiceID            string    `json:"service_id"`
	Revision             string    `json:"revision"`
	VerifiedAt           time.Time `json:"verified_at"`
}

// PendingOAuthReceipt proves only callback verification. It does not prove a
// live token or authorize a pending decision, tool approval, or dispatch.
type PendingOAuthReceipt struct {
	Binding              PendingOAuthBinding
	AuthorizationAttempt string
}

// PendingOAuthReceiptVerifier authenticates provider proof and returns the
// identity it actually covers. Implementations must not infer proof from the
// expected attempt, popup completion, or a client-supplied success flag.
type PendingOAuthReceiptVerifier interface {
	Verify(context.Context, PendingOAuthAttempt, string) (PendingOAuthReceipt, error)
}

// PendingOAuthProvider is intentionally unimplemented for production. Prepare
// must not authorize execution; returned secrets stay inside the authority seam.
type PendingOAuthProvider interface {
	Prepare(context.Context, PendingOAuthBinding) (OAuthStartResult, string, error)
	PendingOAuthReceiptVerifier
}
