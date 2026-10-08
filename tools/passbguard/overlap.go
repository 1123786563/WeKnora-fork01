package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ForbiddenPhrases 是 B0.3 Step 1（freeze:187）禁止出现的歧义表述：
// 治理/brief 文本不得以「先合并者得」「二选一执行」类措辞悬置所有权，
// 每个文件必须恰有一个子计划属主。
var ForbiddenPhrases = []string{
	"first merged wins",
	"先合并者",
	"二选一执行",
}

// AgentRuntimeNativeRuling 冻结 B0.3 Step 2 的 Agent Runtime 精确属主
// （freeze:189-196）：
//   - native_archive 服务与 handler 双文件归 34-agentruntime-protocol；
//   - native_recovery 与 native repository 状态/lease/pending/usage 族
//     （commit/events/memory/oauth/schema/session/tool_journal 均为引擎侧
//     持久化状态）归 33-agentruntime-engine。
//
// 审批（approval）、Run/Attempt、checkpoint、decisions、events、inputs、
// lifecycle、tools journal 的 agent_* 仓库/服务文件同归 33，经
// HandlerSessionRuling 与 CheckOwnership 全量覆盖，不在此重复枚举。
var AgentRuntimeNativeRuling = map[string]PlanID{
	"internal/application/service/native_archive.go":         "34-agentruntime-protocol",
	"internal/handler/session/native_archive.go":             "34-agentruntime-protocol",
	"internal/application/service/native_recovery.go":        "33-agentruntime-engine",
	"internal/application/repository/native_commit.go":       "33-agentruntime-engine",
	"internal/application/repository/native_events.go":       "33-agentruntime-engine",
	"internal/application/repository/native_lease.go":        "33-agentruntime-engine",
	"internal/application/repository/native_memory.go":       "33-agentruntime-engine",
	"internal/application/repository/native_oauth.go":        "33-agentruntime-engine",
	"internal/application/repository/native_pending.go":      "33-agentruntime-engine",
	"internal/application/repository/native_schema.go":       "33-agentruntime-engine",
	"internal/application/repository/native_session.go":      "33-agentruntime-engine",
	"internal/application/repository/native_tool_journal.go": "33-agentruntime-engine",
	"internal/application/repository/native_usage.go":        "33-agentruntime-engine",
	"internal/application/service/native_admission.go":       "33-agentruntime-engine",
	"internal/application/service/native_oauth.go":           "33-agentruntime-engine",
	"internal/application/service/native_pending.go":         "33-agentruntime-engine",
	"internal/application/service/native_usage.go":           "33-agentruntime-engine",
}

// ProtocolFamilyPackages 是纯协议包（模块内已有、边界收敛归
// 34-agentruntime-protocol；freeze:195）：native 会话协议、契约、探针、
// tRPC durable run、OpenCode 本地代理与 recovery 验收工具。
var ProtocolFamilyPackages = []string{
	"agent/native",
	"agent/nativecontract",
	"agent/nativeprobe",
	"agent/trpc",
	"agent/opencode",
	"agent/recoverytest",
}

