// Package kbfreeze 以守卫测试机器强制知识子程序（Pass B B2，计划
// docs/plans/passb/20-knowledge-program.md）K0 冻结的两类不变量：
//
//  1. R0 类型单一事实源（计划 §5）：冻结的全部共享类型（Chunk/KnowledgeBase/
//     Tag/semantic 族）唯一事实源是 internal/types（含 internal/types/interfaces），
//     internal/modules/knowledge/** 内禁止出现同名/同形影子定义；K1-K4 搬迁
//     不得复制类型，类型变更走 conventions §5 升级。
//  2. B0 冻结的六个 capability-port 接口签名零漂移（contracts.yaml knowledge
//     契约区，stability: frozen）：ChunkService（knowledge.chunk-service）、
//     KnowledgeTagService（knowledge.tag-service）方法名集合精确相等；
//     KnowledgeService（knowledge.service）、KnowledgeBaseService
//     （knowledge.knowledge-base-service）、WikiPageService
//     （knowledge.wiki-page-service）、RetrieveEngineService
//     （knowledge.retrieve-engine-service）锚点方法存在。
//
// 本包仅含测试文件，无生产代码；依赖仅 internal/types/interfaces（模块公开
// 契约），不 import 任何 legacy 宿主包（架构守护合规，K0.2 验收项）。
package kbfreeze

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// frozenSharedTypes 是计划 §5 分配表的冻结类型名清单（R0：唯一事实源
// internal/types）。影子扫描对这些名字做 ^type\s+Name\b 行级匹配。
var frozenSharedTypes = []string{
	// Chunk 族（types/chunk.go，语义归属 K1）
	"Chunk", "ChunkType", "ChunkStatus", "ChunkFlags", "ImageInfo", "VideoInfo",
	// KnowledgeBase 族（types/knowledgebase.go，语义归属 K2）
	"KnowledgeBase", "KnowledgeBaseConfig", "AutoTagConfig", "ChunkingConfig", "StorageConfig",
	// Knowledge 族（types/knowledge.go，语义归属 K4）
	"Knowledge", "KnowledgeListFilter", "KnowledgeSearchScope",
	// Tag 族（types/tag.go，语义归属 K2）
	"KnowledgeTag", "KnowledgeTagWithStats", "TagReferenceCounts",
	// 检索面（types/search.go，语义归属 K2）
	"TagScope", "SearchResult", "SearchParams",
	// Wiki 族（types/wiki_page.go，语义归属 K3）
	"WikiPage", "WikiFolder",
	// semantic 族（types/semantic_model.go，语义归属 K2）
	"SemanticModelWireRequest", "SemanticModelMessage", "SemanticModelParameters",
	"SemanticModelCapability", "SemanticModelIssuedCapability",
	"SemanticModelInvocationResult", "SemanticModelInvocationDisposition",
	"SemanticModelInvocationClaim",
}

// shadowExemptions 是影子扫描豁免表（知识模块内 repo 相对路径 → 允许的本地
// 类型名）。唯一初始条目：chunker/splitter.go 的 Chunk 是 docreader 递归文本
// 切分器的分段概念（splitter.go:14 注释「Chunk represents a piece of split text
// with position tracking」；字段 Content/ContextHeader/Seq/Start），chunker 包
// 零 import internal/types，与 types.Chunk 无引用关系，属合法同名异义。
// 新增豁免条目 = 计划冻结表修订，必须走计划评审（计划 §8 K0.2）。
var shadowExemptions = map[string]map[string]bool{
	"internal/infrastructure/chunker/splitter.go": {"Chunk": true},
}

// kbfreezePaths 返回 (repoRoot, knowledgeRoot)：由本测试文件源码位置推导，
// 不依赖 go test 的工作目录。
func kbfreezePaths(t *testing.T) (string, string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位 freeze_test.go 源文件路径")
	}
	// freeze_test.go = <repoRoot>/internal/modules/knowledge/kbfreeze/freeze_test.go
	dir := filepath.Dir(thisFile)
	for i := 0; i < 4; i++ {
		dir = filepath.Dir(dir)
	}
	repoRoot := dir
	return repoRoot, filepath.Join(repoRoot, "internal", "modules", "knowledge")
}

