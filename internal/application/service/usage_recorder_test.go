package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/commercial/usage"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubUsageRepo captures AddUsage calls; the aggregation readers are unused
// by the recorder and stay nil-panicky on unexpected use.
type stubUsageRepo struct {
	added []*types.UserUsage
	err   error
}

func (s *stubUsageRepo) AddUsage(_ context.Context, u *types.UserUsage) error {
	if s.err != nil {
		return s.err
	}
	s.added = append(s.added, u)
	return nil
}

func (s *stubUsageRepo) AggregateByUser(context.Context, uint64, string, time.Time, time.Time) ([]interfaces.UsageAggregate, error) {
	panic("unexpected call")
}

func (s *stubUsageRepo) AggregateAllUsers(context.Context, uint64, time.Time, time.Time, int, int) ([]interfaces.UsageAggregate, error) {
	panic("unexpected call")
}

func (s *stubUsageRepo) ExportRows(context.Context, uint64, time.Time, time.Time) ([]interfaces.UsageAggregate, error) {
	panic("unexpected call")
}

// newRecorderForTest builds a recorder with a frozen clock so WindowStart is
// deterministic.
func newRecorderForTest(repo *stubUsageRepo, rates usage.ModelRates, frozen time.Time) *UsageRecorder {
	s := NewUsageRecorderService(repo, rates)
	s.now = func() time.Time { return frozen }
	return s
}

func TestRecordChatTurnMapsDimensions(t *testing.T) {
	repo := &stubUsageRepo{}
	frozen := time.Date(2026, 9, 19, 14, 23, 51, 988, time.FixedZone("CST", 8*3600))
	rates := usage.ModelRates{"gpt-x": {RateMicro: 1500, Units: 1000}}
	s := newRecorderForTest(repo, rates, frozen)

	u := &types.TokenUsage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
		CacheReadTokens:  300,
		CacheWriteTokens: 120,
	}
	if err := s.RecordChatTurn(context.Background(), 7, "user-1", "gpt-x", u); err != nil {
		t.Fatalf("RecordChatTurn: %v", err)
	}
	if len(repo.added) != 1 {
		t.Fatalf("want 1 AddUsage call, got %d", len(repo.added))
	}
	got := repo.added[0]
	want := &types.UserUsage{
		TenantID:         7,
		UserID:           "user-1",
		WindowStart:      time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
		Model:            "gpt-x",
		Flow:             types.UsageFlowChat,
		InputTokens:      1000,
		OutputTokens:     500,
		CacheReadTokens:  300,
		CacheWriteTokens: 120,
		CostMicrocredits: 2250, // 1500 * (1000+500) / 1000
	}
	if *got != *want {
		t.Errorf("bucket mismatch:\n got  %+v\n want %+v", *got, *want)
	}
}

func TestRecordChatTurnCacheFallback(t *testing.T) {
	cases := []struct {
		name      string
		usage     types.TokenUsage
		wantRead  int64
		wantWrite int64
	}{
		{
			name:      "legacy cached_tokens used when cache_read is zero",
			usage:     types.TokenUsage{PromptTokens: 10, CachedTokens: 80},
			wantRead:  80,
			wantWrite: 0,
		},
		{
			name:      "explicit cache_read wins over legacy alias",
			usage:     types.TokenUsage{PromptTokens: 10, CachedTokens: 999, CacheReadTokens: 40},
			wantRead:  40,
			wantWrite: 0,
		},
		{
			name:      "both zero stays zero",
			usage:     types.TokenUsage{PromptTokens: 10},
			wantRead:  0,
			wantWrite: 0,
		},
		{
			name:      "write counter passes through",
			usage:     types.TokenUsage{PromptTokens: 10, CacheWriteTokens: 55, CachedTokens: 5},
			wantRead:  5,
			wantWrite: 55,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubUsageRepo{}
			s := newRecorderForTest(repo, nil, time.Date(2026, 9, 19, 1, 0, 0, 0, time.UTC))
			if err := s.RecordChatTurn(context.Background(), 1, "u", "m", &tc.usage); err != nil {
				t.Fatalf("RecordChatTurn: %v", err)
			}
			if len(repo.added) != 1 {
				t.Fatalf("want 1 AddUsage call, got %d", len(repo.added))
			}
			if got := repo.added[0].CacheReadTokens; got != tc.wantRead {
				t.Errorf("CacheReadTokens = %d, want %d", got, tc.wantRead)
			}
			if got := repo.added[0].CacheWriteTokens; got != tc.wantWrite {
				t.Errorf("CacheWriteTokens = %d, want %d", got, tc.wantWrite)
			}
		})
	}
}

