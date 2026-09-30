package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeDiscoverFixture 在 root 下写入一个 fixture 文件（含父目录）。
func writeDiscoverFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// wolfFixture 是语法毒丸：一旦被 discoverGoTree 走查解析即 parse 报错，
// 用于证明点前缀目录被整体跳过而非"恰好没有 .go 文件"。
const wolfFixture = "package wolf\nfunc broken( {\n"

// TestDiscoverGoTreeSkipsDotPrefixedDirs 覆盖 OCR R1 #8：本仓工作流把 git
// worktree 放在仓库根 .worktrees/ 下（worktree 内 .git 是文件非目录，仅按
// 目录名 ".git" SkipDir 命中不了），发现树必须整体跳过一切点前缀目录与
// testdata，只保留正常树上可解析的 .go 文件。
func TestDiscoverGoTreeSkipsDotPrefixedDirs(t *testing.T) {
	root := t.TempDir()
	writeDiscoverFixture(t, root, "internal/application/repository/widget.go", "package repository\n")
	writeDiscoverFixture(t, root, "pkg/lib/lib.go", "package lib\nimport \"fmt\"\n")
	// 点前缀目录（任意名字）整体跳过：内放语法毒丸，被走查即失败。
	writeDiscoverFixture(t, root, ".worktrees/passb-b0/wolf.go", wolfFixture)
	writeDiscoverFixture(t, root, ".superpowers/sdd/wolf.go", wolfFixture)
	writeDiscoverFixture(t, root, ".githooks/wolf.go", wolfFixture)
	writeDiscoverFixture(t, root, "internal/.hidden/wolf.go", wolfFixture)
	// .git 目录形态（普通检出）同样必须跳过。
	writeDiscoverFixture(t, root, ".git/hooks/wolf.go", wolfFixture)
	// testdata 惯例跳过保持不变（Go 工具链不构建 testdata）。
	writeDiscoverFixture(t, root, "internal/application/repository/testdata/golden.go", wolfFixture)

	files, imports, err := discoverGoTree(root)
	require.NoError(t, err)
	require.Equal(t, []string{
		"internal/application/repository/widget.go",
		"pkg/lib/lib.go",
	}, files)
	require.Equal(t, map[string][]string{
		"internal/application/repository/widget.go": nil, // 零 import 解析为 nil 切片
		"pkg/lib/lib.go": {"fmt"},
	}, imports)
}

// TestDiscoverImportExceptionsRequiresCompositeLiteral 覆盖 OCR R1 #f3：
// architectureguard 若把 importExceptions 改为无值声明（var importExceptions
// []importException）或非复合字面量初始化，发现必须返回干净错误而非以
// vs.Values[0] 越界 panic 丢失全部诊断。
func TestDiscoverImportExceptionsRequiresCompositeLiteral(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			// 无值声明：vs.Values 为空，索引前必须防护。
			name: "valueless declaration",
			source: `package architectureguard

type importException struct {
	ImporterFile string
	ImportedPath string
	Reason       string
	PassBTask    string
}

var importExceptions []importException
`,
			wantErr: "importExceptions 声明缺少初始化复合字面量",
		},
		{
			// 非字面量初始化（Ident 引用）：不是 CompositeLit，返回既有错误。
			name: "non-literal initializer",
			source: `package architectureguard

type importException struct {
	ImporterFile string
}

var exceptionRows = []importException{{ImporterFile: "a.go"}}

var importExceptions = exceptionRows
`,
			wantErr: "importExceptions 不是复合字面量",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "check.go")
			writeDiscoverFixture(t, root, "check.go", tc.source)
			_, err := discoverImportExceptions(path)
			require.Error(t, err, "无值/非字面量声明必须返回错误")
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestReferencesSymbolIgnoresSameNameBindings 覆盖 OCR R1 #05：同包（sameDir）
// 文件里与符号同名的绑定位置标识符不是引用——字段名（Field.Names）、局部
// 变量声明（AssignStmt 左侧 :=）、复合字面量键（KeyValueExpr.Key）均不得
// 计为消费方；真实的裸引用（类型/表达式位置）仍必须命中。
// 该发现驱动 contract-consumer-unrecorded/vanished 诊断，同包重构引入同名
// 标识符不应触发误报。
func TestReferencesSymbolIgnoresSameNameBindings(t *testing.T) {
	root := t.TempDir()
	writeDiscoverFixture(t, root, "internal/svc/svc.go", `package svc

// SessionService 是同包冻结符号 fixture。
type SessionService interface{ Boot() }
`)
	// shadow.go 与定义文件同目录同包：仅含同名的绑定位置标识符，无真实引用。
	writeDiscoverFixture(t, root, "internal/svc/shadow.go", `package svc

type holderConfig struct{ Booted bool }

// 字段名与符号同名（Field.Names 绑定位置）：不是对 SessionService 的引用。
type Holder struct {
	SessionService *holderConfig
}

type cfg struct{ SessionService int }

func shadow() {
	// 局部变量声明与符号同名（AssignStmt 左侧 := 绑定）：不是引用。
	SessionService := 1
	// 复合字面量键与符号同名（KeyValueExpr.Key 绑定）：不是引用。
	_ = cfg{SessionService: 2}
}

var _ = Holder{}
`)
	// user.go 是阳性对照：类型位置的真实裸引用必须命中。
	writeDiscoverFixture(t, root, "internal/svc/user.go", `package svc

var booter SessionService
`)

	files, imports, err := discoverGoTree(root)
	require.NoError(t, err)
	d := &Discovery{Root: root, GoFiles: files, Imports: imports}

	fact, found, err := DiscoverSymbol(root, "internal/svc/svc.go", "SessionService")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "github.com/Tencent/WeKnora/internal/svc", fact.ImportPath)

	consumers, tests, err := DiscoverSymbolConsumers(d, fact)
	require.NoError(t, err)
	require.Equal(t, []string{"internal/svc/user.go"}, consumers,
		"同名绑定位置不得计为消费方，真实裸引用必须命中")
	require.Empty(t, tests)
}