// TestKnowledgeSharedTypesHaveNoShadowDefinitions 强制 R0：遍历
// internal/modules/knowledge/** 全部 .go 文件（跳过 kbfreeze 守卫包自身与
// _test.go），对冻结类型名做 ^type\s+Name\b 行级匹配；命中且不在豁免表内即失败。
func TestKnowledgeSharedTypesHaveNoShadowDefinitions(t *testing.T) {
	repoRoot, knowledgeRoot := kbfreezePaths(t)
	kbfreezeDir := filepath.Join(knowledgeRoot, "kbfreeze")

	// 单一组合正则；备选项按长度降序排列，保证前缀名（Chunk/ChunkType 等）
	// 先尝试更长候选，\b 阻止前缀误命中。
	alternatives := append([]string(nil), frozenSharedTypes...)
	sort.Slice(alternatives, func(i, j int) bool { return len(alternatives[i]) > len(alternatives[j]) })
	for i, name := range alternatives {
		alternatives[i] = regexp.QuoteMeta(name)
	}
	shadowRe, err := regexp.Compile(`^type\s+(` + strings.Join(alternatives, "|") + `)\b`)
	if err != nil {
		t.Fatalf("编译影子扫描正则失败: %v", err)
	}

	err = filepath.WalkDir(knowledgeRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path == kbfreezeDir {
				return fs.SkipDir // 跳过守卫包自身
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		relPath, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return relErr
		}
		relPath = filepath.ToSlash(relPath)
		for lineNo, line := range strings.Split(string(data), "\n") {
			m := shadowRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			typeName := m[1]
			if shadowExemptions[relPath][typeName] {
				continue
			}
			t.Errorf("%s:%d: 影子类型定义 %q 违反 R0（唯一事实源 internal/types，"+
				"见 docs/plans/passb/20-knowledge-program.md §5；如属合法同名异义，"+
				"须新增豁免并走计划评审）", relPath, lineNo+1, typeName)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历 %s 失败: %v", knowledgeRoot, err)
	}
}

// TestFrozenCapabilityPortInterfacesUnchanged 强制 B0 冻结的六个 capability-port
// 接口签名零漂移（contracts.yaml knowledge 契约区，stability: frozen）：
// ChunkService 19 方法与 KnowledgeTagService 7 方法为集合精确断言（reflect
// NumMethod 含内嵌接口提升方法，两接口实测无内嵌）；其余四端口锚点方法存在。
func TestFrozenCapabilityPortInterfacesUnchanged(t *testing.T) {
	// knowledge.chunk-service（contracts.yaml:1591；internal/types/interfaces/chunk.go:142-190）
	// 2026-09 #3694 image gallery 增补 ListImagesByKnowledgeBaseID（只读扩展，
	// 实现于 service/chunk.go:229，消费方 handler/knowledge.go:1253）。
	assertExactMethodSet(t, "knowledge.chunk-service",
		reflect.TypeOf((*interfaces.ChunkService)(nil)).Elem(), []string{
			"CreateChunks", "GetChunkByID", "GetChunkByIDOnly",
			"ListChunksByKnowledgeID", "ListPagedChunksByKnowledgeID",
			"UpdateChunk", "UpdateChunks",
			"DeleteChunk", "DeleteChunks", "DeleteChunksByKnowledgeID", "DeleteByKnowledgeList",
			"ListChunkByParentID", "GetRepository",
			"DeleteGeneratedQuestion", "UpdateDocumentChunk", "ListChunkRevisions",
			"RevertDocumentChunk", "UpsertGeneratedQuestion",
			"ListImagesByKnowledgeBaseID",
		})

	// knowledge.tag-service（contracts.yaml:1843；internal/types/interfaces/tag.go:11-31）
	assertExactMethodSet(t, "knowledge.tag-service",
		reflect.TypeOf((*interfaces.KnowledgeTagService)(nil)).Elem(), []string{
			"ListTags", "CreateTag", "UpdateTag", "DeleteTag",
			"FindOrCreateTagByName", "DeleteOrphanTagByName", "ProcessIndexDelete",
		})

	// knowledge.service（contracts.yaml:1773；internal/types/interfaces/knowledge.go:13）
	assertMethodsExist(t, "knowledge.service",
		reflect.TypeOf((*interfaces.KnowledgeService)(nil)).Elem(), []string{
			"CreateKnowledgeFromFile", "ProcessDocument", "SearchKnowledgeForScopes",
		})

	// knowledge.knowledge-base-service（contracts.yaml:1627；internal/types/interfaces/knowledgebase.go:18）
	assertMethodsExist(t, "knowledge.knowledge-base-service",
		reflect.TypeOf((*interfaces.KnowledgeBaseService)(nil)).Elem(), []string{
			"HybridSearch", "ResolveEmbeddingModelKeys", "ProcessKBDelete",
		})

	// knowledge.wiki-page-service（contracts.yaml:1860；internal/types/interfaces/wiki_page.go:12）
	assertMethodsExist(t, "knowledge.wiki-page-service",
		reflect.TypeOf((*interfaces.WikiPageService)(nil)).Elem(), []string{
			"RepairContentLinks", "RevertPageToVersion", "PruneEmptyFolderChains",
		})

	// knowledge.retrieve-engine-service（contracts.yaml:1726；internal/types/interfaces/retriever.go:102）：
	// 只断言自有 9 方法；内嵌 RetrieveEngine 的方法不参与断言。
	assertMethodsExist(t, "knowledge.retrieve-engine-service",
		reflect.TypeOf((*interfaces.RetrieveEngineService)(nil)).Elem(), []string{
			"Index", "BatchIndex", "EstimateStorageSize", "CopyIndices",
			"DeleteByChunkIDList", "DeleteBySourceIDList", "DeleteByKnowledgeIDList",
			"BatchUpdateChunkEnabledStatus", "BatchUpdateChunkTagID",
		})
}

// assertExactMethodSet 断言接口类型的方法名集合与冻结清单精确相等
// （多、少、改名均失败）。
func assertExactMethodSet(t *testing.T, port string, typ reflect.Type, expected []string) {
	t.Helper()
	actual := make([]string, 0, typ.NumMethod())
	for i := 0; i < typ.NumMethod(); i++ {
		actual = append(actual, typ.Method(i).Name)
	}
	sort.Strings(actual)
	want := append([]string(nil), expected...)
	sort.Strings(want)
	if !reflect.DeepEqual(actual, want) {
		t.Errorf("端口 %s（%s）方法集合漂移：冻结清单 %v，实测 %v", port, typ, want, actual)
	}
}

// assertMethodsExist 断言接口类型上存在全部冻结锚点方法。
func assertMethodsExist(t *testing.T, port string, typ reflect.Type, required []string) {
	t.Helper()
	for _, name := range required {
		if _, ok := typ.MethodByName(name); !ok {
			t.Errorf("端口 %s（%s）缺少冻结锚点方法 %s", port, typ, name)
		}
	}
}
