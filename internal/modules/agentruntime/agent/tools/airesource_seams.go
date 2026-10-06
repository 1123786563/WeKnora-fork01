package tools

// R2 消费侧 seam（owner: 32-agentruntime-tools，remove_at: ib3）。
// 本文件按 32 计划 §0.4/§0.5 收敛 tools 包对 airesource 模块 mcp、
// models/rerank、models/chat 三个子包的深 import：仅允许类型别名 /
// 常量别名 / 薄委托三种无逻辑形态（spec §4.4 过渡薄别名），单一真源
// 仍在 airesource 模块内；ib3 门面合法化契约任务落地后翻转本文件
// import 至模块根并随之删除。
// 例外登记：guard importExceptions 3 行 + exception-ledger exc-0145/0146/0147
// （Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）。

import (
	"github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/rerank"
)

// ---- 类型别名（mcp）----

// MCPManager R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type MCPManager = mcp.MCPManager // mcp/manager.go:16

// MCPClient R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type MCPClient = mcp.MCPClient // mcp/client.go:25（接口）

// ContentItem R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type ContentItem = mcp.ContentItem // mcp/types.go:51

// CallToolResult R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type CallToolResult = mcp.CallToolResult // mcp/types.go:45

// OAuthReauthorizationRequiredError R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type OAuthReauthorizationRequiredError = mcp.OAuthReauthorizationRequiredError // mcp/oauth_lifecycle.go:39

// ---- 类型别名（rerank）----

// Reranker R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type Reranker = rerank.Reranker // rerank/reranker.go:14（接口）

// RankResult R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type RankResult = rerank.RankResult // rerank/reranker.go:25

// ---- 类型别名（chat）----

// Message R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type Message = chat.Message // chat/chat.go:79
