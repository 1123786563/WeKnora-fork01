package tools

// R2 消费侧 seam（owner: 32-agentruntime-tools，remove_at: ib3）。
// 本文件按 32 计划 §0.4/§0.5 收敛 tools 包对 execution 模块 sandbox 与
// browserskill 两个子包的深 import：仅允许类型别名 / 常量别名 / 薄委托三种
// 无逻辑形态（spec §4.4 过渡薄别名），单一真源仍在 execution 模块内；
// ib3 门面合法化契约任务落地后翻转本文件 import 至模块根并随之删除。
// 例外登记：guard importExceptions 2 行 + exception-ledger exc-0143/0144
// （Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）。

import (
	"context"
	"encoding/json"

	"github.com/Tencent/WeKnora/internal/modules/execution/browserskill"
	"github.com/Tencent/WeKnora/internal/sandbox"
)

// ---- 类型别名（sandbox）----

// SessionBoundManager R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type SessionBoundManager = sandbox.SessionBoundManager // sandbox/session_manager.go:77

// ExecuteResult R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type ExecuteResult = sandbox.ExecuteResult // sandbox/sandbox.go:219

// ShellExecOptions R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type ShellExecOptions = sandbox.ShellExecOptions // sandbox/session_manager.go:709

// ShellOutputSnapshot R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type ShellOutputSnapshot = sandbox.ShellOutputSnapshot // sandbox/session_manager.go:768

// RemoteDirEntry R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type RemoteDirEntry = sandbox.RemoteDirEntry // sandbox/remote_client.go:370

// RemoteStatEntry R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type RemoteStatEntry = sandbox.RemoteStatEntry // sandbox/remote_client.go:390

// SessionInstallShellExecutor R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type SessionInstallShellExecutor = sandbox.SessionInstallShellExecutor // sandbox/capabilities.go:96（接口）

// ---- 类型别名（browserskill）----

// Manager R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type Manager = browserskill.Manager // browserskill/manager.go:99

// Scope R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type Scope = browserskill.Scope // browserskill/manager.go:34

// Status R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type Status = browserskill.Status // browserskill/manager.go:46

// AccountStatus R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type AccountStatus = browserskill.AccountStatus // browserskill/authorization.go:158

// RPCError R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
type RPCError = browserskill.RPCError // browserskill/errors.go:7

// ---- 常量别名（sandbox）----

// SessionWorkspaceRoot R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
const SessionWorkspaceRoot = sandbox.SessionWorkspaceRoot // "/workspace"，session_manager.go:61

// SessionInputRoot R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
const SessionInputRoot = sandbox.SessionInputRoot // "/workspace/input"，session_manager.go:42

// SessionOutputRoot R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
const SessionOutputRoot = sandbox.SessionOutputRoot // "/workspace/output"，session_manager.go:48

// SkillsImageRoot R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
const SkillsImageRoot = sandbox.SkillsImageRoot // skill_paths.go:14

// RemoteEntryFile R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
// （R2.1 实测补入：sandbox_ls.go 消费，32 计划 §0.8-A 勘误，见 R2.2 报告）
const RemoteEntryFile = sandbox.RemoteEntryFile // remote_client.go:382

// RemoteEntryDir R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
// （R2.1 实测补入：sandbox_ls.go 消费，32 计划 §0.8-A 勘误，见 R2.2 报告）
const RemoteEntryDir = sandbox.RemoteEntryDir // remote_client.go:383

// ---- 变量别名（sandbox）----

// ErrTimeout R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
var ErrTimeout = sandbox.ErrTimeout // sandbox.go:101

// ---- 薄委托函数（单一真源，签名逐字取自源）----

// ResolveWorkspacePath R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func ResolveWorkspacePath(value string) string { // sandbox/workspace_path.go:10
	return sandbox.ResolveWorkspacePathIn(sandbox.RemoteWorkspaceLayout(), value)
}

// ShellQuote R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func ShellQuote(s string) string { // sandbox/shell_quote.go:12
	return sandbox.ShellQuote(s)
}

// WithCommandOutput R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func WithCommandOutput(ctx context.Context, callback func(string, []byte)) context.Context { // sandbox/command_output.go:9
	return sandbox.WithCommandOutput(ctx, callback)
}

// WithSessionFileOperation R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func WithSessionFileOperation(ctx context.Context) context.Context { // sandbox/session_file_operation.go:25
	return sandbox.WithSessionFileOperation(ctx)
}

// IsValidSkillName R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func IsValidSkillName(name string) bool { // sandbox/skill_paths.go:30
	return sandbox.IsValidSkillName(name)
}

// SkillDirFor R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func SkillDirFor(skillName string) (string, error) { // sandbox/skill_paths.go:45
	return sandbox.SkillDirFor(skillName)
}

// ValidatedImageSkillDir R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func ValidatedImageSkillDir(skillDir string) (string, bool) { // sandbox/skill_paths.go:106
	return sandbox.ValidatedImageSkillDir(skillDir)
}

// SkillNameFromImagePath R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func SkillNameFromImagePath(p string) (name string, inImage bool) { // sandbox/skill_paths.go:133
	return sandbox.SkillNameFromImagePath(p)
}

// NavigationIncomplete R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func NavigationIncomplete(method string, raw json.RawMessage) bool { // browserskill/result.go:9
	return browserskill.NavigationIncomplete(method, raw)
}