// HandlerSessionRuling 冻结 B0.3 Step 3 的共享宿主 internal/handler/session
// 全部 36 个 legacy 文件的逐路径属主（freeze:198-205）：
//   - wiki_fixer_scope.go → Knowledge Wiki/FAQ（23）；
//   - workbench_*/artifact 文件 → Workbench（40）；
//   - craft* 文件 → Craft（41）；
//   - Agent Run/stream 文件 → Agent Runtime Engine（33）；
//   - native_archive.go → Agent Runtime Protocol（34）；
//   - browserskill/sandbox_terminal_* → Execution（13）；
//   - 其余 Session/Message/Feedback/share/attachment/stream 文件 →
//     Conversation Session（35）。
var HandlerSessionRuling = map[string]PlanID{
	"internal/handler/session/agent_run.go":            "33-agentruntime-engine",
	"internal/handler/session/agent_stream_handler.go": "33-agentruntime-engine",
	"internal/handler/session/native_archive.go":       "34-agentruntime-protocol",
	// wiki_fixer_scope.go 已随 K3 迁入模块（IB2 回写批删 matrix 残留行）；
	// 宿主过渡 shim wiki_fixer_scope_compat.go 曾承接裁定属主（删除点=ib2），
	// R2.5 上游布局回归收尾（2026-10-07）随文件删除一并移除裁定与 matrix 行。
	"internal/handler/session/browserskill.go":            "13-execution",
	"internal/handler/session/sandbox_terminal_bridge.go": "13-execution",
	"internal/handler/session/sandbox_terminal_ws.go":     "13-execution",
	"internal/handler/session/artifact_download.go":       "40-workbench",
	"internal/handler/session/artifact_reference.go":      "40-workbench",
	"internal/handler/session/workbench_artifacts.go":     "40-workbench",
	"internal/handler/session/workbench_commands.go":      "40-workbench",
	"internal/handler/session/workbench_inbox.go":         "40-workbench",
	"internal/handler/session/workbench_list.go":          "40-workbench",
	"internal/handler/session/workbench_overview.go":      "40-workbench",
	"internal/handler/session/workbench_read.go":          "40-workbench",
	"internal/handler/session/workbench_start.go":         "40-workbench",
	"internal/handler/session/craft.go":                   "41-craft",
	"internal/handler/session/craft_interaction.go":       "41-craft",
	"internal/handler/session/craft_preview.go":           "41-craft",
	"internal/handler/session/craft_scheduled.go":         "41-craft",
	"internal/handler/session/craft_usage.go":             "41-craft",
	"internal/handler/session/attachment_processor.go":    "35-conversation-program",
	"internal/handler/session/fork.go":                    "35-conversation-program",
	"internal/handler/session/handler.go":                 "35-conversation-program",
	"internal/handler/session/helpers.go":                 "35-conversation-program",
	"internal/handler/session/image_upload.go":            "35-conversation-program",
	"internal/handler/session/qa.go":                      "35-conversation-program",
	"internal/handler/session/query_history_admin.go":     "35-conversation-program",
	"internal/handler/session/quick_answer_timeline.go":   "35-conversation-program",
	"internal/handler/session/resource_urls.go":           "35-conversation-program",
	"internal/handler/session/share.go":                   "35-conversation-program",
	"internal/handler/session/steer.go":                   "35-conversation-program",
	"internal/handler/session/stream.go":                  "35-conversation-program",
	"internal/handler/session/temporary_document.go":      "35-conversation-program",
	"internal/handler/session/title.go":                   "35-conversation-program",
	"internal/handler/session/types.go":                   "35-conversation-program",
}

// HousekeepingRuling 冻结 B0.3 Step 4（freeze:207-209）：
// knowledge_housekeeping.go 与业务清扫规则留 Knowledge（24-knowledge-process）；
// System（42-system-policy）经窄 KnowledgeHousekeeping port 拥有调度/生命周期调用，
// 不得出现第二套清扫实现。
var HousekeepingRuling = map[string]PlanID{
	"internal/application/service/knowledge_housekeeping.go": "24-knowledge-process",
}

// PlatformPreservedFiles 冻结 B0.3 Step 3 末条（freeze:205）：
// 通用分页/上传限制（及错误判定 helper，如 isRequestBodyTooLarge）保持 platform，
// 直至独立的消费者抽取计划另行裁定；任何 Pass B 子计划不得认领。
var PlatformPreservedFiles = []string{
	"internal/handler/list_pagination.go",
	"internal/handler/upload_limit.go",
}

