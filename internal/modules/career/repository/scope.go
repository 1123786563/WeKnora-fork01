package repository

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// ScopeFromContext derives Career ownership exclusively from auth middleware.
func ScopeFromContext(ctx context.Context) (Scope, error) {
	caller, callerOK := ctx.Value(types.CallerContextKey).(types.Caller)
	caller = types.CallerFromContext(ctx)
	executionTenant, tenantOK := types.TenantIDFromContext(ctx)
	ownerID, ownerOK := types.UserIDFromContext(ctx)
	principal, principalOK := types.PrincipalFromContext(ctx)
	if !callerOK || !tenantOK || !ownerOK || !principalOK || caller.TenantID == 0 || caller.UserID == "" || executionTenant != caller.TenantID || ownerID != caller.UserID || principal.Type != types.PrincipalWebUser || principal.ID != caller.UserID {
		return Scope{}, ErrUnauthorized
	}
	scope := Scope{TenantID: caller.TenantID, OwnerID: caller.UserID}
	return scope, scope.Validate()
}
