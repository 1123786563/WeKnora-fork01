package tools

// R2.1 契约快照（特征化测试，节点 b3-r-tools，计划 docs/plans/passb/32-agentruntime-tools.md §1 Task R2.1）。
//
// 用途：在 R2.2–R2.4 深 import 收敛（消费侧 seam 化）之前，把工具子系统的四个外部
// 可观察契约冻结为字节级基线（conventions §6 前拍）：
//  1. AvailableToolDefinitions()（definitions.go:85）——UI 可见工具目录整表；
//  2. DefaultAllowedTools()（definitions.go:116）——默认允许清单精确切片；
//  3. persistStripFields / persistStripFieldsByTool / clientStripFieldsByTool
//     （persist.go:11-35）——SSE/持久化剥离三表；
//  4. 可零依赖构造工具的 BaseTool.Parameters()（tool.go:37）JSON 字节 SHA-256。
//
// 特征化语义：期望值由基线实现的实测输出生成（锚定旧行为）。R2.2–R2.4 的 seam 化
// 是纯别名/薄委托（类型同一性保持），这些快照必须逐字节不变——任何一个变红即说明
// 收敛改变了外部契约（brief 义务 2 违约），立即停下排查。
//
// MCP/wiki 族的 schema 由既有 mcp_catalog_regression_test.go / wiki_*_test.go 覆盖，
// 此处不重复构造（计划 R2.1 步骤 4 同口径）。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
)

