package service

import (
	"github.com/Tencent/WeKnora/internal/types"
)

// MarketHostAdapters 承接旧宿主包内仍留驻的 agentruntime 能力位（25b HostAdapters
// 同型；IB2/B3 按 re-export 裁定收口）。字段语义与被替换符号 1:1，nil 即 fail-fast。
//
// remove_at: ib2 — 对端符号 re-export 后逐位删除。
type MarketHostAdapters struct {
	// BuildReleaseBundle 替代 experts.BuildAgentReleaseBundle（agent_release.go:64，
	// 纯函数：无盘无网）。宿主残差构造器绑真源 experts.BuildAgentReleaseBundle。
	BuildReleaseBundle func(version types.AgentVersionSnapshot, metadata types.ReleaseMetadata, lock types.DependencyLock) (types.AgentReleaseBundle, error)
}
