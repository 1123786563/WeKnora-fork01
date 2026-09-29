package codedelivery

import (
	"context"
	"encoding/json"
	"errors"
)

type ActionSubject struct {
	TenantID uint64
	ActorID  string
}
type ActionInput struct {
	TenantID                            uint64
	ActorID, ConnectionID, Target, Risk string
	AuthVersion                         int64
	Args                                json.RawMessage
}
type ConnectionIdentity struct {
	ID, InstallationID, Kind, OwnerID, State string
	TenantID                                 uint64
	AuthVersion                              int64
}
type ActionLifecycle interface {
	Prepare(context.Context, ActionInput) (string, error)
	Execute(context.Context, string) error
	ResolveUnknown(context.Context, string) error
}
type RunIdentity struct{ SessionID string }
type ProviderInstallation struct{ AppID string }
type RunReader interface {
	GetOwnedRun(context.Context, uint64, string, string) (RunIdentity, error)
}
type ProviderSource interface {
	GetInstallationByID(context.Context, uint64, string) (ProviderInstallation, error)
}
type ActionRecord struct {
	ID                                                                 string
	TenantID                                                           uint64
	ActorID, ConnectionID, AppVersion, Target, Risk, ArgsDigest, State string
	Fence                                                              int64
	ArgsSnapshot                                                       string
	AuthVersion                                                        int64
	DigestVersion                                                      int
	ProviderResult                                                     string
}
type ActionStoreSource interface {
	FindAction(context.Context, string) (ActionRecord, error)
}
type CredentialResolver interface {
	Resolve(context.Context, string, int64) ([]byte, error)
}
type A02Guard interface {
	Check(context.Context, ActionSubject, string, int64) error
}
type ActionSnapshot struct {
	ID                            string
	TenantID                      uint64
	ActorID, ConnectionID, Target string
	AuthVersion                   int64
	Args                          json.RawMessage
}
type DispatchOutcome struct{ Status, ProviderResult string }
type ActionDispatcher interface {
	Dispatch(context.Context, ActionSnapshot, string) (DispatchOutcome, error)
	QueryProvider(context.Context, ActionSnapshot, string) (DispatchOutcome, error)
}

var ErrDispatchNotStarted = errors.New("code_delivery_dispatch_not_started")
var ErrDispatchUnknown = errors.New("code_delivery_dispatch_unknown")
