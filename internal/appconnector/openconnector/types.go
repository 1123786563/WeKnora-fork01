// Package openconnector implements the WeKnora-side HTTP execution client
// for the pinned open-connector runtime API (POST /v1/actions/{actionId}).
//
// Wire facts consumed here are FROZEN by the T01 runtime-evidence contract
// (docs/integrations/open-connector-contract.md §3, upstream
// @oomol-lab/open-connector v1.5.0 @ 33dd4ad6ee22f9ce5158a1516a11d8b8566b5c8a):
//
//   - success envelope: {"success":true,"message":"OK","data":...,
//     "meta":{"executionId","actionId","auditPersisted"}}
//   - failure envelope: {"success":false,"message","data","errorCode",...} —
//     the error code field is errorCode (NOT code), and executionId /
//     auditPersisted live inside meta (NOT at the top level).
//
// This package only transports and decodes: transport and protocol failures
// surface as Go errors, while business success/failure stays raw in Result.
// Conservative result classification is task T11 and must not happen here.
package openconnector

import (
	"context"
	"encoding/json"
)

// Executor executes exactly one scoped open-connector action call over HTTP:
// a single POST, never retried, never following redirects.
type Executor interface {
	// Execute performs one POST /v1/actions/{call.ActionID} authenticated with
	// token — the scoped runtime token held for exactly one connection. ctx
	// bounds the whole call (context-aware; an earlier caller deadline wins
	// over the client default).
	Execute(ctx context.Context, token string, call Call) (Result, error)
}

// Call is one action invocation against the runtime API.
type Call struct {
	// Alias is forwarded as the x-oo-connector-alias header. Upstream's
	// default alias resolves only the connection literally named "default"
	// with NO fallback to other named connections (contract doc §3.4,
	// fixtures/default_alias.json), so WeKnora always sets the alias
	// explicitly; an empty alias is rejected before any network I/O.
	Alias string
	// ActionID names the action invoked via POST /v1/actions/{ActionID}. It
	// must match ^[A-Za-z0-9_.-]+$.
	ActionID string
	// Key is the Idempotency-Key header value: non-empty (after upstream's
	// trim semantics) and at most 255 UTF-8 bytes.
	Key string
	// Input is the raw JSON action input. It is marshalled into the wire body
	// {"input": ...} EXACTLY, with no other wrapper fields, and must itself be
	// valid JSON.
	Input json.RawMessage
}

// Result is the raw transport-level outcome of one call. Business-result
// classification (which failures are retryable, which are final, how
// meta.auditPersisted=false weighs in) belongs to T11 and is deliberately NOT
// done here.
type Result struct {
	// HTTPStatus is the HTTP status code of the single response received
	// (non-2xx included, returned raw without business interpretation).
	HTTPStatus int
	// Success is the envelope's success field, verbatim.
	Success bool
	// Code maps the upstream failure envelope field errorCode (NOT code).
	// Empty when the envelope carries no errorCode (e.g. success envelopes).
	Code string
	// Message is the envelope's message field, verbatim.
	Message string
	// Data is the envelope's data field as raw JSON, verbatim (may be null).
	Data json.RawMessage
	// ActionID maps meta.actionId from the envelope, verbatim.
	ActionID string
	// ExecutionID maps meta.executionId (success and failure envelopes nest it
	// inside meta — NOT at the top level).
	ExecutionID string
	// AuditPersisted maps meta.auditPersisted as a pointer: nil exactly when
	// the envelope omits it; false means the upstream audit write failed while
	// the action result itself is unchanged (contract doc §3.7,
	// fixtures/audit_failure.json). Never fabricated either way.
	AuditPersisted *bool
}
