// Command architectureguard 是 `make check-backend-architecture` 的执行体：
// 以 F0 基线（docs/architecture/backend-baseline.md §3.2）同一口径发现服务端资产，
// 校验 manifest 所有权覆盖、模块间禁互导、注册唯一性与横向目录遗留文件守卫。
//
// 用法:
//
//	go run ./tools/architectureguard [--root <repo>]
//
// 任何违规都以排序后的诊断输出并以非零码退出；当前 HEAD（未搬迁）必须为 0 违规。
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	root := flag.String("root", ".", "仓库根目录")
	flag.Parse()

	mods, err := LoadManifestViews(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "architectureguard: %v\n", err)
		os.Exit(2)
	}
	rep, err := Run(*root, mods)
	if err != nil {
		fmt.Fprintf(os.Stderr, "architectureguard: %v\n", err)
		os.Exit(2)
	}
	printReport(os.Stdout, rep)
	if len(rep.Diagnostics) > 0 {
		os.Exit(1)
	}
}

func printReport(f *os.File, rep Report) {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "architectureguard: literal=%d apiKeyRoute=%d handle=%d total=%d | "+
		"redis=%d lite=%d | hooks=%d | modules=%d\n",
		rep.Summary.RoutesLiteral, rep.Summary.RoutesAPIKey, rep.Summary.RoutesHandle, rep.Summary.RoutesTotal,
		rep.Summary.WorkerRedis, rep.Summary.WorkerLite, rep.Summary.Hooks, rep.Summary.ModulesScanned)
	for _, d := range rep.Diagnostics {
		b.WriteString(d.String())
		b.WriteString("\n")
	}
	if len(rep.Diagnostics) == 0 {
		b.WriteString("architectureguard: OK (0 violations)\n")
	}
	_, _ = fmt.Fprint(f, b.String())
}
