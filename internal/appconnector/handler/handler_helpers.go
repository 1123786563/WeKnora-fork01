package handler

import "errors"

// ErrMissingTenantScope is the module-local copy of the host sentinel
// defined at internal/handler/commercial.go:34 (Pass B 27-appconnector):
// the message matches verbatim so the 403 MISSING_TENANT_SCOPE responses
// issued by appTenantScope/appRequireWriteCapability stay byte-identical.
// The consolidation ruling for this copy, the host original and the future
// 12-commercial modules/commercial/handler copy is registered in
// docs/architecture/passb/briefs/b2-appconnector.md for IB2 to settle as a
// single implementation.
var ErrMissingTenantScope = errors.New("missing_tenant_scope")
