package main

import (
	"fmt"
	"sort"
)

// Diagnostic 是一条可读的校验诊断（排序后输出）。
type Diagnostic struct {
	Module  string // 所属 manifest 的 module id（跨 manifest 检查用 "*"）
	Check   string // 检查名，如 "alias-1to1"、"ownership-overlap"
	Message string // 具体违规描述
}

// String 输出形如 "modulemove: <module>: <check>: <message>"。
func (d Diagnostic) String() string {
	return fmt.Sprintf("modulemove: %s: %s: %s", d.Module, d.Check, d.Message)
}

// SortDiagnostics 按 String 排序并去重。
func SortDiagnostics(ds []Diagnostic) []Diagnostic {
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

// GoListError 镜像 go list -json 输出中的 Error 字段。
type GoListError struct {
	Err string `json:"Err"`
}

// GoPackage 是 go list -json 输出中本工具关心的字段子集。
type GoPackage struct {
	ImportPath string
	Dir        string
	Name       string
	Error      *GoListError `json:"Error,omitempty"`
}

// GoLister 对一个 go list pattern（如 "internal/agent/..."）执行 `go list -e -json`。
// 生产实现走真实 go 命令；测试注入假实现以保持用例离线、快速。
type GoLister func(pattern string) ([]GoPackage, error)

// Verifier 持有校验所需的环境（仓库根 + 可注入的 go list）。
type Verifier struct {
	Root   string
	GoList GoLister
}

// VerifyManifest 校验单个 manifest 的内部一致性。
func (v *Verifier) VerifyManifest(m *MoveManifest) []Diagnostic {
	return nil
}

// VerifyAll 在逐 manifest 校验之上追加跨 manifest 所有权重叠检查
// （同一 from/to/legacy 路径不得被两份 manifest 声明）。
func (v *Verifier) VerifyAll(mods []*MoveManifest) []Diagnostic {
	return nil
}

// goList 是生产环境的 GoLister：在 Verifier.Root 下执行 `go list -e -json <pattern>`。
func goList(pattern string) ([]GoPackage, error) {
	return nil, fmt.Errorf("not implemented: modulemove.goList(%s)", pattern)
}

// mandatoryForbidden 是每份 manifest 都必须禁改的四个共享文件
// （moves/README.md §forbidden_shared_files；go.mod/go.sum/migrations/ 为额外项）。
var mandatoryForbidden = []string{
	"internal/router/router.go",
	"internal/router/task.go",
	"internal/router/sync_task.go",
	"internal/container/container.go",
}