func TestRecordChatTurnSkipsInvalidInput(t *testing.T) {
	cases := []struct {
		name   string
		tenant uint64
		userID string
		model  string
		usage  *types.TokenUsage
	}{
		{name: "nil usage", tenant: 1, userID: "u", model: "m", usage: nil},
		{name: "empty user", tenant: 1, userID: "", model: "m", usage: &types.TokenUsage{PromptTokens: 5}},
		{name: "empty model", tenant: 1, userID: "u", model: "", usage: &types.TokenUsage{PromptTokens: 5}},
		{name: "zero tenant", tenant: 0, userID: "u", model: "m", usage: &types.TokenUsage{PromptTokens: 5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubUsageRepo{}
			s := newRecorderForTest(repo, nil, time.Now())
			if err := s.RecordChatTurn(context.Background(), tc.tenant, tc.userID, tc.model, tc.usage); err != nil {
				t.Errorf("RecordChatTurn error = %v, want nil", err)
			}
			if len(repo.added) != 0 {
				t.Errorf("AddUsage called %d times, want 0", len(repo.added))
			}
		})
	}
}

func TestRecordChatTurnCostFromInjectedRates(t *testing.T) {
	cases := []struct {
		name  string
		rates usage.ModelRates
		model string
		in    int
		out   int
		want  int64
	}{
		{
			// 1000 * 1 / 3 = 333.33 -> 333 (half away from zero down)
			name: "rounds down below half", rates: usage.ModelRates{"m": {RateMicro: 1000, Units: 3}}, model: "m", in: 1, out: 0, want: 333,
		},
		{
			// 1000 * 2 / 3 = 666.67 -> 667 (half away from zero up)
			name: "rounds up above half", rates: usage.ModelRates{"m": {RateMicro: 1000, Units: 3}}, model: "m", in: 2, out: 0, want: 667,
		},
		{
			name: "unrated model costs zero", rates: usage.ModelRates{"other": {RateMicro: 1, Units: 1}}, model: "m", in: 100, out: 100, want: 0,
		},
		{
			name: "nil rates cost zero", rates: nil, model: "m", in: 100, out: 100, want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubUsageRepo{}
			s := newRecorderForTest(repo, tc.rates, time.Now())
			u := &types.TokenUsage{PromptTokens: tc.in, CompletionTokens: tc.out}
			if err := s.RecordChatTurn(context.Background(), 1, "u", tc.model, u); err != nil {
				t.Fatalf("RecordChatTurn: %v", err)
			}
			if len(repo.added) != 1 {
				t.Fatalf("want 1 AddUsage call, got %d", len(repo.added))
			}
			if got := repo.added[0].CostMicrocredits; got != tc.want {
				t.Errorf("CostMicrocredits = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestRecordChatTurnPropagatesRepoError(t *testing.T) {
	boom := errors.New("db down")
	repo := &stubUsageRepo{err: boom}
	s := newRecorderForTest(repo, nil, time.Now())
	err := s.RecordChatTurn(context.Background(), 1, "u", "m", &types.TokenUsage{PromptTokens: 1})
	if !errors.Is(err, boom) {
		t.Errorf("RecordChatTurn error = %v, want %v", err, boom)
	}
}

func TestRecordChatTurnNilReceiverSafe(t *testing.T) {
	var s *UsageRecorder
	if err := s.RecordChatTurn(context.Background(), 1, "u", "m", &types.TokenUsage{PromptTokens: 1}); err != nil {
		t.Errorf("nil receiver RecordChatTurn error = %v, want nil", err)
	}
}
