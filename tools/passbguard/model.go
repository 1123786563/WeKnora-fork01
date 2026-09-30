// Package main 提供 Pass B 治理文件（ownership-matrix.yaml / contracts.yaml /
// event-catalog.yaml / exception-ledger.yaml）的严格 schema 模型、跨文件稳定 ID
// 与结构校验，并经 main.go 聚合为 passbguard CLI。
//
// B0.1–B0.5 期间本目录是库包 passbguard（B0 各任务逐份生成治理文件，由测试
// 直接消费）；B0.6 落地 CLI 时创建 main.go 并把整目录翻转为 package main
// （`go run ./tools/passbguard` 要求 main 包），模型与校验逻辑保持不变。
package main

import (
	"fmt"
	"regexp"
	"strings"
)

// PlanID 是子计划 id，形态 NN-name（两位数字 + 短横线 + 小写名），取自
// docs/plans/2026-09-23-backend-modularization-pass-b-framework.md 的计划表，
// 例如 10-identity、20-knowledge-program、34-agentruntime-protocol、50-final-cleanup-and-acceptance。
type PlanID string

// LegacyID 是 legacy 行的稳定 ID：规范化后的仓库相对精确路径。
type LegacyID string

// ContractID 是 contracts.yaml 记录 id。
type ContractID string

// EventID 是 event-catalog.yaml 记录 id。
type EventID string

// ExceptionID 是 exception-ledger.yaml 记录 id。
type ExceptionID string

// LegacyOwnership 声明一个 legacy 文件的唯一属主与去向（ownership-matrix.yaml）。
// Path 是仓库相对精确路径；Module 必须是已知模块 id；Plan 是删除属主子计划；
// Destination 是搬迁目标包；IntegrationOwner 为共享文件的装配属主（可空）；
// DeleteBarrier 是删除收口屏障（ib1..ib4 / b5）。
type LegacyOwnership struct {
	Path             string `yaml:"path"`
	Module           string `yaml:"module"`
	Plan             PlanID `yaml:"plan"`
	Destination      string `yaml:"destination"`
	IntegrationOwner string `yaml:"integration_owner"`
	DeleteBarrier    string `yaml:"delete_barrier"`
}

// Key 返回该行的稳定 ID（仓库相对路径）。
func (l LegacyOwnership) Key() LegacyID { return LegacyID(l.Path) }

// AliasOwnership 声明一个 Pass A alias 包的删除属主（ownership-matrix.yaml），
// 与 docs/architecture/moves/*.yaml 的 alias_obligations 一一对应。
type AliasOwnership struct {
	OldImportPath string `yaml:"old_import_path"`
	Plan          PlanID `yaml:"plan"`
	DeleteBarrier string `yaml:"delete_barrier"`
}

// Contract 冻结一条能力/组合契约（contracts.yaml）。
// Consumers 是当前生产消费方（仓库相对路径）；
// CharacterizationTests 是锚定旧行为的特征化测试路径；
// Items 是 set 类契约（route-set/worker-set/lifecycle-set/module-construction）
// 的成员清单（路由入口、任务类型、挂点、façade 操作），供 B0.4 Step 6 记录组合面。
type Contract struct {
	ID                    ContractID `yaml:"id"`
	Owner                 string     `yaml:"owner"`
	Kind                  string     `yaml:"kind"`
	Symbol                string     `yaml:"symbol"`
	Signature             string     `yaml:"signature"`
	Stability             string     `yaml:"stability"`
	Consumers             []string   `yaml:"consumers"`
	CharacterizationTests []string   `yaml:"characterization_tests"`
	Items                 []string   `yaml:"items"`
}

// Event 冻结一条版本化权威事件（event-catalog.yaml）。事件只记录已发生的事实；
// 命令式操作保持同步命令/port，不得进事件目录。Transport 记录当前投递形态
// （如 in_process），Replay 记录重放规则（如 source-query）——B0 不新增 broker。
type Event struct {
	ID               EventID  `yaml:"id"`
	Version          int      `yaml:"version"`
	Producer         string   `yaml:"producer"`
	Meaning          string   `yaml:"meaning"`
	Ordering         string   `yaml:"ordering"`
	Replay           string   `yaml:"replay"`
	Transport        string   `yaml:"transport"`
	Consumers        []string `yaml:"consumers"`
	RequiredMetadata []string `yaml:"required_metadata"`
}

