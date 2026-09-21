// Pass A compatibility alias for github.com/Tencent/WeKnora/internal/modules/airesource/mcp — zero logic. Deleted by Pass B task B-airesource.
package mcp

import "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"

// Type aliases to the moved package.
type CallToolResult = mcp.CallToolResult
type ClientConfig = mcp.ClientConfig
type ContentItem = mcp.ContentItem
type InitializeResult = mcp.InitializeResult
type MCPClient = mcp.MCPClient
type MCPManager = mcp.MCPManager
type OAuthAttempt = mcp.OAuthAttempt
type OAuthAuthorizationStatus = mcp.OAuthAuthorizationStatus
type OAuthManager = mcp.OAuthManager
type OAuthReauthorizationRequiredError = mcp.OAuthReauthorizationRequiredError
type OAuthRefreshTemporaryError = mcp.OAuthRefreshTemporaryError
type OAuthRequiredError = mcp.OAuthRequiredError
type OAuthState = mcp.OAuthState
type PromptsCapability = mcp.PromptsCapability
type ReadResourceResult = mcp.ReadResourceResult
type ResourceContent = mcp.ResourceContent
type ResourcesCapability = mcp.ResourcesCapability
type ServerCapabilities = mcp.ServerCapabilities
type ServerInfo = mcp.ServerInfo
type ToolsCapability = mcp.ToolsCapability

// Variables forwarded to the moved package.
var ErrAlreadyConnected = mcp.ErrAlreadyConnected
var ErrConnectionClosed = mcp.ErrConnectionClosed
var ErrInitializeFailed = mcp.ErrInitializeFailed
var ErrInvalidResponse = mcp.ErrInvalidResponse
var ErrNotConnected = mcp.ErrNotConnected
var ErrResourceNotFound = mcp.ErrResourceNotFound
var ErrTimeout = mcp.ErrTimeout
var ErrToolNotFound = mcp.ErrToolNotFound
var ErrUnsupportedTransport = mcp.ErrUnsupportedTransport

// Functions forwarded to the moved package (function values; zero logic).
var NewMCPClient = mcp.NewMCPClient
var NewMCPManager = mcp.NewMCPManager
var NewOAuthManager = mcp.NewOAuthManager
var ValidateServiceOutboundURLs = mcp.ValidateServiceOutboundURLs
