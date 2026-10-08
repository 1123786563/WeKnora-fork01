package service
// FAQ 域锚定测试（原 internal/knowledge/faq 包；随上游对齐 round 2 归位 service 包）：锚定（Pass B 23-knowledge-wikifaq K3.2 Step 2，TDD RED→GREEN）。
//
// TestNewService* 锚定 faq.Service 构造器语义（依赖接线、sync.Map 指针共享、
// seam 透传）。acquireFAQCreateGuard 三用例的迁移前后双跑锚定由随迁的
// knowledge_faq_create_guard_test.go 承载（T0 宿主基线 vs Step 6 faq 终态，
// 计划 §8「FAQ 导入 TypeFAQImport」差分锚点族），本文件不重复声明同名用例。


import (
	"context"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestNewServiceWiresDependencies(t *testing.T) {
	rdb := &redis.Client{}
	sharedProgress := &sync.Map{}
	sharedRunning := &sync.Map{}
	seams := FAQSeams{KBActivityTrigger: func(context.Context) string { return "user" }}

	svc := NewService(Deps{
		RedisClient:         rdb,
		MemFAQProgress:      sharedProgress,
		MemFAQRunningImport: sharedRunning,
		Seams:                  seams,
	})

	if svc.redisClient != rdb {
		t.Fatal("redisClient not wired verbatim")
	}
	// sync.Map 以指针共享（计划 §6.2 D1(c)）：宿主 knowledgeService 的两个
	// 内存回落表跨 faqSvc() 重构造不丢状态。
	if svc.memFAQProgress != sharedProgress || svc.memFAQRunningImport != sharedRunning {
		t.Fatal("sync.Map pointers must be shared, not copied")
	}
	if svc.seams.KBActivityTrigger == nil {
		t.Fatal("seams not carried through constructor")
	}
}

func TestNewServiceDefaultsInMemoryTables(t *testing.T) {
	svc := NewService(Deps{})
	if svc.memFAQProgress == nil || svc.memFAQRunningImport == nil {
		t.Fatal("NewService must default nil in-memory tables to fresh maps")
	}
	svc.memFAQProgress.Store("task", "progress")
	if v, ok := svc.memFAQProgress.Load("task"); !ok || v != "progress" {
		t.Fatal("defaulted memFAQProgress must be usable")
	}
}
