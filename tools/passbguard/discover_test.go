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
