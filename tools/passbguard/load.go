package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// GovernanceDir 是治理文件目录（相对仓库根）。
const GovernanceDir = "docs/architecture/passb"

// ownershipMatrixFile 镜像 ownership-matrix.yaml 的顶层 schema（KnownFields 严格解码）。
type ownershipMatrixFile struct {
	LegacyFiles []LegacyOwnership `yaml:"legacy_files"`
	Aliases     []AliasOwnership  `yaml:"aliases"`
}

// contractsFile 镜像 contracts.yaml 的顶层 schema。
type contractsFile struct {
	Contracts []Contract `yaml:"contracts"`
}

// eventCatalogFile 镜像 event-catalog.yaml 的顶层 schema。
type eventCatalogFile struct {
	Events []Event `yaml:"events"`
}

// exceptionLedgerFile 镜像 exception-ledger.yaml 的顶层 schema。
type exceptionLedgerFile struct {
	Exceptions []Exception `yaml:"exceptions"`
}

// LoadGovernance 从仓库根加载四份治理文件并做结构校验。
// 治理文件在 B0 内逐任务生成：尚不存在的文件按空集处理（不报错）；
// 存在但含 schema 外字段或结构违规则返回 nil 与聚合错误。
// 返回的 Governance 各分片已按稳定 ID 排序（legacy/aliases 按路径，其余按 id）。
func LoadGovernance(root string) (*Governance, error) {
	var om ownershipMatrixFile
	if _, err := decodeStrict(governancePath(root, "ownership-matrix.yaml"), &om); err != nil {
		return nil, err
	}
	var cf contractsFile
	if _, err := decodeStrict(governancePath(root, "contracts.yaml"), &cf); err != nil {
		return nil, err
	}
	var ef eventCatalogFile
	if _, err := decodeStrict(governancePath(root, "event-catalog.yaml"), &ef); err != nil {
		return nil, err
	}
	var xf exceptionLedgerFile
	if _, err := decodeStrict(governancePath(root, "exception-ledger.yaml"), &xf); err != nil {
		return nil, err
	}

	g := &Governance{
		Legacy:     om.LegacyFiles,
		Aliases:    om.Aliases,
		Contracts:  cf.Contracts,
		Events:     ef.Events,
		Exceptions: xf.Exceptions,
	}
	normalizeGovernance(g)
	sortGovernance(g)
	diags := validate(g)
	if len(diags) > 0 {
		slices.Sort(diags)
		return nil, fmt.Errorf("passb governance: %d structural problem(s):\n%s", len(diags), strings.Join(diags, "\n"))
	}
	return g, nil
}

// governancePath 返回治理文件的绝对路径。
func governancePath(root, name string) string {
	return filepath.Join(root, filepath.FromSlash(GovernanceDir), name)
}

// decodeStrict 以 KnownFields(true) 严格解码一份治理文件。
// 文件不存在返回 present=false（空集）；空文件/仅注释同样按空集处理。
func decodeStrict(path string, out any) (present bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		if errors.Is(err, io.EOF) {
			return true, nil
		}
		return false, fmt.Errorf("strict decode %s: %w", path, err)
	}
	return true, nil
}

// normalizeRepoPath 把路径规范化为 slash 分隔的仓库相对形态：
// 重复剥去前导 "./"；其余非规范形态（反斜杠、绝对路径、"..")由 validate 拒绝。
func normalizeRepoPath(p string) string {
	for strings.HasPrefix(p, "./") {
		p = strings.TrimPrefix(p, "./")
	}
	return p
}

// normalizeGovernance 就地规范化所有路径形态字段。
func normalizeGovernance(g *Governance) {
	for i := range g.Legacy {
		g.Legacy[i].Path = normalizeRepoPath(g.Legacy[i].Path)
		g.Legacy[i].Destination = normalizeRepoPath(g.Legacy[i].Destination)
	}
	for i := range g.Aliases {
		g.Aliases[i].OldImportPath = normalizeRepoPath(g.Aliases[i].OldImportPath)
	}
	for i := range g.Contracts {
		for j, p := range g.Contracts[i].Consumers {
			g.Contracts[i].Consumers[j] = normalizeRepoPath(p)
		}
		for j, p := range g.Contracts[i].CharacterizationTests {
			g.Contracts[i].CharacterizationTests[j] = normalizeRepoPath(p)
		}
	}
	for i := range g.Events {
		for j, p := range g.Events[i].Consumers {
			g.Events[i].Consumers[j] = normalizeRepoPath(p)
		}
	}
	for i := range g.Exceptions {
		g.Exceptions[i].From = normalizeRepoPath(g.Exceptions[i].From)
		g.Exceptions[i].To = normalizeRepoPath(g.Exceptions[i].To)
	}
}

// sortGovernance 按稳定 ID 排序各分片（校验前执行，保证重复检测可依赖相邻性、
// 诊断顺序确定）。
func sortGovernance(g *Governance) {
	slices.SortFunc(g.Legacy, func(a, b LegacyOwnership) int {
		return strings.Compare(a.Path, b.Path)
	})
	slices.SortFunc(g.Aliases, func(a, b AliasOwnership) int {
		return strings.Compare(a.OldImportPath, b.OldImportPath)
	})
	slices.SortFunc(g.Contracts, func(a, b Contract) int {
		return strings.Compare(string(a.ID), string(b.ID))
	})
	slices.SortFunc(g.Events, func(a, b Event) int {
		return strings.Compare(string(a.ID), string(b.ID))
	})
	slices.SortFunc(g.Exceptions, func(a, b Exception) int {
		return strings.Compare(string(a.ID), string(b.ID))
	})
}
