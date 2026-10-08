package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// readinessLineRE 是 B0.6 Step 2 冻结的就绪行（freeze:341-347）：
// 字段、顺序、拼写不得漂移；成功路径只输出这一行。
var readinessLineRE = regexp.MustCompile(
	`^pass-b readiness: legacy=(\d+) aliases=(\d+) exceptions=(\d+) contracts=(\d+) ` +
		`events=(\d+) overlaps=(\d+) missing=(\d+)$`)

// TestCLISuccessPrintsStableTotalsAndExitsZero 覆盖 freeze B0.6 Step 1 前半：
// 成功时打印稳定总量行并以 0 退出。计数期望不从字面量读取（conventions §8 /
// freeze:349 修正）：legacy/exceptions 参数化自 Pass A 验收台账（当前基线
// 396/105，台账注明实测命令与日期），aliases 取 manifest 发现值，
// contracts/events 由本测试独立解析 YAML（不经 passbguard 加载器）。
func TestCLISuccessPrintsStableTotalsAndExitsZero(t *testing.T) {
	root := repoRootFromTest(t)

	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr, []string{"-root", root})
	require.Equal(t, 0, code, "stderr:\n%s", stderr.String())
	require.Empty(t, stderr.String(), "成功路径不得写 stderr")

	line := strings.TrimSpace(stdout.String())
	m := readinessLineRE.FindStringSubmatch(line)
	require.NotNil(t, m, "stdout 必须是冻结就绪行，got %q", line)

	got := map[string]int{}
	for i, key := range []string{"legacy", "aliases", "exceptions", "contracts", "events", "overlaps", "missing"} {
		n, err := strconv.Atoi(m[i+1])
		require.NoError(t, err)
		got[key] = n
	}

	// legacy/exceptions：与 Pass A 验收台账一致（框架冻结基线 396/105 的参数化来源）。
	wantLegacy, wantExceptions := acceptanceLedgerCount(t, root)
	require.Equal(t, wantLegacy, got["legacy"], "legacy 计数必须等于台账记录值")
	require.Equal(t, wantExceptions, got["exceptions"], "exceptions 计数必须等于台账记录值")

	// overlaps/missing 必须为零（freeze B0.6 Step 3 预期）。
	require.Zero(t, got["overlaps"], "overlaps 必须为 0")
	require.Zero(t, got["missing"], "missing 必须为 0")

	// aliases：与 manifest 发现的 alias 义务集相等。
	d, err := DiscoverPassB(root)
	require.NoError(t, err)
	require.Equal(t, len(d.Aliases), got["aliases"], "aliases 计数必须等于发现值")

	// contracts/events：独立解析治理 YAML（KnownFields 关闭、不经 passbguard 模型）。
	require.Equal(t, governanceYAMLCount(t, root, "contracts.yaml", "contracts"), got["contracts"],
		"contracts 计数必须等于独立解析值")
	require.Equal(t, governanceYAMLCount(t, root, "event-catalog.yaml", "events"), got["events"],
		"events 计数必须等于独立解析值")
}

// TestCLIDiagnosticsPrintSortedCheckPathMessageAndExitsOne 覆盖 freeze B0.6
// Step 1 后半：存在诊断时按 `check: path: message` 逐行输出到 stderr（排序）、
// 不写 stdout、以 1 退出。fixture 根构造最小仓库事实（1 份 manifest 声明
// 1 个 legacy 文件 + 1 条 alias 义务、1 条 guard importException），治理文件
// 缺失即产生 legacy-missing / alias-missing / exception-missing 等诊断。
func TestCLIDiagnosticsPrintSortedCheckPathMessageAndExitsOne(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, filepath.Join(root, "docs/architecture/moves/knowledge.yaml"), `module: knowledge
description: fixture manifest for CLI diagnostics
legacy_files:
  - path: internal/application/repository/widget.go
    reason: fixture legacy file
    passb_task: B-knowledge
alias_obligations:
  - old_import_path: internal/application/service/widgets
    passb_task: B-knowledge
`)
	writeMainTestFile(t, filepath.Join(root, "internal/application/repository/widget.go"),
		"package repository\n")
	writeMainTestFile(t, filepath.Join(root, "tools/architectureguard/check.go"), `package architectureguard

type importException struct {
	ImporterFile string
	ImportedPath string
	Reason       string
	PassBTask    string
}

var importExceptions = []importException{
	{
		ImporterFile: "internal/agentruntime/agent/engine.go",
		ImportedPath: "github.com/Tencent/WeKnora/internal/airesource/models/chat",
		Reason:       "fixture 预存横向包耦合",
		PassBTask:    "B-agentruntime",
	},
}
`)
	// 治理目录存在但四份治理文件缺失：合法空集（B0.1 语义），由各 check 报诊断。
	require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.FromSlash(GovernanceDir)), 0o755))

	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr, []string{"-root", root})
	require.Equal(t, 1, code, "存在诊断必须以 1 退出")
	require.Empty(t, stdout.String(), "失败路径不得写 stdout")

	out := stderr.String()
	require.NotEmpty(t, out)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	for _, line := range lines {
		require.NotEmpty(t, line, "不得输出空行")
		check := strings.SplitN(line, ":", 2)[0]
		require.Regexp(t, `^[a-z][a-z-]*$`, check,
			"诊断行必须是 `check: path: message`，got %q", line)
	}
	sorted := append([]string(nil), lines...)
	sort.Strings(sorted)
	require.Equal(t, sorted, lines, "诊断行必须排序输出")

	// fixture 的三类缺属主诊断必须出现在输出中（check 前缀可机器分派）。
	require.Contains(t, out, "legacy-missing: internal/application/repository/widget.go:")
	require.Contains(t, out, "alias-missing: internal/application/service/widgets:")
	require.Contains(t, out, "exception-missing: internal/agentruntime/agent/engine.go:")
	// OCR R1 #15：event-catalog.yaml 缺失（空集）必须报空目录诊断，
	// 不得静默通过。
	require.Contains(t, out, "event-catalog-empty:")
}

// governanceYAMLCount 用普通 yaml.Unmarshal（KnownFields 关闭）独立计数一份
// 治理文件顶层清单的行数——不经过 passbguard 自身的加载与模型。
func governanceYAMLCount(t *testing.T, root, name, topKey string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(GovernanceDir), name))
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(data, &doc))
	list, ok := doc[topKey].([]any)
	require.True(t, ok, "%s 缺少顶层键 %s", name, topKey)
	return len(list)
}

// writeMainTestFile 写入 fixture 文件（含父目录）。
func writeMainTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
