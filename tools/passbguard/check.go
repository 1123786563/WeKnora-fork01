package passbguard

import (
	"fmt"
	"sort"
	"strings"
)

// Diagnostic 是 passbguard 的一条治理违规。
type Diagnostic struct {
	Check   string
	Path    string
	Message string
}

// String 输出形如 "passbguard: <check>: <path>: <message>"（path 可空时省略）。
func (d Diagnostic) String() string {
	if d.Path == "" {
		return fmt.Sprintf("passbguard: %s: %s", d.Check, d.Message)
	}
	return fmt.Sprintf("passbguard: %s: %s: %s", d.Check, d.Path, d.Message)
}

// sortDiagnosticsDiag 按 String 排序并去重。
func sortDiagnosticsDiag(ds []Diagnostic) []Diagnostic {
	out := append([]Diagnostic(nil), ds...)
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	dedup := make([]Diagnostic, 0, len(out))
	for i, d := range out {
		if i == 0 || d.String() != out[i-1].String() {
			dedup = append(dedup, d)
		}
	}
	return dedup
}

// PlanInfo 描述一个 Pass B 子计划：可属主的模块集合与阶段集成屏障。
type PlanInfo struct {
	Modules []string
	Barrier string
}

var allModules = append([]string(nil), KnownModules...)

// KnownPlans 是框架计划表（docs/plans/2026-09-23-backend-modularization-pass-b-framework.md）
// 中可拥有 legacy/alias/exception 的计划；IB 计划（19/29/39/49）不拥有文件，不在此列。
var KnownPlans = map[PlanID]PlanInfo{
	"10-identity":                     {Modules: []string{"identity"}, Barrier: "ib1"},
	"11-airesource":                   {Modules: []string{"airesource"}, Barrier: "ib1"},
	"12-commercial":                   {Modules: []string{"commercial"}, Barrier: "ib1"},
	"13-execution":                    {Modules: []string{"execution"}, Barrier: "ib1"},
	"20-knowledge-program":            {Modules: []string{"knowledge"}, Barrier: "ib2"},
	"21-knowledge-ingest":             {Modules: []string{"knowledge"}, Barrier: "ib2"},
	"22-knowledge-retrieval":          {Modules: []string{"knowledge"}, Barrier: "ib2"},
	"23-knowledge-wikifaq":            {Modules: []string{"knowledge"}, Barrier: "ib2"},
	"24-knowledge-process":            {Modules: []string{"knowledge"}, Barrier: "ib2"},
	"25-agentcatalog-program":         {Modules: []string{"agentcatalog"}, Barrier: "ib2"},
	"26-datasource":                   {Modules: []string{"datasource"}, Barrier: "ib2"},
	"27-appconnector":                 {Modules: []string{"appconnector"}, Barrier: "ib2"},
	"30-agentruntime-program":         {Modules: []string{"agentruntime"}, Barrier: "ib3"},
	"31-agentruntime-memory":          {Modules: []string{"agentruntime"}, Barrier: "ib3"},
	"32-agentruntime-tools":           {Modules: []string{"agentruntime"}, Barrier: "ib3"},
	"33-agentruntime-engine":          {Modules: []string{"agentruntime"}, Barrier: "ib3"},
	"34-agentruntime-protocol":        {Modules: []string{"agentruntime"}, Barrier: "ib3"},
	"35-conversation-program":         {Modules: []string{"conversation"}, Barrier: "ib3"},
	"36-channels":                     {Modules: []string{"channels"}, Barrier: "ib3"},
	"37-insights":                     {Modules: []string{"insights"}, Barrier: "ib3"},
	"40-workbench":                    {Modules: []string{"workbench"}, Barrier: "ib4"},
	"41-craft":                        {Modules: []string{"craft"}, Barrier: "ib4"},
	"42-system-policy":                {Modules: []string{"system", "policy"}, Barrier: "ib4"},
	"50-final-cleanup-and-acceptance": {Modules: allModules, Barrier: "b5"},
}