// BriefPlans 是 passb brief 文件名到其代表子计划的映射（框架计划表，
// docs/plans/2026-09-23-backend-modularization-pass-b-framework.md）。
// conversation 两份 brief 同属 35-conversation-program（会话面/查询历史面
// 是同一计划的两个义务拆分）；未列名的 *.md（如 b0-evidence.md）不参与
// brief 认领校验。
var BriefPlans = map[string][]PlanID{
	"agentruntime-engine.md":       {"33-agentruntime-engine"},
	"agentruntime-memory.md":       {"31-agentruntime-memory"},
	"agentruntime-protocol.md":     {"34-agentruntime-protocol"},
	"agentruntime-tools.md":        {"32-agentruntime-tools"},
	"conversation-queryhistory.md": {"35-conversation-program"},
	"conversation-session.md":      {"35-conversation-program"},
	"craft.md":                     {"41-craft"},
	"execution.md":                 {"13-execution"},
	"insights.md":                  {"37-insights"},
	"knowledge-ingest.md":          {"21-knowledge-ingest"},
	"knowledge-process.md":         {"24-knowledge-process"},
	"knowledge-retrieval.md":       {"22-knowledge-retrieval"},
	"knowledge-wikifaq.md":         {"23-knowledge-wikifaq"},
	"workbench.md":                 {"40-workbench"},
}

// OwnershipRulings 汇总全部 B0.3 裁定行（native 族 + handler/session 全集 +
// housekeeping），供 CheckRulings/CheckBriefClaims 与 B0.6 CLI 消费。
func OwnershipRulings() map[string]PlanID {
	out := make(map[string]PlanID, len(AgentRuntimeNativeRuling)+len(HandlerSessionRuling)+len(HousekeepingRuling))
	for k, v := range AgentRuntimeNativeRuling {
		out[k] = v
	}
	for k, v := range HandlerSessionRuling {
		out[k] = v
	}
	for k, v := range HousekeepingRuling {
		out[k] = v
	}
	return out
}

// CheckAmbiguity 扫描 docs/architecture/passb 下全部 *.md 与 *.yaml 治理文本，
// 拒绝任何 ForbiddenPhrases 命中（诊断含行号）。
func CheckAmbiguity(root string) ([]Diagnostic, error) {
	dir := filepath.Join(root, filepath.FromSlash(GovernanceDir))
	var names []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		ext := filepath.Ext(entry.Name())
		if ext == ".md" || ext == ".yaml" || ext == ".yml" {
			names = append(names, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan governance dir: %w", err)
	}
	sort.Strings(names)

	var ds []Diagnostic
	for _, path := range names {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}
		slash := filepath.ToSlash(rel)
		for lineNo, line := range strings.Split(string(data), "\n") {
			for _, phrase := range ForbiddenPhrases {
				if strings.Contains(line, phrase) {
					ds = append(ds, Diagnostic{
						Check: "ambiguous-phrase",
						Path:  slash,
						Message: fmt.Sprintf("line %d: ambiguous ownership phrase %q; "+
							"every file must have exactly one child-plan owner (freeze B0.3 Step 1)",
							lineNo+1, phrase),
					})
				}
			}
		}
	}
	return sortDiagnosticsDiag(ds), nil
}

// CheckRulings 把 ownership-matrix 与 B0.3 冻结裁定逐路径对照：
// 裁定路径缺行、属主漂移、platform 保留文件被认领均为诊断。
func CheckRulings(g *Governance) []Diagnostic {
	return checkRulings(g, OwnershipRulings(), PlatformPreservedFiles)
}

func checkRulings(g *Governance, rulings map[string]PlanID, platform []string) []Diagnostic {
	owner := map[string]PlanID{}
	for _, row := range g.Legacy {
		owner[row.Path] = row.Plan
	}
	var ds []Diagnostic
	paths := make([]string, 0, len(rulings))
	for p := range rulings {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		want := rulings[p]
		got, ok := owner[p]
		switch {
		case !ok:
			ds = append(ds, Diagnostic{
				Check: "ruling-missing", Path: p,
				Message: fmt.Sprintf(
					"B0.3 ruled path has no ownership-matrix row (want owner %q)", want),
			})
		case got != want:
			ds = append(ds, Diagnostic{
				Check: "ruling-owner", Path: p,
				Message: fmt.Sprintf("owner %q violates B0.3 ruling (want %q)", got, want),
			})
		}
	}
	for _, p := range platform {
		if _, claimed := owner[p]; claimed {
			ds = append(ds, Diagnostic{
				Check: "ruling-platform-claimed", Path: p,
				Message: "platform-preserved generic helper must not be claimed by any child plan (B0.3 Step 3)",
			})
		}
	}
	return sortDiagnosticsDiag(ds)
}