// Exception 声明一条 Pass A import 例外的删除属主与期限（exception-ledger.yaml）。
// From 是导入方文件的仓库相对精确路径（.go）；To 是被导入包的完整 import 路径——
// 与 tools/architectureguard 的 importExceptions (ImporterFile, ImportedPath)
// 精确对语义一致，不做 glob/前缀/子串匹配。
type Exception struct {
	ID       ExceptionID `yaml:"id"`
	From     string      `yaml:"from"`
	To       string      `yaml:"to"`
	Plan     PlanID      `yaml:"plan"`
	RemoveAt string      `yaml:"remove_at"`
	Reason   string      `yaml:"reason"`
}

// Governance 是四份治理文件聚合后的内存形态；LoadGovernance 保证各分片
// 已按稳定 ID 排序且通过结构校验。
type Governance struct {
	Legacy     []LegacyOwnership
	Aliases    []AliasOwnership
	Contracts  []Contract
	Events     []Event
	Exceptions []Exception
}

// KnownModules 是 16 个模块 id，与 internal/modules/* 目录及
// docs/architecture/moves/*.yaml manifest 文件名一致。
var KnownModules = []string{
	"agentcatalog",
	"agentruntime",
	"airesource",
	"appconnector",
	"channels",
	"commercial",
	"conversation",
	"craft",
	"datasource",
	"execution",
	"identity",
	"insights",
	"knowledge",
	"policy",
	"system",
	"workbench",
}

// KnownBarriers 是集成屏障与终局节点 id（conventions §1.1/§8）。
var KnownBarriers = []string{"ib1", "ib2", "ib3", "ib4", "b5"}

// KnownContractKinds 是契约 kind 枚举（计划 B0.4 Step 3 冻结）。
var KnownContractKinds = []string{
	"module-construction",
	"capability-port",
	"route-set",
	"worker-set",
	"lifecycle-set",
	"wire-protocol",
	"data-ownership",
}

// imperativeEventVerbs 是命令式动词黑名单（事件 id 的任一段落精确命中即拒绝）：
// 事件是事实，命令走同步 port。过去时/完成体（started/completed/appended）不受影响。
var imperativeEventVerbs = map[string]struct{}{
	"apply": {}, "call": {}, "create": {}, "delete": {}, "dispatch": {},
	"emit": {}, "execute": {}, "fetch": {}, "get": {}, "invoke": {},
	"notify": {}, "patch": {}, "post": {}, "publish": {}, "put": {},
	"remove": {}, "request": {}, "send": {}, "set": {}, "update": {},
	"write": {},
}

var (
	planIDRE    = regexp.MustCompile(`^[0-9]{2}-[a-z][a-z0-9-]*$`)
	eventIDRE   = regexp.MustCompile(`^[a-z0-9_]+(?:\.[a-z0-9_]+)+$`)
	metadataRE  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	goFileExtRE = regexp.MustCompile(`\.go$`)
)

func validPlanID(p PlanID) bool { return planIDRE.MatchString(string(p)) }

func knownModule(m string) bool {
	for _, k := range KnownModules {
		if k == m {
			return true
		}
	}
	return false
}

func knownBarrier(b string) bool {
	for _, k := range KnownBarriers {
		if k == b {
			return true
		}
	}
	return false
}

func knownContractKind(k string) bool {
	for _, c := range KnownContractKinds {
		if c == k {
			return true
		}
	}
	return false
}

