// Pass B（23-knowledge-wikifaq）临时测试装置垫片。
// Ruling 2026-09-24-TEST-SUPPORT-SHIM：孤儿测试装置（宿主留驻测试
// knowledge_write_access_test.go 等依赖随 K3.2 迁出的 FAQ 未导出方法）
// → 授权宿主包唯一 _test.go 垫片（最小符号定义 = 一行委托到 faq 包 R1
// 导出符号）。remove_at: ib2（先到先删，最迟 B5）。
// 台账「临时测试装置垫片（B5 清理范围）」追踪。
package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// UpdateFAQEntryStatus 委托 Service 同名方法（不在冻结接口面；宿主唯一
// 调用方是留驻白盒测试 knowledge_write_access_test.go:200）。
func (s *knowledgeService) UpdateFAQEntryStatus(ctx context.Context,
	kbID string, entryID string, isEnabled bool,
) error {
	return s.faqSvc().UpdateFAQEntryStatus(ctx, kbID, entryID, isEnabled)
}

// UpdateFAQEntryTag 委托 Service 同名方法（不在冻结接口面；宿主唯一
// 调用方是留驻白盒测试 knowledge_write_access_test.go:208）。
func (s *knowledgeService) UpdateFAQEntryTag(ctx context.Context, kbID string, entryID string, tagID *string) error {
	return s.faqSvc().UpdateFAQEntryTag(ctx, kbID, entryID, tagID)
}

// resolveTagID 委托 Service.ResolveTagID（原宿主未导出方法；宿主唯一
// 调用方是留驻白盒测试 knowledge_write_access_test.go:362）。
func (s *knowledgeService) resolveTagID(ctx context.Context, kbID string, payload *types.FAQEntryPayload) (string, error) {
	return s.faqSvc().ResolveTagID(ctx, kbID, payload)
}

// buildFAQTagResolver 委托 Service.BuildFAQTagResolver（原宿主未导出
// 方法；宿主唯一调用方是留驻白盒测试 knowledge_write_access_test.go:364）。
func (s *knowledgeService) buildFAQTagResolver(
	ctx context.Context, kbID string, entries []types.FAQEntryPayload,
) FAQTagResolver {
	return s.faqSvc().BuildFAQTagResolver(ctx, kbID, entries)
}

// faqImportCompletedOutcome 委托 FaqImportCompletedOutcome（原宿主未导出
// 纯函数；宿主唯一调用方是留驻测试 kb_activity_test.go:68 直测）。
func faqImportCompletedOutcome(successCount, failedCount, skippedCount int) types.AuditOutcome {
	return FaqImportCompletedOutcome(successCount, failedCount, skippedCount)
}

// faqImportActivityDetails 委托 FaqImportActivityDetails（原宿主未导出
// 纯函数；宿主唯一调用方是留驻测试 kb_activity_test.go:79 直测）。
func faqImportActivityDetails(payload *types.FAQImportPayload, progress *types.FAQImportProgress, totalEntries int) map[string]any {
	return FaqImportActivityDetails(payload, progress, totalEntries)
}