// TestAvailableToolDefinitionsContractSnapshot freezes the full UI tool catalog.
//
// 2026-10-06 re-baseline: the upstream merge unified the retrieval surface
// (search_knowledge / read_document / list_documents replace the retired
// grep_chunks / knowledge_search / list_knowledge_chunks / get_document_info
// legacy identifiers; wiki_read_source_doc retired with them — see the
// LegacyTool* block in definitions.go). The catalog below freezes the
// post-merge contract; the pre-merge 21-entry snapshot stayed red because it
// predated that intentional change.
func TestAvailableToolDefinitionsContractSnapshot(t *testing.T) {
	want := []AvailableTool{
		{Name: "thinking", Label: "思考", Description: "动态和反思性的问题解决思考工具"},
		{Name: "todo_write", Label: "制定计划", Description: "创建结构化的研究计划"},
		{Name: "search_knowledge", Label: "检索知识库", Description: "语义、关键词或混合检索知识库分块"},
		{Name: "read_document", Label: "阅读文档", Description: "读取文档元数据与分块内容，支持分页和文内查找"},
		{Name: "list_documents", Label: "浏览文档列表", Description: "分页列出知识库中的文档"},
		{Name: "query_knowledge_graph", Label: "查询知识图谱", Description: "从知识图谱中查询关系"},
		{Name: "search_conversations", Label: "回顾历史对话", Description: "在用户自己的历史会话中查找之前聊过的内容"},
		{Name: "database_query", Label: "查询数据库", Description: "查询数据库中的信息"},
		{Name: "data_analysis", Label: "数据分析", Description: "理解数据文件并进行数据分析"},
		{Name: "data_schema", Label: "查看数据元信息", Description: "获取表格文件的元信息"},
		{Name: "wiki_read_page", Label: "读取Wiki页面", Description: "读取指定的Wiki页面内容"},
		{Name: "wiki_search", Label: "搜索Wiki", Description: "在Wiki中搜索页面"},
		{Name: "wiki_flag_issue", Label: "标记Wiki问题", Description: "标记页面中存在的事实错误或合并冲突问题"},
		{Name: "wiki_write_page", Label: "创建/覆盖Wiki", Description: "创建新页面或完全覆盖已有页面"},
		{Name: "wiki_replace_text", Label: "局部替换Wiki", Description: "替换Wiki页面中的特定文本"},
		{Name: "wiki_rename_page", Label: "重命名Wiki", Description: "重命名Wiki页面并自动更新关联链接"},
		{Name: "wiki_delete_page", Label: "删除Wiki", Description: "删除Wiki页面并自动清理关联死链"},
		{Name: "wiki_read_issue", Label: "查看Wiki问题", Description: "查看特定的Wiki页面问题详情"},
		{Name: "wiki_update_issue", Label: "更新Wiki问题状态", Description: "更新特定的Wiki页面问题状态"},
	}
	got := AvailableToolDefinitions()
	if len(got) != len(want) {
		t.Fatalf("AvailableToolDefinitions() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AvailableToolDefinitions()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestDefaultAllowedToolsSnapshot freezes the exact default allowlist order.
// Re-baselined with the catalog above: the unified retrieval names replace the
// retired knowledge_search / grep_chunks / list_knowledge_chunks /
// get_document_info entries.
func TestDefaultAllowedToolsSnapshot(t *testing.T) {
	want := []string{
		"search_knowledge",
		"read_document",
		"list_documents",
		"search_conversations",
	}
	got := DefaultAllowedTools()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultAllowedTools() = %q, want %q", got, want)
	}
}

// TestPersistStripTablesSnapshot freezes the three strip-field tables
// (key sets and value slices, order-insensitive keys / order-sensitive slices).
func TestPersistStripTablesSnapshot(t *testing.T) {
	wantPersistStrip := map[string][]string{
		"knowledge_chunks_list": {"chunks"},
		"grep_results":          {"chunk_results"},
	}
	wantByTool := map[string][]string{
		"read_file":          {"content", "content_base64", "instructions"},
		"shell_exec":         {"content", "content_base64"},
		"read_sandbox_file":  {"content", "content_base64"},
		"write_sandbox_file": {"content", "content_base64"},
		"edit_sandbox_file":  {"content", "content_base64"},
	}
	if !reflect.DeepEqual(persistStripFields, wantPersistStrip) {
		t.Errorf("persistStripFields = %v, want %v", persistStripFields, wantPersistStrip)
	}
	if !reflect.DeepEqual(persistStripFieldsByTool, wantByTool) {
		t.Errorf("persistStripFieldsByTool = %v, want %v", persistStripFieldsByTool, wantByTool)
	}
	if !reflect.DeepEqual(clientStripFieldsByTool, wantByTool) {
		t.Errorf("clientStripFieldsByTool = %v, want %v", clientStripFieldsByTool, wantByTool)
	}
}

// TestConstructibleToolSchemaBytesSnapshot freezes the JSON schema bytes of
// every zero-dependency constructible tool as a SHA-256 digest. The craft
// delegate gets a stub Delegate (never invoked: only Parameters() is read).
func TestConstructibleToolSchemaBytesSnapshot(t *testing.T) {
	craftCfg := CraftDelegateToolConfig{
		Scope:       craft.Scope{TenantID: 1, UserID: "u", SessionID: "s"},
		WorkspaceID: "w",
		Delegate: func(context.Context, craft.Task) (craft.Result, error) {
			return craft.Result{}, nil
		},
	}
	craftTool, err := NewCraftDelegateTool(craftCfg)
	if err != nil {
		t.Fatalf("NewCraftDelegateTool: %v", err)
	}

	cases := []struct {
		name    string
		tool    interface{ Parameters() json.RawMessage }
		wantSHA string
	}{
		{"app_connector", NewAppConnectorTool(nil), "dea9299165858a2359a011aae0e5306b57ed47a34caff95e91ad02c552d0d55e"},
		{"list_sandbox_files", NewListSandboxFilesTool(nil), "29ac5d7626d1f0e25eb91385b556a01ff8d01305cb4ee2a7fa8e353235cb92cc"},
		{"write_sandbox_file", NewWriteSandboxFileTool(nil, 0), "443aa6f35b28d21ce7618869e9fbcd3bb712daed3dfd8c43555d5e932a38ae43"},
		{"edit_sandbox_file", NewEditSandboxFileTool(nil), "5d7f59f08c706b6d64724eb2ba74e28c099ab302593c2a147b1fc7eb736033e2"},
		{"shell_exec", NewShellExecTool(nil, nil), "70ff73d6f5bbb963f4973d7be1168f51b18278b7445401b5c12556d568fda9ed"},
		{"write_skill_file", NewWriteSkillFileTool(nil, ""), "5300f631df4a4510c478229983eb4dcb471dcd3a266064efd38a9c651aa6a169"},
		{"edit_skill_file", NewEditSkillFileTool(nil, ""), "82f09f98031ede0454b5543f2b1bc2a0f599c154c4f469efc107a7f85723251c"},
		{"craft_delegate", craftTool, "d6fc02b714d207cade89c9c3c0dcdc789c5911147cc01caddf9a5c48392299e8"},
	}
	for _, tc := range cases {
		raw := tc.tool.Parameters()
		sum := sha256.Sum256(raw)
		if got := hex.EncodeToString(sum[:]); got != tc.wantSHA {
			t.Errorf("%s Parameters() schema bytes changed: sha256=%s (len=%d), want %s", tc.name, got, len(raw), tc.wantSHA)
		}
	}
}
