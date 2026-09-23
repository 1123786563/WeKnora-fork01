package container

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
)

// NewPluginMCPEndpointLister verifies plugin endpoints through the production
// MCP client stack. It lives in the composition root because
// internal/modules 之间禁止互相 import（architectureguard forbidden-import）：
// plugins 模块定义 EndpointLister/ErrOAuthProtectedEndpoint 契约，本适配器在此
// 把 airesource 的 MCPManager 粘合到该契约上。
//
// Each verification gets a UNIQUE ephemeral service ID (URL hash + random
// nonce): the manager caches connections by service ID, so a deterministic ID
// would make two concurrent verifications of the same endpoint share one
// client — and the first finisher's Disconnect would tear down the other's
// in-flight ListTools (false-negative preview rejection). Uniqueness buys
// each verification an exclusive connection and Disconnect scope;
// CloseClient retires the entry immediately instead of leaving a disconnected
// corpse for the idle-cleanup goroutine.
func NewPluginMCPEndpointLister(manager *mcp.MCPManager) plugins.EndpointLister {
	return func(ctx context.Context, transportType, endpointURL string) ([]*types.MCPTool, error) {
		if manager == nil {
			return nil, fmt.Errorf("mcp manager is required")
		}
		var nonce [4]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, fmt.Errorf("generate verification nonce: %w", err)
		}
		sum := sha256.Sum256([]byte(endpointURL))
		serviceID := "plugin-verify-" + hex.EncodeToString(sum[:8]) + "-" + hex.EncodeToString(nonce[:])
		service := &types.MCPService{
			ID:            serviceID,
			TenantID:      0,
			Name:          "plugin-verify",
			Enabled:       true,
			TransportType: types.MCPTransportType(transportType),
			URL:           &endpointURL,
		}
		client, err := manager.GetOrCreateClient(ctx, service)
		if err != nil {
			// The caller's ctx may expire (or the handshake fail) while the
			// manager's background goroutine — on its own lifeCtx — is still
			// connecting; a client that then finishes Connect/Initialize is
			// mounted under this globally unique nonce key, which nothing will
			// ever reference again, and idle cleanup only removes
			// !IsConnected() entries. Retire the key on the error path too:
			// CloseClient cancels a pending connection and disconnects an
			// already-mounted one.
			_ = manager.CloseClient(service.ID)
			return nil, wrapPluginOAuth(err)
		}
		defer func() {
			_ = client.Disconnect()
			_ = manager.CloseClient(service.ID)
		}()
		tools, err := client.ListTools(ctx)
		if err != nil {
			return nil, wrapPluginOAuth(err)
		}
		return tools, nil
	}
}

// wrapPluginOAuth maps the MCP 层的 *OAuthRequiredError onto the plugins 模块
// 的 ErrOAuthProtectedEndpoint 哨兵：endpoint 以 401 + RFC 9728 元数据回应握手。
// 双 %w 同时保留两个身份——调用方既能 errors.Is 哨兵（plugins.IsOAuthProtected），
// 也能 errors.As 取回底层 *mcp.OAuthRequiredError。非 OAuth 错误原样返回。
func wrapPluginOAuth(err error) error {
	var oauthErr *mcp.OAuthRequiredError
	if errors.As(err, &oauthErr) {
		return fmt.Errorf("%w: %w", plugins.ErrOAuthProtectedEndpoint, err)
	}
	return err
}
