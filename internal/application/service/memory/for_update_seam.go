package memory

import "gorm.io/gorm/clause"

// forUpdateClause returns the gorm SELECT ... FOR UPDATE clause.
//
// 同形本地 seam（b3-r-memory / R1.2；先例 10-identity.md §3.3、22-knowledge-retrieval.md §5.2）：
// 原符号定义于宿主 internal/application/repository/tenant_member.go:25（identity 属主、
// B1-ID 推迟件），memory_extraction.go 迁出宿主后不可再裸名调用。冻结口径
// 10-identity.md:458：tenant_member.go 迁移时导出该 helper 并给本包留 shim；
// ib3 按 conventions §7.1 收口为单一实现后删除本文件改消费 identity 导出端口。
func forUpdateClause() clause.Expression {
	return clause.Locking{Strength: "UPDATE"}
}