// briefClaimsPath 判断 brief 文本是否认领了 ruled path。brief 两种认领形态
// 并存（agentruntime-protocol.md 完整路径、conversation-session.md 目录标题下
// 裸 basename），匹配分两层（OCR R1 ocr-r1-5：ruled 集含 types.go/handler.go
// 等极常见 basename，仅凭 basename 认领会让非属主 brief 提及「另一个文件的
// 更长路径」即误报 brief-claim-conflict）：
//  1. 完整 path 以独立 token 出现即认领——前一字符不得是词字符或 '.'
//     （排除子串误报），之后若紧跟 `:数字` 则该出现是 :line 消费方引用
//     （如 `agent_run_graph.go:22`），不算认领；
//  2. basename 回退——同上边界约束，且出现位置前驱为 '/'（即属于更长路径
//     的一部分，如 internal/workbench/types.go 之于 ruled
//     internal/handler/session/types.go）不得计为裸 basename 认领。
func briefClaimsPath(text, path string) bool {
	if matchIndependentToken(text, path) {
		return true
	}
	return matchIndependentToken(text, pathBase(path))
}

// matchIndependentToken 判断 needle 在 text 中是否以独立 token 出现：
// 前一字符不得是词字符、'.' 或 '/'（更长路径的尾段不是裸名认领）；后一
// 字符若是词字符则是更长标识符的子串，若是 `:数字` 则是 :line 消费方引用。
func matchIndependentToken(text, needle string) bool {
	for i := 0; i < len(text); {
		idx := strings.Index(text[i:], needle)
		if idx < 0 {
			return false
		}
		start := i + idx
		end := start + len(needle)
		i = end
		if start > 0 {
			if prev := text[start-1]; isWordRune(prev) || prev == '.' || prev == '/' {
				continue
			}
		}
		if end == len(text) {
			return true
		}
		next := text[end]
		if isWordRune(next) {
			continue // 更长标识符的子串
		}
		if next == ':' && end+1 < len(text) && text[end+1] >= '0' && text[end+1] <= '9' {
			continue // :line 消费方引用
		}
		return true
	}
	return false
}

func pathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func isWordRune(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// CheckBriefClaims 校验 passb brief 的文件认领与 B0.3 裁定一致：
// 每个裁定路径必须被其属主计划的 brief 认领（brief-claim-missing），
// 且不得被代表其他计划的 brief 认领（brief-claim-conflict）。
func CheckBriefClaims(root string) ([]Diagnostic, error) {
	return checkBriefClaims(root, OwnershipRulings())
}

func checkBriefClaims(root string, rulings map[string]PlanID) ([]Diagnostic, error) {
	dir := filepath.Join(root, filepath.FromSlash(GovernanceDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read brief dir: %w", err)
	}
	briefs := map[string]string{} // brief 文件名 -> 全文
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		if _, known := BriefPlans[e.Name()]; !known {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read brief %s: %w", e.Name(), err)
		}
		briefs[e.Name()] = string(data)
	}

	var ds []Diagnostic
	paths := make([]string, 0, len(rulings))
	for p := range rulings {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		want := rulings[p]
		claimed := false
		names := make([]string, 0, len(briefs))
		for name := range briefs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if !briefClaimsPath(briefs[name], p) {
				continue
			}
			plans := BriefPlans[name]
			if containsPlan(plans, want) {
				claimed = true
				continue
			}
			ds = append(ds, Diagnostic{
				Check: "brief-claim-conflict", Path: name,
				Message: fmt.Sprintf("claims ruled file %s owned by %q (B0.3 ruling); "+
					"remove the claim or the ruling owner changes via baseline change", p, want),
			})
		}
		if !claimed {
			ds = append(ds, Diagnostic{
				Check: "brief-claim-missing", Path: p,
				Message: fmt.Sprintf(
					"ruling owner %q brief does not claim this file (B0.3 Step 2/3/4)", want),
			})
		}
	}
	return sortDiagnosticsDiag(ds), nil
}

func containsPlan(plans []PlanID, want PlanID) bool {
	for _, p := range plans {
		if p == want {
			return true
		}
	}
	return false
}