// checkRepoPath 校验仓库相对路径形态：精确路径（禁通配符）、禁绝对路径、
// 禁 ".."、禁反斜杠、禁尾部分隔符；requireGoFile 时还必须是 .go 文件路径。
// 返回空串表示合法，否则返回诊断文本。
func checkRepoPath(field, p string, requireGoFile bool) string {
	switch {
	case p == "":
		return fmt.Sprintf("%s: path must not be empty", field)
	case strings.ContainsAny(p, "*?"):
		return fmt.Sprintf("%s: %q must be an exact repository path (wildcards not allowed)", field, p)
	case strings.HasPrefix(p, "/"):
		return fmt.Sprintf("%s: %q must be a repository-relative path (absolute paths not allowed)", field, p)
	case p == ".." || strings.HasPrefix(p, "../") || strings.HasSuffix(p, "/..") || strings.Contains(p, "/../"):
		return fmt.Sprintf("%s: %q must not contain parent traversal \"..\"", field, p)
	case strings.Contains(p, "\\"):
		return fmt.Sprintf("%s: %q must use slash separators", field, p)
	case strings.HasSuffix(p, "/"):
		return fmt.Sprintf("%s: %q must not end with a path separator", field, p)
	case requireGoFile && !goFileExtRE.MatchString(p):
		return fmt.Sprintf("%s: %q must be a .go file path", field, p)
	}
	return ""
}

// checkImportPath 校验 import 路径形态（exception 的 to 字段）：
// 禁通配符、禁 ".."、禁反斜杠、禁前导/尾部分隔符。
func checkImportPath(field, p string) string {
	switch {
	case p == "":
		return fmt.Sprintf("%s: import path must not be empty", field)
	case strings.ContainsAny(p, "*?"):
		return fmt.Sprintf("%s: %q must be an exact repository path (wildcards not allowed)", field, p)
	case strings.HasPrefix(p, "/"):
		return fmt.Sprintf("%s: %q must not be an absolute path", field, p)
	case p == ".." || strings.HasPrefix(p, "../") || strings.HasSuffix(p, "/..") || strings.Contains(p, "/../"):
		return fmt.Sprintf("%s: %q must not contain parent traversal \"..\"", field, p)
	case strings.Contains(p, "\\"):
		return fmt.Sprintf("%s: %q must use slash separators", field, p)
	case strings.HasSuffix(p, "/"):
		return fmt.Sprintf("%s: %q must not end with a path separator", field, p)
	}
	return ""
}