// TestReferencesSymbolCountsMapKeyAndAssignAsUsages 覆盖 OCR R3：
// bindingIdentPositions 曾把两类真实使用误计为绑定，referencesSymbol 对
// 同包消费方漏检（违背"宁可多报 unrecorded 也不漏报真实消费方"的保守策略）：
// (1) map 复合字面量裸键（map[Priority]int{PriorityHigh: 1}）是值表达式；
// (2) = 赋值（token.ASSIGN，非 :=）左侧是对既有名字的使用。
// mapkey.go 与 assign.go 各只含一处被误计的使用，不得再漏检。
// 另锚定保守方向：键的具名类型（elsewhere）声明在同包另一文件时，消费方
// 文件内无法精确区分 struct/map，键按 map 处理——计为使用（落多报方向）。
func TestReferencesSymbolCountsMapKeyAndAssignAsUsages(t *testing.T) {
	root := t.TempDir()
	writeDiscoverFixture(t, root, "internal/svc/priority.go", `package svc

// Priority 与 PriorityHigh 是同包冻结符号 fixture。
type Priority int

var PriorityHigh Priority = 1
`)
	// mapkey.go 仅含 map 裸键这一处符号使用：修复前被计为绑定而漏检。
	writeDiscoverFixture(t, root, "internal/svc/mapkey.go", `package svc

var weights = map[Priority]int{PriorityHigh: 1}
`)
	// assign.go 仅含 = 左侧这一处符号使用：修复前被计为绑定而漏检。
	writeDiscoverFixture(t, root, "internal/svc/assign.go", `package svc

func bump() {
	PriorityHigh = 2
}
`)
	// other.go + unknown.go：键的具名类型声明在同包另一文件，本文件内
	// 无法精确区分 struct/map → 按 map 处理（键计为使用，落多报方向）。
	writeDiscoverFixture(t, root, "internal/svc/other.go", `package svc

type elsewhere struct{ PriorityHigh int }
`)
	writeDiscoverFixture(t, root, "internal/svc/unknown.go", `package svc

var row = elsewhere{PriorityHigh: 1}
`)

	files, imports, err := discoverGoTree(root)
	require.NoError(t, err)
	d := &Discovery{Root: root, GoFiles: files, Imports: imports}

	fact, found, err := DiscoverSymbol(root, "internal/svc/priority.go", "PriorityHigh")
	require.NoError(t, err)
	require.True(t, found)

	consumers, tests, err := DiscoverSymbolConsumers(d, fact)
	require.NoError(t, err)
	require.Equal(t, []string{
		"internal/svc/assign.go",
		"internal/svc/mapkey.go",
		"internal/svc/unknown.go",
	}, consumers, "map 裸键、= 左侧与无法精确区分的键均须计为消费方（真实使用方向）")
	require.Empty(t, tests)
}

// TestDiscoverGoTreeKeepsRootItself 确保 SkipDir 收紧到点前缀目录后不误伤
// 根目录本身：`-root .`（根回调 entry.Name() == "."）与点前缀根名都必须照常
// 发现（make check-passb-readiness 以 `-root .` 运行）。
func TestDiscoverGoTreeKeepsRootItself(t *testing.T) {
	parent := t.TempDir()
	dotRoot := filepath.Join(parent, ".hiddencockpit")
	writeDiscoverFixture(t, dotRoot, "internal/app/main.go", "package main\n")
	files, _, err := discoverGoTree(dotRoot)
	require.NoError(t, err)
	require.Equal(t, []string{"internal/app/main.go"}, files)

	// 相对根 "."：chdir 到小 fixture 目录后走查，正常文件必须被发现。
	// 同时放入 worktree 形态的 .git 文件（gitdir 指针，非 .go 后缀）——不干扰遍历。
	relRoot := filepath.Join(parent, "relroot")
	writeDiscoverFixture(t, relRoot, "internal/app/main.go", "package main\n")
	require.NoError(t, os.WriteFile(filepath.Join(relRoot, ".git"), []byte("gitdir: /elsewhere/.git\n"), 0o644))
	t.Chdir(relRoot)
	files, _, err = discoverGoTree(".")
	require.NoError(t, err)
	require.Equal(t, []string{"internal/app/main.go"}, files)
}
