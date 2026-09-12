package approval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/Tencent/WeKnora/internal/types"
)

// ErrToolDefinitionChanged marks an advertised MCP tool that no longer
// matches its admin-pinned version (input schema or risk). Callers must
// park the installation in pending re-review instead of executing it.
var ErrToolDefinitionChanged = errors.New("mcp_tool_definition_changed")

// ToolPin is the admin-reviewed version pin of one MCP tool: the digest of
// its canonical input schema plus its risk category. External readOnly
// hints are advisory metadata and are intentionally not pinned.
type ToolPin struct {
	ToolName     string
	SchemaDigest string
	Risk         string
}

// PinToolDigest fingerprints one reviewed tool definition. Identical
// canonical schemas with identical risk produce identical digests.
func PinToolDigest(canonicalInputSchema string, risk string) string {
	h := sha256.New()
	h.Write([]byte(canonicalInputSchema))
	h.Write([]byte{0})
	h.Write([]byte(risk))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyToolPin compares one advertised tool against its pin; any schema or
// risk drift wraps ErrToolDefinitionChanged. This is the version-pinned
// schema/risk integration point the connector-side MCP adapter reports
// through: the enabled-tools policy above stays a pure allowlist, while
// this check decides reviewed-version equality.
func VerifyToolPin(pin ToolPin, advertisedName, canonicalInputSchema, risk string) error {
	if advertisedName != pin.ToolName {
		return fmt.Errorf("%w: advertised %q, pinned %q", ErrToolDefinitionChanged, advertisedName, pin.ToolName)
	}
	if PinToolDigest(canonicalInputSchema, risk) != pin.SchemaDigest {
		return fmt.Errorf("%w: tool %q schema/risk digest changed", ErrToolDefinitionChanged, pin.ToolName)
	}
	return nil
}

type enabledChecker interface {
	IsEnabled(context.Context, uint64, string, string) (bool, error)
}

// BulkEnabledChecker avoids a query per tool when enumerating a directory.
// Implementations return an explicit decision for each requested name.
type BulkEnabledChecker interface {
	EnabledTools(context.Context, uint64, string, []string) (map[string]bool, error)
}

// EnabledTools uses batch policies where available while retaining compatibility
// with custom gates implementing only the original single-tool contract.
func EnabledTools(
	ctx context.Context,
	checker enabledChecker,
	tenantID uint64,
	serviceID string,
	names []string,
) (map[string]bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tenantID == 0 || serviceID == "" {
		return nil, fmt.Errorf("MCP policy identity is required")
	}
	if bulk, ok := checker.(BulkEnabledChecker); ok {
		return bulk.EnabledTools(ctx, tenantID, serviceID, names)
	}
	return enabledToolsIndividually(ctx, checker, tenantID, serviceID, names)
}

func enabledToolsIndividually(
	ctx context.Context,
	checker enabledChecker,
	tenantID uint64,
	serviceID string,
	names []string,
) (map[string]bool, error) {
	result := make(map[string]bool, len(names))
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		enabled := true
		if checker != nil {
			var err error
			enabled, err = checker.IsEnabled(ctx, tenantID, serviceID, name)
			if err != nil {
				return nil, err
			}
		}
		result[name] = enabled
	}
	return result, nil
}

// EnabledTools delegates directory policy checks to the configured checker.
func (g *Gate) EnabledTools(
	ctx context.Context,
	tenantID uint64,
	serviceID string,
	names []string,
) (map[string]bool, error) {
	if g == nil {
		return EnabledTools(ctx, nil, tenantID, serviceID, names)
	}
	return EnabledTools(ctx, g.checker, tenantID, serviceID, names)
}

// EnabledTools reads service policies in one batch when supported by the service.
func (a *Adapter) EnabledTools(
	ctx context.Context,
	tenantID uint64,
	serviceID string,
	names []string,
) (map[string]bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tenantID == 0 || serviceID == "" {
		return nil, fmt.Errorf("MCP policy identity is required")
	}
	if a == nil || a.Svc == nil {
		return enabledToolsIndividually(ctx, nil, tenantID, serviceID, names)
	}
	lister, ok := a.Svc.(interface {
		ListByService(context.Context, uint64, string) ([]*types.MCPToolApproval, error)
	})
	if !ok {
		return enabledToolsIndividually(ctx, a, tenantID, serviceID, names)
	}
	rows, err := lister.ListByService(ctx, tenantID, serviceID)
	if err != nil {
		return nil, err
	}
	result, err := enabledToolsIndividually(ctx, nil, tenantID, serviceID, names)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row == nil || row.TenantID != tenantID || row.ServiceID != serviceID {
			continue
		}
		if _, requested := result[row.ToolName]; requested {
			result[row.ToolName] = row.Enabled
		}
	}
	return result, nil
}