// validate 对排序后的 Governance 做结构校验，返回诊断清单（调用方负责排序/聚合）。
func validate(g *Governance) []string {
	var diags []string

	for i, l := range g.Legacy {
		field := fmt.Sprintf("legacy_files[%d]", i)
		if d := checkRepoPath(field+".path", l.Path, false); d != "" {
			diags = append(diags, d)
		}
		if i > 0 && g.Legacy[i-1].Path == l.Path {
			diags = append(diags, fmt.Sprintf("%s.path: duplicate legacy path %q", field, l.Path))
		}
		if !knownModule(l.Module) {
			diags = append(diags, fmt.Sprintf(
				"%s.module: unknown module id %q (known: %s)", field, l.Module, strings.Join(KnownModules, " ")))
		}
		if l.Plan == "" {
			diags = append(diags, fmt.Sprintf("%s.plan: plan owner must not be empty", field))
		} else if !validPlanID(l.Plan) {
			diags = append(diags, fmt.Sprintf(
				"%s.plan: invalid plan id %q (want NN-name form, e.g. 10-identity)", field, l.Plan))
		}
		if d := checkRepoPath(field+".destination", l.Destination, false); d != "" {
			diags = append(diags, d)
		} else if l.Destination == "" {
			diags = append(diags, fmt.Sprintf("%s.destination: destination must not be empty", field))
		}
		if l.IntegrationOwner != "" && !validPlanID(PlanID(l.IntegrationOwner)) && !knownBarrier(l.IntegrationOwner) {
			diags = append(diags, fmt.Sprintf(
				"%s.integration_owner: %q must be a plan id or barrier (%s)",
				field, l.IntegrationOwner, strings.Join(KnownBarriers, "/")))
		}
		if !knownBarrier(l.DeleteBarrier) {
			diags = append(diags, fmt.Sprintf(
				"%s.delete_barrier: %q must be one of %s",
				field, l.DeleteBarrier, strings.Join(KnownBarriers, "/")))
		}
	}

	for i, a := range g.Aliases {
		field := fmt.Sprintf("aliases[%d]", i)
		if d := checkRepoPath(field+".old_import_path", a.OldImportPath, false); d != "" {
			diags = append(diags, d)
		}
		if i > 0 && g.Aliases[i-1].OldImportPath == a.OldImportPath {
			diags = append(diags, fmt.Sprintf(
				"%s.old_import_path: duplicate alias old_import_path %q", field, a.OldImportPath))
		}
		if a.Plan == "" {
			diags = append(diags, fmt.Sprintf("%s.plan: plan owner must not be empty", field))
		} else if !validPlanID(a.Plan) {
			diags = append(diags, fmt.Sprintf(
				"%s.plan: invalid plan id %q (want NN-name form, e.g. 10-identity)", field, a.Plan))
		}
		if a.DeleteBarrier != "" && !knownBarrier(a.DeleteBarrier) {
			diags = append(diags, fmt.Sprintf(
				"%s.delete_barrier: %q must be one of %s",
				field, a.DeleteBarrier, strings.Join(KnownBarriers, "/")))
		}
	}

	for i, c := range g.Contracts {
		field := fmt.Sprintf("contracts[%d]", i)
		if c.ID == "" {
			diags = append(diags, fmt.Sprintf("%s.id: contract id must not be empty", field))
		}
		if i > 0 && g.Contracts[i-1].ID == c.ID {
			diags = append(diags, fmt.Sprintf("%s.id: duplicate contract id %q", field, c.ID))
		}
		if !knownModule(c.Owner) {
			diags = append(diags, fmt.Sprintf(
				"%s.owner: unknown module id %q (known: %s)", field, c.Owner, strings.Join(KnownModules, " ")))
		}
		if !knownContractKind(c.Kind) {
			diags = append(diags, fmt.Sprintf(
				"%s.kind: unknown contract kind %q (known: %s)",
				field, c.Kind, strings.Join(KnownContractKinds, " ")))
		}
		if c.Symbol == "" {
			diags = append(diags, fmt.Sprintf("%s.symbol: symbol must not be empty", field))
		}
		if c.Stability == "" {
			diags = append(diags, fmt.Sprintf("%s.stability: stability must not be empty", field))
		}
		for j, p := range c.Consumers {
			if d := checkRepoPath(fmt.Sprintf("%s.consumers[%d]", field, j), p, false); d != "" {
				diags = append(diags, d)
			}
		}
		for j, p := range c.CharacterizationTests {
			if d := checkRepoPath(fmt.Sprintf("%s.characterization_tests[%d]", field, j), p, false); d != "" {
				diags = append(diags, d)
			}
		}
		for j, item := range c.Items {
			if strings.TrimSpace(item) == "" {
				diags = append(diags, fmt.Sprintf("%s.items[%d]: item must not be blank", field, j))
			}
		}
	}

	seenProducerVersion := map[string]int{}
	for i, e := range g.Events {
		field := fmt.Sprintf("events[%d]", i)
		if e.ID == "" {
			diags = append(diags, fmt.Sprintf("%s.id: event id must not be empty", field))
		} else if !eventIDRE.MatchString(string(e.ID)) {
			diags = append(diags, fmt.Sprintf(
				"%s.id: event id %q must be dot-separated lowercase snake segments "+
					"(at least 2 segments, e.g. conversation.message.appended)", field, e.ID))
		} else if v := imperativeEventVerb(string(e.ID)); v != "" {
			diags = append(diags, fmt.Sprintf(
				"%s.id: event id %q contains imperative command verb %q; "+
					"events record facts, commands stay synchronous ports", field, e.ID, v))
		}
		if i > 0 && g.Events[i-1].ID == e.ID {
			diags = append(diags, fmt.Sprintf("%s.id: duplicate event id %q", field, e.ID))
		}
		if e.Version < 1 {
			diags = append(diags, fmt.Sprintf(
				"%s.version: version must be a positive integer, got %d", field, e.Version))
		}
		if e.Producer == "" {
			diags = append(diags, fmt.Sprintf("%s.producer: producer must not be empty", field))
		}
		if e.Meaning == "" {
			diags = append(diags, fmt.Sprintf("%s.meaning: meaning must not be empty", field))
		}
		if e.Ordering == "" {
			diags = append(diags, fmt.Sprintf("%s.ordering: ordering must not be empty", field))
		}
		if e.Replay == "" {
			diags = append(diags, fmt.Sprintf("%s.replay: replay rule must not be empty", field))
		}
		if e.Transport == "" {
			diags = append(diags, fmt.Sprintf("%s.transport: transport must not be empty", field))
		}
		if len(e.Consumers) == 0 {
			diags = append(diags, fmt.Sprintf("%s.consumers: consumers must not be empty", field))
		}
		for j, p := range e.Consumers {
			if d := checkRepoPath(fmt.Sprintf("%s.consumers[%d]", field, j), p, false); d != "" {
				diags = append(diags, d)
			}
		}
		if len(e.RequiredMetadata) == 0 {
			diags = append(diags, fmt.Sprintf("%s.required_metadata: required_metadata must not be empty", field))
		}
		hasMeta := func(key string) bool {
			for _, m := range e.RequiredMetadata {
				if m == key {
					return true
				}
			}
			return false
		}
		if !hasMeta("idempotency_key") {
			diags = append(diags, fmt.Sprintf("%s.required_metadata: must contain \"idempotency_key\"", field))
		}
		if hasMeta("tenant_id") {
			// OCR R1 #b0-ocr-r1-event-metadata-actor-origin：与 event-catalog.yaml
			// 头注释声明的强制五元组对齐（tenant_id 触发级联，其余四键逐一强制，
			// idempotency_key 由上方独立检查覆盖）。
			for _, key := range []string{"occurred_at", "event_id", "actor_origin"} {
				if !hasMeta(key) {
					diags = append(diags, fmt.Sprintf(
						"%s.required_metadata: tenant-scoped event must also require %q", field, key))
				}
			}
		}
		for j, m := range e.RequiredMetadata {
			if !metadataRE.MatchString(m) {
				diags = append(diags, fmt.Sprintf(
					"%s.required_metadata[%d]: metadata key %q must be lower snake_case", field, j, m))
			}
		}
		pair := fmt.Sprintf("%s/v%d", e.Producer, e.Version)
		if first, dup := seenProducerVersion[pair]; dup {
			diags = append(diags, fmt.Sprintf(
				"%s.id: duplicate event producer/version pair %q (first declared at events[%d])",
				field, pair, first))
		} else {
			seenProducerVersion[pair] = i
		}
	}

	for i, x := range g.Exceptions {
		field := fmt.Sprintf("exceptions[%d]", i)
		if x.ID == "" {
			diags = append(diags, fmt.Sprintf("%s.id: exception id must not be empty", field))
		}
		if i > 0 && g.Exceptions[i-1].ID == x.ID {
			diags = append(diags, fmt.Sprintf("%s.id: duplicate exception id %q", field, x.ID))
		}
		if d := checkRepoPath(field+".from", x.From, true); d != "" {
			diags = append(diags, d)
		}
		if d := checkImportPath(field+".to", x.To); d != "" {
			diags = append(diags, d)
		}
		if x.Plan == "" {
			diags = append(diags, fmt.Sprintf("%s.plan: plan owner must not be empty", field))
		} else if !validPlanID(x.Plan) {
			diags = append(diags, fmt.Sprintf(
				"%s.plan: invalid plan id %q (want NN-name form, e.g. 10-identity)", field, x.Plan))
		}
		if !knownBarrier(x.RemoveAt) {
			diags = append(diags, fmt.Sprintf(
				"%s.remove_at: %q must be one of %s",
				field, x.RemoveAt, strings.Join(KnownBarriers, "/")))
		}
		if x.Reason == "" {
			diags = append(diags, fmt.Sprintf("%s.reason: reason must not be empty", field))
		}
	}

	return diags
}

// imperativeEventVerb 返回事件 id 中首个命中命令式动词黑名单的段落（精确匹配），
// 无命中返回空串。
func imperativeEventVerb(id string) string {
	for _, seg := range strings.Split(id, ".") {
		if _, hit := imperativeEventVerbs[seg]; hit {
			return seg
		}
	}
	return ""
}
