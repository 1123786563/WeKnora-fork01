// Package main 实现 passbguard CLI（B0.6，freeze:316-381）：聚合 B0.1–B0.5
// 的全部校验——结构 schema（LoadGovernance）、legacy/alias/exception 覆盖与
// 属主（CheckOwnership）、能力/组合契约（CheckContracts）、B0.3 裁定与 brief
// 认领（CheckRulings/CheckBriefClaims）、歧义措辞扫描（CheckAmbiguity）。
// 零诊断时打印冻结就绪行并以 0 退出；否则按 `check: path: message` 逐行输出
// 排序后的诊断到 stderr 并以 1 退出。本目录自 B0.6 起是 package main
// （`go run ./tools/passbguard -root .` 要求），模型与校验逻辑保持库函数形态
// 供同包测试直接消费。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

// ReadinessReport 是 CLI 成功路径打印的稳定计数集合（freeze B0.6 Step 2 的
// 字段集：legacy/aliases/exceptions/contracts/events/overlaps/missing）。
// Overlaps 计 legacy-overlap 诊断（同一仓库路径被两个计划认领）；
// Missing 计 legacy-missing / alias-missing / exception-missing 诊断
// （manifest/guard 发现集里没有治理属主的条目）。
type ReadinessReport struct {
	Legacy     int
	Aliases    int
	Exceptions int
	Contracts  int
	Events     int
	Overlaps   int
	Missing    int
}

// RunPassBGuard 对仓库根执行完整就绪校验并汇总报告与全量诊断。
// 诊断已排序去重；空切片即就绪。结构性加载/发现错误返回 error（非诊断）。
func RunPassBGuard(root string) (*ReadinessReport, []Diagnostic, error) {
	g, err := LoadGovernance(root)
	if err != nil {
		return nil, nil, err
	}
	d, err := DiscoverPassB(root)
	if err != nil {
		return nil, nil, err
	}

	var all []Diagnostic
	all = append(all, CheckOwnership(g, d)...)
	all = append(all, CheckContracts(g, d)...)
	amb, err := CheckAmbiguity(root)
	if err != nil {
		return nil, nil, err
	}
	all = append(all, amb...)
	all = append(all, CheckRulings(g)...)
	brief, err := CheckBriefClaims(root)
	if err != nil {
		return nil, nil, err
	}
	all = append(all, brief...)
	diags := sortDiagnosticsDiag(all)

	report := &ReadinessReport{
		Legacy:     len(g.Legacy),
		Aliases:    len(g.Aliases),
		Exceptions: len(g.Exceptions),
		Contracts:  len(g.Contracts),
		Events:     len(g.Events),
	}
	for _, dd := range diags {
		switch dd.Check {
		case "legacy-overlap":
			report.Overlaps++
		case "legacy-missing", "alias-missing", "exception-missing":
			report.Missing++
		}
	}
	return report, diags, nil
}

// FormatDiagnostic 渲染 CLI 诊断行 `check: path: message`（freeze B0.6 Step 1；
// path 为空时省略中段，如 contract-baseline-ledger）。
func FormatDiagnostic(d Diagnostic) string {
	if d.Path == "" {
		return fmt.Sprintf("%s: %s", d.Check, d.Message)
	}
	return fmt.Sprintf("%s: %s: %s", d.Check, d.Path, d.Message)
}

// run 执行 CLI：args 为进程参数（不含程序名），输出写 stdout/stderr（测试
// 注入），返回进程退出码（0 就绪 / 1 诊断或错误 / 2 参数非法）。
func run(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("passbguard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	report, diags, err := RunPassBGuard(*root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "passbguard: %v\n", err)
		return 1
	}
	if len(diags) > 0 {
		lines := make([]string, 0, len(diags))
		for _, d := range diags {
			lines = append(lines, FormatDiagnostic(d))
		}
		sort.Strings(lines)
		for _, line := range lines {
			_, _ = fmt.Fprintln(stderr, line)
		}
		return 1
	}
	_, _ = fmt.Fprintf(stdout,
		"pass-b readiness: legacy=%d aliases=%d exceptions=%d contracts=%d "+
			"events=%d overlaps=%d missing=%d\n",
		report.Legacy, report.Aliases, report.Exceptions, report.Contracts,
		report.Events, report.Overlaps, report.Missing)
	return 0
}

func main() {
	os.Exit(run(os.Stdout, os.Stderr, os.Args[1:]))
}
