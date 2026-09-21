package interfaces

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
)

// SemanticScopeInvalidator is the pre-write barrier for KB-relevant ACL changes.
// Errors must abort the following mutation; a later mutation failure may safely
// leave a conservatively advanced epoch.
type SemanticScopeInvalidator interface {
	InvalidateTenant(context.Context, uint64) error
	InvalidateUser(context.Context, string) error
	InvalidateOrganization(context.Context, string) error
	InvalidateKB(context.Context, uint64, string) error
	InvalidateTransfer(context.Context, types.SemanticScopeKey, types.SemanticScopeKey) error
}
