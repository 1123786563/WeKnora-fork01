package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// 测试本地哨兵孪生值（plan §6.3 末行）：原测试中 4 处
// repository.ErrKnowledgeNotFound / ErrKnowledgeBaseNotFound 改以 errors.New
// 孪生值注入同名字段——比对语义不变（字段即 errors.Is 的比对目标）。
var (
	testKnowledgeNotFound     = errors.New("knowledge not found")
	testKnowledgeBaseNotFound = errors.New("knowledge base not found")
)

// testPostProcessTaskOptions 是 K1.4 增量 seam（postProcessTaskOptionsFn）的
// 测试桩：本文件用例仅断言入队计数与任务类型，不涉及 options 内容；生产路径
// 注入真源 knowledgePostProcessTaskOptions（零复制），生产代码对 nil 字段保持
// 快速失败、不设静默回退。
func testPostProcessTaskOptions() []asynq.Option { return nil }

type orphanKnowledgeRepo struct {
	interfaces.KnowledgeRepository
	knowledge *types.Knowledge
	err       error
}

func (r *orphanKnowledgeRepo) GetKnowledgeByIDOnly(_ context.Context, _ string) (*types.Knowledge, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.knowledge, nil
}

type orphanKBService struct {
	interfaces.KnowledgeBaseService
	kb  *types.KnowledgeBase
	err error
}

func (s *orphanKBService) GetKnowledgeBaseByIDOnly(_ context.Context, _ string) (*types.KnowledgeBase, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.kb, nil
}

func TestShouldDropOrphanedMultimodal(t *testing.T) {
	t.Parallel()
	svc := &ImageMultimodalService{
		knowledgeNotFoundErr:     testKnowledgeNotFound,
		knowledgeBaseNotFoundErr: testKnowledgeBaseNotFound,
	}

	drop, err := svc.shouldDropOrphanedMultimodal(context.Background(), &types.ImageMultimodalPayload{
		KnowledgeID: "missing",
	})
	if err != nil || drop {
		t.Fatalf("nil repo should not drop: drop=%v err=%v", drop, err)
	}

	svc.knowledgeRepo = &orphanKnowledgeRepo{err: testKnowledgeNotFound}
	drop, err = svc.shouldDropOrphanedMultimodal(context.Background(), &types.ImageMultimodalPayload{
		KnowledgeID: "missing",
	})
	if err != nil || !drop {
		t.Fatalf("missing knowledge should drop: drop=%v err=%v", drop, err)
	}

	svc.knowledgeRepo = &orphanKnowledgeRepo{knowledge: &types.Knowledge{ParseStatus: types.ParseStatusCancelled}}
	drop, err = svc.shouldDropOrphanedMultimodal(context.Background(), &types.ImageMultimodalPayload{
		KnowledgeID: "cancelled",
	})
	if err != nil || !drop {
		t.Fatalf("cancelled knowledge should drop: drop=%v err=%v", drop, err)
	}

	svc.knowledgeRepo = &orphanKnowledgeRepo{knowledge: &types.Knowledge{ParseStatus: types.ParseStatusProcessing}}
	svc.kbService = &orphanKBService{err: testKnowledgeBaseNotFound}
	drop, err = svc.shouldDropOrphanedMultimodal(context.Background(), &types.ImageMultimodalPayload{
		KnowledgeID:     "live",
		KnowledgeBaseID: "missing-kb",
	})
	if err != nil || !drop {
		t.Fatalf("missing kb should drop: drop=%v err=%v", drop, err)
	}
}

func TestImageMultimodalHandleDropsMissingKnowledge(t *testing.T) {
	t.Parallel()
	svc := &ImageMultimodalService{
		knowledgeNotFoundErr:     testKnowledgeNotFound,
		knowledgeBaseNotFoundErr: testKnowledgeBaseNotFound,
		knowledgeRepo:            &orphanKnowledgeRepo{err: testKnowledgeNotFound},
		kbService:                &orphanKBService{kb: &types.KnowledgeBase{ID: "kb-1"}},
	}
	payload, err := json.Marshal(types.ImageMultimodalPayload{
		TenantID:        1,
		KnowledgeID:     "missing",
		KnowledgeBaseID: "kb-1",
		ImageURL:        "minio://bucket/img.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(context.Background(), asynq.NewTask(types.TypeImageMultimodal, payload)); err != nil {
		t.Fatalf("orphan task should succeed without retry: %v", err)
	}
}

type orphanTaskEnqueuer struct {
	interfaces.TaskEnqueuer
	enqueued []*asynq.Task
}

func (e *orphanTaskEnqueuer) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	e.enqueued = append(e.enqueued, task)
	return &asynq.TaskInfo{ID: "post-process"}, nil
}

func TestImageMultimodalHandleDropFinalizesPendingCounter(t *testing.T) {
	t.Parallel()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	enqueuer := &orphanTaskEnqueuer{}
	svc := &ImageMultimodalService{
		knowledgeNotFoundErr:     testKnowledgeNotFound,
		knowledgeBaseNotFoundErr: testKnowledgeBaseNotFound,
		knowledgeRepo:            &orphanKnowledgeRepo{err: testKnowledgeNotFound},
		kbService:                &orphanKBService{kb: &types.KnowledgeBase{ID: "kb-1"}},
		redisClient:              rdb,
		taskEnqueuer:             enqueuer,
		postProcessTaskOptionsFn: testPostProcessTaskOptions,
	}
	const knowledgeID = "missing"
	redisKey := "multimodal:pending:" + knowledgeID
	if err := rdb.Set(context.Background(), redisKey, 1, 0).Err(); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(types.ImageMultimodalPayload{
		TenantID:        1,
		KnowledgeID:     knowledgeID,
		KnowledgeBaseID: "kb-1",
		ImageURL:        "minio://bucket/img.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(context.Background(), asynq.NewTask(types.TypeImageMultimodal, payload)); err != nil {
		t.Fatalf("orphan drop should succeed: %v", err)
	}
	if mr.Exists(redisKey) {
		t.Fatal("pending counter should be cleared after drop finalize")
	}
	if len(enqueuer.enqueued) != 1 || enqueuer.enqueued[0].Type() != types.TypeKnowledgePostProcess {
		t.Fatalf("expected post-process enqueue, got %d tasks", len(enqueuer.enqueued))
	}
}

func TestImageMultimodalHandlePropagatesTransientKnowledgeError(t *testing.T) {
	t.Parallel()
	dbErr := errors.New("db unavailable")
	svc := &ImageMultimodalService{
		knowledgeRepo: &orphanKnowledgeRepo{err: dbErr},
		kbService:     &orphanKBService{kb: &types.KnowledgeBase{ID: "kb-1"}},
	}
	payload, err := json.Marshal(types.ImageMultimodalPayload{
		TenantID:        1,
		KnowledgeID:     "k-1",
		KnowledgeBaseID: "kb-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(context.Background(), asynq.NewTask(types.TypeImageMultimodal, payload)); !errors.Is(err, dbErr) {
		t.Fatalf("transient error should propagate: %v", err)
	}
}
