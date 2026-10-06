package ingest

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// extractSeamArgs 承载 NewChunkExtractService 的六项 seam 闭包参数，
// 供构造期 nil fail-fast 守卫测试逐项置 nil（OCR f3）。
type extractSeamArgs struct {
	spanTrace              SpanTraceSeam // 唯一允许 nil 的 seam（trace() noop 回退，plan §6.3）
	attemptSupersededFn    func(context.Context, string, int) bool
	previewTextFn          func(s string, maxRunes int) string
	finalizeSubtaskFn      func(ctx context.Context, repo interfaces.KnowledgeRepository, knowledgeID, source string, retErr error, superseded, final bool)
	isFinalAttemptFn       func(ctx context.Context) bool
	resolveProcessConfigFn func(kb *types.KnowledgeBase, overrides *types.KnowledgeProcessOverrides) types.EffectiveProcessConfig
	newGraphExtractor      GraphExtractorFactory
}

func nonNilExtractSeamArgs() extractSeamArgs {
	return extractSeamArgs{
		attemptSupersededFn: func(context.Context, string, int) bool { return false },
		previewTextFn:       func(s string, maxRunes int) string { return s },
		finalizeSubtaskFn: func(context.Context, interfaces.KnowledgeRepository, string, string, error, bool, bool) {
		},
		isFinalAttemptFn: func(context.Context) bool { return false },
		resolveProcessConfigFn: func(*types.KnowledgeBase, *types.KnowledgeProcessOverrides) types.EffectiveProcessConfig {
			return types.EffectiveProcessConfig{}
		},
		newGraphExtractor: func(chatModel chat.Chat, template *types.PromptTemplateStructured) GraphExtractorSeam {
			return nil
		},
	}
}

// TestNewChunkExtractServiceNilSeamFailFast 锚定 OCR f3 验收：六项 seam 闭包
// 漏注时构造期即 panic（消息逐一点名、格式对齐宿主先例
// service/session.go:176「NewXxx: yyy is required」），而非 Handle/defer 期
// nil 调用导致 pending_subtasks_count 永不递减。
func TestNewChunkExtractServiceNilSeamFailFast(t *testing.T) {
	cases := []struct {
		name    string
		nilSeam func(*extractSeamArgs)
		wantMsg string
	}{
		{
			name:    "attemptSupersededFn",
			nilSeam: func(a *extractSeamArgs) { a.attemptSupersededFn = nil },
			wantMsg: "NewChunkExtractService: attemptSupersededFn is required",
		},
		{
			name:    "previewTextFn",
			nilSeam: func(a *extractSeamArgs) { a.previewTextFn = nil },
			wantMsg: "NewChunkExtractService: previewTextFn is required",
		},
		{
			name:    "finalizeSubtaskFn",
			nilSeam: func(a *extractSeamArgs) { a.finalizeSubtaskFn = nil },
			wantMsg: "NewChunkExtractService: finalizeSubtaskFn is required",
		},
		{
			name:    "isFinalAttemptFn",
			nilSeam: func(a *extractSeamArgs) { a.isFinalAttemptFn = nil },
			wantMsg: "NewChunkExtractService: isFinalAttemptFn is required",
		},
		{
			name:    "resolveProcessConfigFn",
			nilSeam: func(a *extractSeamArgs) { a.resolveProcessConfigFn = nil },
			wantMsg: "NewChunkExtractService: resolveProcessConfigFn is required",
		},
		{
			name:    "newGraphExtractor",
			nilSeam: func(a *extractSeamArgs) { a.newGraphExtractor = nil },
			wantMsg: "NewChunkExtractService: newGraphExtractor is required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := nonNilExtractSeamArgs()
			tc.nilSeam(&args)
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("%s = nil: NewChunkExtractService did not panic (want fail-fast)", tc.name)
				}
				got := fmt.Sprint(r)
				if !strings.Contains(got, tc.wantMsg) {
					t.Errorf("panic message = %q, want prefix %q", got, tc.wantMsg)
				}
			}()
			NewChunkExtractService(
				&config.Config{ExtractManager: &config.ExtractManagerConfig{}},
				nil, // modelService：守卫先行，构造期不触达
				nil, // knowledgeBaseRepo
				nil, // knowledgeRepo
				nil, // chunkRepo
				nil, // graphEngine
				args.spanTrace,
				args.attemptSupersededFn,
				args.previewTextFn,
				args.finalizeSubtaskFn,
				args.isFinalAttemptFn,
				args.resolveProcessConfigFn,
				args.newGraphExtractor,
			)
		})
	}
}

// TestNewChunkExtractServiceNilSpanTraceAllowed 锚定 spanTrace 的 nil 语义：
// 唯一允许 nil 的 seam（trace() 回退 noopSpanTraceSeam，plan §6.3 原案），
// 构造期不得 panic。
func TestNewChunkExtractServiceNilSpanTraceAllowed(t *testing.T) {
	args := nonNilExtractSeamArgs()
	svc := NewChunkExtractService(
		&config.Config{ExtractManager: &config.ExtractManagerConfig{}},
		nil, nil, nil, nil, nil,
		nil, // spanTrace：允许 nil
		args.attemptSupersededFn,
		args.previewTextFn,
		args.finalizeSubtaskFn,
		args.isFinalAttemptFn,
		args.resolveProcessConfigFn,
		args.newGraphExtractor,
	)
	if svc == nil {
		t.Fatal("NewChunkExtractService(spanTrace=nil) = nil handler, want non-nil")
	}
}