// PassBTaskModule 把 guard 源 importExceptions 的模块级 PassBTask 映射到模块 id，
// 用于例外属主计划与 guard 源的模块绑定校验。
var PassBTaskModule = map[string]string{
	"B-agentruntime": "agentruntime",
	"B-airesource":   "airesource",
	"B-appconnector": "appconnector",
	"B-channels":     "channels",
	"B-conversation": "conversation",
	"B-craft":        "craft",
	"B-execution":    "execution",
	"B-knowledge":    "knowledge",
	"B-workbench":    "workbench",
}

// modulesDirPrefix 是所有模块目标包的强制前缀。
const modulesDirPrefix = "internal/modules/"

// CheckOwnership 把治理文件（ownership-matrix.yaml / exception-ledger.yaml 生成的
// Governance）与仓库发现（Discovery）对照，报告全部覆盖/重叠/属主/期限诊断。
// 返回值已排序去重；空切片即通过。
func CheckOwnership(g *Governance, d *Discovery) []Diagnostic {
	var ds []Diagnostic
	emit := func(check, path, format string, args ...any) {
		ds = append(ds, Diagnostic{Check: check, Path: path, Message: fmt.Sprintf(format, args...)})
	}

	manifestLegacy := map[string]string{} // path -> module
	for _, lf := range d.LegacyFiles {
		manifestLegacy[lf.Path] = lf.Module
	}
	goFileSet := make(map[string]bool, len(d.GoFiles))
	for _, p := range d.GoFiles {
		goFileSet[p] = true
	}

	// ---- legacy 覆盖 / 重叠 / 属主一致性 ----
	seenLegacy := map[string]PlanID{}
	for _, row := range g.Legacy {
		manifest, declared := manifestLegacy[row.Path]
		switch {
		case !declared:
			emit("legacy-undeclared", row.Path, "not declared by any manifest legacy_files")
		case manifest != row.Module:
			emit("legacy-manifest-module", row.Path, "matrix module %q disagrees with manifest owner %q", row.Module, manifest)
		}
		if !goFileSet[row.Path] {
			emit("legacy-nonexistent", row.Path, "path does not exist on disk")
		}
		if prev, dup := seenLegacy[row.Path]; dup {
			emit("legacy-overlap", row.Path, "claimed by two plans %q and %q", prev, row.Plan)
		} else {
			seenLegacy[row.Path] = row.Plan
		}
		info, ok := KnownPlans[row.Plan]
		if !ok {
			emit("legacy-plan-unknown", row.Path, "plan %q is not a framework plan that can own files", row.Plan)
			continue
		}
		if !containsModule(info.Modules, row.Module) {
			emit("legacy-plan-module", row.Path, "plan %q (modules %s) cannot own a %q file", row.Plan, strings.Join(info.Modules, "/"), row.Module)
		}
		wantPrefix := modulesDirPrefix + row.Module + "/"
		if !strings.HasPrefix(row.Destination, wantPrefix) {
			emit("legacy-destination", row.Path, "destination %q is outside owning module tree %q", row.Destination, wantPrefix)
		}
		if row.DeleteBarrier != info.Barrier {
			emit("legacy-barrier", row.Path, "delete_barrier %q does not match plan %q phase barrier %q", row.DeleteBarrier, row.Plan, info.Barrier)
		}
		if row.IntegrationOwner != "" && knownBarrier(row.IntegrationOwner) && row.IntegrationOwner != info.Barrier {
			emit("legacy-integration-owner", row.Path, "integration_owner %q must be the owning plan phase barrier %q when set to a barrier", row.IntegrationOwner, info.Barrier)
		}
	}
	for path, module := range manifestLegacy {
		if _, owned := seenLegacy[path]; !owned {
			emit("legacy-missing", path, "manifest legacy file of module %q has no ownership-matrix owner", module)
		}
	}

	// ---- alias 义务覆盖 ----
	aliasModule := map[string]string{} // old_import_path -> module
	for _, m := range d.Manifests {
		for _, a := range m.AliasObligations {
			aliasModule[a.OldImportPath] = m.Module
		}
	}
	for _, a := range d.Aliases {
		if _, ok := aliasModule[a.OldImportPath]; !ok {
			aliasModule[a.OldImportPath] = PassBTaskModule[a.PassBTask]
		}
	}
	seenAlias := map[string]bool{}
	for _, row := range g.Aliases {
		module, declared := aliasModule[row.OldImportPath]
		if !declared {
			emit("alias-undeclared", row.OldImportPath, "not declared by any manifest alias_obligations")
		}
		seenAlias[row.OldImportPath] = true
		info, ok := KnownPlans[row.Plan]
		if !ok {
			emit("alias-plan-unknown", row.OldImportPath, "plan %q is not a framework plan that can own files", row.Plan)
			continue
		}
		if declared && module != "" && !containsModule(info.Modules, module) {
			emit("alias-plan-module", row.OldImportPath, "plan %q (modules %s) cannot own an alias of module %q", row.Plan, strings.Join(info.Modules, "/"), module)
		}
		if row.DeleteBarrier != "" && row.DeleteBarrier != info.Barrier {
			emit("alias-barrier", row.OldImportPath, "delete_barrier %q does not match plan %q phase barrier %q", row.DeleteBarrier, row.Plan, info.Barrier)
		}
	}
	for path, module := range aliasModule {
		if !seenAlias[path] {
			emit("alias-missing", path, "manifest alias obligation of module %q has no ownership-matrix owner", module)
		}
	}

	// ---- exception 台账与 guard 源的精确对照（F1）----
	type excKey struct{ from, to string }
	discovered := map[excKey]DiscoveredException{}
	for _, de := range d.Exceptions {
		discovered[excKey{de.ImporterFile, de.ImportedPath}] = de
	}
	ledger := map[excKey]Exception{}
	for _, x := range g.Exceptions {
		ledger[excKey{x.From, x.To}] = x
		key := excKey{x.From, x.To}
		de, registered := discovered[key]
		if !registered {
			emit("exception-unregistered", x.From, "%s → %s is not registered in %s importExceptions", x.From, x.To, architectureGuardCheckFile)
		} else {
			if x.Reason != de.Reason {
				emit("exception-reason-drift", x.From, "reason drift for %s → %s: ledger %q vs guard %q", x.From, x.To, x.Reason, de.Reason)
			}
			taskModule := PassBTaskModule[de.PassBTask]
			info, ok := KnownPlans[x.Plan]
			if !ok {
				emit("exception-plan-unknown", x.From, "plan %q is not a framework plan that can own exceptions", x.Plan)
			} else if taskModule == "" {
				emit("exception-task-module", x.From, "guard PassBTask %q has no module mapping", de.PassBTask)
			} else if !containsModule(info.Modules, taskModule) {
				emit("exception-plan-module", x.From, "plan %q (modules %s) cannot remove an exception bound to task %q", x.Plan, strings.Join(info.Modules, "/"), de.PassBTask)
			} else {
				if x.RemoveAt != info.Barrier {
					emit("exception-barrier", x.From, "remove_at %q does not match plan %q phase barrier %q", x.RemoveAt, x.Plan, info.Barrier)
				}
				if x.RemoveAt == "b5" && info.Barrier != "b5" {
					emit("exception-b5-escape", x.From, "exception deferred to b5 while plan %q can remove it at %q", x.Plan, info.Barrier)
				}
			}
		}
		if !goFileSet[x.From] {
			emit("exception-importer-missing", x.From, "importer file does not exist on disk")
		} else if imports, ok := d.Imports[x.From]; !ok || !containsExact(imports, x.To) {
			emit("exception-import-absent", x.From, "import %q no longer present in file", x.To)
		}
	}
	for key := range discovered {
		if _, owned := ledger[key]; !owned {
			emit("exception-missing", key.from, "no exception-ledger entry for %s → %s registered in %s", key.from, key.to, architectureGuardCheckFile)
		}
	}

	return sortDiagnosticsDiag(ds)
}

func containsModule(modules []string, want string) bool {
	for _, m := range modules {
		if m == want {
			return true
		}
	}
	return false
}

func containsExact(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
