// Pass B (23-knowledge-wikifaq) 过渡 shim — 删除点 ib2（Integration Brief 登记）。
//
// escapeLikePattern 原定义于 repository/wiki_page.go:1258，随 K3 迁移拆出：
// 该 helper 是 repository 包通用 SQL 转义工具，宿主他 owner 消费方
// （tenant_member.go，identity 属主）仍依赖包内同名符号，且 wiki 包
// （经 agentruntime 传递）会与任何 repository→wiki import 成环——故定义
// 单点保留在本包并导出供 wiki 包引用（wiki→repository 方向合法）。
// ib2 由集成工程师收口（评估迁往 wiki 或共享 util）。
package repository

import (
	"errors"
	"strings"
)

// EscapeLikePattern escapes LIKE / ILIKE metacharacters so the returned string
// can be safely concatenated with % wildcards without unintended matches.
// Order matters: escape the backslash first, then the wildcards.
func EscapeLikePattern(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)
	return replacer.Replace(s)
}

// escapeLikePattern 保持宿主包内他 owner 调用点（tenant_member.go）零改动。
var escapeLikePattern = EscapeLikePattern

// --- Wiki 哨兵（原 repository/wiki_page.go 定义，K3.1 物理留驻裁定）---
// agentruntime/agent/tools（wiki_route_resolver.go:136、wiki_tools.go:644）
// 经 repository.ErrWikiPageNotFound 做 errors.Is；wiki 包以别名引用同一实例。

// ErrWikiPageNotFound is returned when a wiki page is not found
var ErrWikiPageNotFound = errors.New("wiki page not found")

// ErrWikiPageConflict is returned when an optimistic lock conflict is detected
var ErrWikiPageConflict = errors.New("wiki page version conflict")

// ErrWikiFolderNotFound is returned when a wiki folder is not found.
var ErrWikiFolderNotFound = errors.New("wiki folder not found")

// ErrWikiFolderConflict is returned when a sibling folder with the same name
// already exists under the same parent.
var ErrWikiFolderConflict = errors.New("wiki folder name conflict")

// ErrWikiFolderNotEmpty is returned when a folder still has a live page or
// child folder at the instant an atomic delete is attempted.
var ErrWikiFolderNotEmpty = errors.New("wiki folder is not empty")
