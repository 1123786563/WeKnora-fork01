package repository

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// ScopeFromContext derives Career ownership exclusively from auth middleware.
func ScopeFromContext(ctx context.Context) (Scope, error) {
	tenantID, tenantOK := types.TenantIDFromContext(ctx)
	ownerID, ownerOK := types.UserIDFromContext(ctx)
	principal, principalOK := types.PrincipalFromContext(ctx)
	if !tenantOK || !ownerOK || !principalOK || tenantID == 0 || ownerID == "" || principal.ID != ownerID {
		return Scope{}, ErrUnauthorized
	}
	scope := Scope{TenantID: tenantID, OwnerID: ownerID}
	return scope, scope.Validate()
}
