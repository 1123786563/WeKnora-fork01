// Package router —— knowledge 模块 worker 双栈装配点（Pass B IB2，K5 Brief (b)）。
//
// 本文件是集成工程师独占的装配切换面：把 knowledge 模块 18 个任务处理器经
// 模块门面（internal/knowledge.Module.RegisterWorkers）登记进 Redis
// （asynq ServeMux）与 Lite（SyncTaskExecutor）双栈，替代 task.go / sync_task.go
// 中原 18+18 行手写注册。Redis 与 Lite 为互斥运行模式（container.go 按配置
// 二选一起动），每次进程内恰有一次 RegisterWorkers 调用，registry 的
// already-registered 幂等保护不受影响；双栈 registry 同窗构造以使
// bootstrap.VerifyWorkerParity 奇偶校验生效（不活动一侧 sink 为 nil，仅记录）。
// 非 knowledge 任务类型的注册行不受本装配影响。
package router

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/Tencent/WeKnora/internal/bootstrap"
	"github.com/Tencent/WeKnora/internal/knowledge"
)

// workerHandlerFunc 是 asynq 与 Lite 两栈统一的处理器签名。
type workerHandlerFunc = func(context.Context, *asynq.Task) error

// asynqMuxSink 把 WorkerRegistry 的登记委托给 asynq ServeMux（Redis 栈）。
type asynqMuxSink struct{ mux *asynq.ServeMux }

// RegisterTaskHandler 实现 bootstrap.WorkerSink。
func (s asynqMuxSink) RegisterTaskHandler(taskType string, handler any) {
	s.mux.HandleFunc(taskType, handler.(workerHandlerFunc))
}

// syncExecutorSink 把登记委托给 SyncTaskExecutor（Lite 栈）。
type syncExecutorSink struct{ exec *SyncTaskExecutor }

// RegisterTaskHandler 实现 bootstrap.WorkerSink。
func (s syncExecutorSink) RegisterTaskHandler(taskType string, handler any) {
	s.exec.RegisterHandler(taskType, handler.(workerHandlerFunc))
}

// RegisterKnowledgeWorkersRedis 在 Redis 模式装配点（RunAsynqServer 内）把
// knowledge 模块 18 个任务处理器登记进 mux；Lite registry 同窗构造（sink=nil）
// 参与 VerifyWorkerParity 奇偶校验，不落 Lite sink。
func RegisterKnowledgeWorkersRedis(mux *asynq.ServeMux, mod *knowledge.Module) error {
	if mod == nil {
		return nil
	}
	redis := bootstrap.NewWorkerRegistry("redis", asynqMuxSink{mux: mux})
	lite := bootstrap.NewWorkerRegistry("lite", nil)
	return mod.RegisterWorkers(redis, lite)
}

// RegisterKnowledgeWorkersLite 是 Lite 模式装配点（RegisterSyncHandlers 内，
// sink=SyncTaskExecutor）；Redis registry 同窗构造（sink=nil）仅参与奇偶校验。
func RegisterKnowledgeWorkersLite(exec *SyncTaskExecutor, mod *knowledge.Module) error {
	if mod == nil {
		return nil
	}
	redis := bootstrap.NewWorkerRegistry("redis", nil)
	lite := bootstrap.NewWorkerRegistry("lite", syncExecutorSink{exec: exec})
	return mod.RegisterWorkers(redis, lite)
}
