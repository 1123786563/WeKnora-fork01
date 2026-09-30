// Pass B（23-knowledge-wikifaq）临时测试装置垫片。
// Ruling 2026-09-24-TEST-SUPPORT-SHIM：孤儿测试装置（宿主留驻测试
// knowledge_move_wiki_test.go 依赖随 wiki_deleted_kb_guard_test.go 迁出的
// 未导出 stub wikiGuardTaskQueue）→ 授权宿主包唯一 _test.go 垫片
// （最小符号定义）。remove_at: ib2（先到先删，最迟 B5）。
// 台账「临时测试装置垫片（B5 清理范围）」追踪。
package service

import (
	"github.com/hibiken/asynq"
)

// wikiGuardTaskQueue 照录迁移前 wiki_deleted_kb_guard_test.go 中的同名
// stub（记录入队任务类型，供断言）。
type wikiGuardTaskQueue struct {
	tasks []*asynq.Task
}

func (q *wikiGuardTaskQueue) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	q.tasks = append(q.tasks, task)
	return &asynq.TaskInfo{ID: "guard-test", Type: task.Type()}, nil
}
