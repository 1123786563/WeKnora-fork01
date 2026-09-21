// Command modulemove 校验 Pass A 搬迁清单（docs/architecture/moves/<module>.yaml）。
//
// 用法:
//
//	go run ./tools/modulemove verify --module <id>   # 校验单个 manifest
//	go run ./tools/modulemove verify --all           # 校验全部 16 份 + 所有权重叠
//
// 任何违规都以排序后的诊断输出并以非零码退出。
package main

import (
	"flag"
	"fmt"
	"os"
)

const defaultRoot = "."

// ManifestsDir 是相对仓库根的 manifest 目录。
const ManifestsDir = "docs/architecture/moves"

func main() {
	root := flag.String("root", defaultRoot, "仓库根目录")
	all := flag.Bool("all", false, "校验全部 manifest 并检查所有权重叠")
	module := flag.String("module", "", "校验单个模块的 manifest")
	flag.Parse()

	if !*all && *module == "" {
		fmt.Fprintln(os.Stderr, "usage: modulemove verify [--all | --module <id>] [--root <repo>]")
		os.Exit(2)
	}

	var mods []*MoveManifest
	if *all {
		ids, err := ListManifestModules(*root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "modulemove: %v\n", err)
			os.Exit(2)
		}
		for _, id := range ids {
			m, err := LoadManifestStrict(ManifestPath(*root, id))
			if err != nil {
				fmt.Fprintf(os.Stderr, "modulemove: %v\n", err)
				os.Exit(2)
			}
			mods = append(mods, m)
		}
	} else {
		if !validModuleID(*module) {
			fmt.Fprintf(os.Stderr, "modulemove: 非法模块 id %q\n", *module)
			os.Exit(2)
		}
		m, err := LoadManifestStrict(ManifestPath(*root, *module))
		if err != nil {
			fmt.Fprintf(os.Stderr, "modulemove: %v\n", err)
			os.Exit(2)
		}
		mods = append(mods, m)
	}

	v := &Verifier{Root: *root, GoList: goList}
	var ds []Diagnostic
	if *all {
		ds = v.VerifyAll(mods)
	} else {
		ds = v.VerifyManifest(mods[0])
	}
	ds = SortDiagnostics(ds)
	for _, d := range ds {
		fmt.Println(d.String())
	}
	if len(ds) > 0 {
		os.Exit(1)
	}
	if *all {
		fmt.Printf("modulemove: OK (%d manifests verified)\n", len(mods))
	} else {
		fmt.Printf("modulemove: OK (%s)\n", mods[0].Module)
	}
}
