package repository

import "strings"

// Pass B 过渡 seam（R2）：escapeLikeKeyword 定义于 repository/knowledge.go:22
// （24-knowledge-process 属主，docs/architecture/passb/b0-evidence.md:86 B0.2 消歧，
// 无 K brief 枚举、显式归 K4）。K4 搬出导出窄端口后由 ib2 收口
// （docs/plans/passb/10-identity.md §3.3 先例）。本文件实现与
// internal/application/repository/knowledge.go:24-28 逐字对齐；tag.go 调用点零改动。

// likeEscapeChar is the SQL ESCAPE character paired with escapeLikeKeyword.
const likeEscapeChar = `\`

// escapeLikeKeyword escapes SQL LIKE wildcards (%, _) in a keyword
// so they are treated as literal characters.
func escapeLikeKeyword(keyword string) string {
	keyword = strings.ReplaceAll(keyword, `\`, `\\`)
	keyword = strings.ReplaceAll(keyword, "%", `\%`)
	keyword = strings.ReplaceAll(keyword, "_", `\_`)
	return keyword
}
