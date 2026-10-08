package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TestCheckSufficientSummaryContent verifies the gate that prevents getSummary
// from calling the LLM (and ProcessSummaryGeneration from creating a summary
// chunk) when the document has no usable text. This is the entry point for
// the errInsufficientSummaryContent → SummaryStatusFailed flow that the
// caller in ProcessSummaryGeneration relies on.
func TestCheckSufficientSummaryContent(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{
			name:      "empty content rejected",
			content:   "",
			wantError: true,
		},
		{
			name:      "only whitespace rejected",
			content:   "   \n\n\t  ",
			wantError: true,
		},
		{
			name:      "below threshold rejected",
			content:   "hi",
			wantError: true,
		},
		{
			name: "scanned PDF with no OCR (image-only) rejected",
			content: "![MX5280_page_1.png](images/MX5280_page_1.png)\n" +
				"![MX5280_page_2.png](images/MX5280_page_2.png)",
			wantError: true,
		},
		{
			name:      "scanned PDF with empty <image> wrapper rejected",
			content:   `<image url="x"><image_original>![a](x)</image_original></image>`,
			wantError: true,
		},
		{
			name:      "short legitimate note above threshold accepted",
			content:   "Meeting at 3pm tomorrow.",
			wantError: false,
		},
		{
			name: "scanned PDF with successful VLM OCR accepted",
			content: `<image url="images/p1.png">
<image_original>![p1](images/p1.png)</image_original>
<image_caption>scanned letter</image_caption>
<image_ocr>Sehr geehrter Herr Mustermann, in der Sache 4711/2024 ...</image_ocr>
</image>`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSufficientSummaryContent(ctx, "test-knowledge-id", tt.content)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected errInsufficientSummaryContent, got nil")
					return
				}
				if !errors.Is(err, errInsufficientSummaryContent) {
					t.Errorf("expected errInsufficientSummaryContent sentinel, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
				}
			}
		})
	}
}

// TestCheckSufficientSummaryContent_ThresholdOverride verifies that
// MinTextContentRunes is a `var` (not const) so tests and future runtime
// configuration can adjust the threshold without a rebuild.
func TestCheckSufficientSummaryContent_ThresholdOverride(t *testing.T) {
	ctx := context.Background()
	content := "Meeting at 3pm." // 15 runes

	originalThreshold := MinTextContentRunes
	t.Cleanup(func() { MinTextContentRunes = originalThreshold })

	// With default threshold (10), this content passes.
	if err := checkSufficientSummaryContent(ctx, "kid", content); err != nil {
		t.Fatalf("default threshold: expected pass, got %v", err)
	}

	// With a tighter threshold (50), the same content is rejected.
	MinTextContentRunes = 50
	err := checkSufficientSummaryContent(ctx, "kid", content)
	if !errors.Is(err, errInsufficientSummaryContent) {
		t.Fatalf("tightened threshold: expected errInsufficientSummaryContent, got %v", err)
	}
}

func TestValidateSummaryOutput(t *testing.T) {
	tests := []struct {
		name     string
		response *types.ChatResponse
		want     string
		wantErr  error
	}{
		{name: "nil response rejected", response: nil, wantErr: errEmptySummaryOutput},
		{name: "empty response rejected", response: &types.ChatResponse{}, wantErr: errEmptySummaryOutput},
		{
			name:     "whitespace response rejected",
			response: &types.ChatResponse{Content: " \n\t "},
			wantErr:  errEmptySummaryOutput,
		},
		{
			name:     "budget-truncated reply rejected despite content",
			response: &types.ChatResponse{Content: `{"summary": "Half a profile that was cut at the to`, FinishReason: "length"},
			wantErr:  errSummaryOutputTruncated,
		},
		{
			name:     "valid response is trimmed",
			response: &types.ChatResponse{Content: "  useful summary \n", FinishReason: "stop"},
			want:     "useful summary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateSummaryOutput(tt.response)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("summary = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFirstTextChunkSummaryFallback(t *testing.T) {
	t.Run("uses only the first chunk", func(t *testing.T) {
		got := firstTextChunkSummaryFallback([]*types.Chunk{
			{Content: "  first chunk  "},
			{Content: "second chunk"},
		})
		if got != "first chunk" {
			t.Fatalf("fallback = %q, want first chunk", got)
		}
	})

	t.Run("does not skip an empty first chunk", func(t *testing.T) {
		got := firstTextChunkSummaryFallback([]*types.Chunk{
			{Content: " \n\t "},
			{Content: "second chunk"},
		})
		if got != "" {
			t.Fatalf("fallback = %q, want empty", got)
		}
	})

	t.Run("caps unicode content by runes", func(t *testing.T) {
		got := firstTextChunkSummaryFallback([]*types.Chunk{{
			Content: strings.Repeat("摘", summaryFallbackMaxRunes+25),
		}})
		if len([]rune(got)) != summaryFallbackMaxRunes {
			t.Fatalf("fallback rune count = %d, want %d", len([]rune(got)), summaryFallbackMaxRunes)
		}
	})

	t.Run("empty input stays empty", func(t *testing.T) {
		if got := firstTextChunkSummaryFallback(nil); got != "" {
			t.Fatalf("fallback = %q, want empty", got)
		}
	})
}

func TestApplyRetryableSummaryFailureState(t *testing.T) {
	chunks := []*types.Chunk{{Content: "first body chunk"}}

	t.Run("retry keeps existing description", func(t *testing.T) {
		knowledge := &types.Knowledge{
			Description:   "previous summary",
			SummaryStatus: types.SummaryStatusProcessing,
		}
		fallback := applyRetryableSummaryFailureState(knowledge, chunks, true)
		if fallback != "" {
			t.Fatalf("retry fallback = %q, want empty", fallback)
		}
		if knowledge.Description != "previous summary" {
			t.Fatalf("retry changed description to %q", knowledge.Description)
		}
		if knowledge.SummaryStatus != types.SummaryStatusPending {
			t.Fatalf("retry status = %q, want pending", knowledge.SummaryStatus)
		}
	})

	t.Run("terminal failure publishes fallback and fails summary", func(t *testing.T) {
		knowledge := &types.Knowledge{
			Description:   "previous summary",
			SummaryStatus: types.SummaryStatusProcessing,
		}
		fallback := applyRetryableSummaryFailureState(knowledge, chunks, false)
		if fallback != "first body chunk" {
			t.Fatalf("terminal fallback = %q", fallback)
		}
		if knowledge.Description != fallback {
			t.Fatalf("description = %q, want %q", knowledge.Description, fallback)
		}
		if knowledge.SummaryStatus != types.SummaryStatusFailed {
			t.Fatalf("terminal status = %q, want failed", knowledge.SummaryStatus)
		}
	})
}

func TestSummaryRetryStateSupportsLiteExecutorContext(t *testing.T) {
	retryCtx := types.WithTaskRetryMetadata(context.Background(), 1, 3)
	if !summaryTaskWillRetry(retryCtx) {
		t.Fatal("attempt 1 of maxRetry 3 should have another retry")
	}
	if isFinalAsynqAttempt(retryCtx) {
		t.Fatal("attempt 1 of maxRetry 3 should not be final")
	}

	finalCtx := types.WithTaskRetryMetadata(context.Background(), 3, 3)
	if summaryTaskWillRetry(finalCtx) {
		t.Fatal("attempt 3 of maxRetry 3 should not retry")
	}
	if !isFinalAsynqAttempt(finalCtx) {
		t.Fatal("attempt 3 of maxRetry 3 should be final")
	}
}

// stubSummaryChat stands in for the summary LLM with a canned response.
type stubSummaryChat struct {
	response *types.ChatResponse
}

func (m *stubSummaryChat) Chat(
	context.Context, []chat.Message, *chat.ChatOptions,
) (*types.ChatResponse, error) {
	return m.response, nil
}

func (m *stubSummaryChat) ChatStream(
	context.Context, []chat.Message, *chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	return nil, nil
}

func (m *stubSummaryChat) GetModelName() string { return "stub-summary" }
func (m *stubSummaryChat) GetModelID() string   { return "stub-summary" }

// defaultSummaryTestConfig mirrors the stock deployment: config.yaml points
// generate_summary_prompt_id at "default_summary", the default-marked entry
// of prompt_templates/generate_summary.yaml whose contract is strict JSON.
func defaultSummaryTestConfig() *config.Config {
	return &config.Config{
		Conversation: &config.ConversationConfig{
			GenerateSummaryPromptID: "default_summary",
			GenerateSummaryPrompt:   "Return strict JSON only.",
		},
		PromptTemplates: &config.PromptTemplatesConfig{
			GenerateSummary: []config.PromptTemplate{
				{ID: "default_summary", Name: "Document Profile", Default: true},
			},
		},
	}
}

func TestSummaryPromptRequiresJSON(t *testing.T) {
	customCfg := defaultSummaryTestConfig()
	customCfg.PromptTemplates.GenerateSummary = []config.PromptTemplate{
		{ID: "default_summary", Default: true},
		{ID: "my_plain_text_template"},
	}
	customCfg.Conversation.GenerateSummaryPromptID = "my_plain_text_template"

	unresolvedCfg := defaultSummaryTestConfig()
	unresolvedCfg.Conversation.GenerateSummaryPromptID = "deleted_template"

	tests := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{name: "nil config keeps the plain-text fallback", cfg: nil, want: false},
		{
			name: "missing template config keeps the plain-text fallback",
			cfg:  &config.Config{Conversation: &config.ConversationConfig{}},
			want: false,
		},
		{name: "unresolvable id keeps the plain-text fallback", cfg: unresolvedCfg, want: false},
		{name: "default-marked template requires JSON", cfg: defaultSummaryTestConfig(), want: true},
		{name: "custom template keeps the plain-text fallback", cfg: customCfg, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := summaryPromptRequiresJSON(tt.cfg); got != tt.want {
				t.Fatalf("summaryPromptRequiresJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSummaryDefaultTemplateContract(t *testing.T) {
	chunks := []*types.Chunk{{
		ID: "c-1", ChunkIndex: 0,
		Content: "This document covers the quarterly sales report and its main conclusions for the board.",
	}}
	plainTextReply := "The report covers\nquarterly sales results for the board."

	t.Run("bare plain text is rejected under the default template", func(t *testing.T) {
		svc := &knowledgeService{config: defaultSummaryTestConfig(), chunkRepo: summaryImageInfoChunkRepo{}}
		_, err := svc.getSummary(context.Background(), &stubSummaryChat{
			response: &types.ChatResponse{Content: plainTextReply, FinishReason: "stop"},
		}, &types.Knowledge{ID: "k-1"}, chunks)
		if !errors.Is(err, errInvalidSummaryOutput) {
			t.Fatalf("expected errInvalidSummaryOutput, got %v", err)
		}
	})

	t.Run("MaxTokens-truncated half JSON is rejected", func(t *testing.T) {
		svc := &knowledgeService{config: defaultSummaryTestConfig(), chunkRepo: summaryImageInfoChunkRepo{}}
		_, err := svc.getSummary(context.Background(), &stubSummaryChat{
			response: &types.ChatResponse{
				Content:      `{"summary": "Quarterly sales rose across all regions and the board appro`,
				FinishReason: "stop",
			},
		}, &types.Knowledge{ID: "k-1"}, chunks)
		if !errors.Is(err, errInvalidSummaryOutput) {
			t.Fatalf("expected errInvalidSummaryOutput, got %v", err)
		}
	})

	t.Run("budget-truncated reply is rejected before parsing", func(t *testing.T) {
		svc := &knowledgeService{config: defaultSummaryTestConfig(), chunkRepo: summaryImageInfoChunkRepo{}}
		_, err := svc.getSummary(context.Background(), &stubSummaryChat{
			response: &types.ChatResponse{
				Content:      `{"summary": "Looks complete but the tail never arrived"`,
				FinishReason: "length",
			},
		}, &types.Knowledge{ID: "k-1"}, chunks)
		if !errors.Is(err, errSummaryOutputTruncated) {
			t.Fatalf("expected errSummaryOutputTruncated, got %v", err)
		}
	})

	t.Run("custom template keeps the plain-text fallback", func(t *testing.T) {
		customCfg := defaultSummaryTestConfig()
		customCfg.PromptTemplates.GenerateSummary = []config.PromptTemplate{
			{ID: "default_summary", Default: true},
			{ID: "my_plain_text_template"},
		}
		customCfg.Conversation.GenerateSummaryPromptID = "my_plain_text_template"

		svc := &knowledgeService{config: customCfg, chunkRepo: summaryImageInfoChunkRepo{}}
		result, err := svc.getSummary(context.Background(), &stubSummaryChat{
			response: &types.ChatResponse{Content: plainTextReply, FinishReason: "stop"},
		}, &types.Knowledge{ID: "k-1"}, chunks)
		if err != nil {
			t.Fatalf("custom template must keep the plain-text fallback, got %v", err)
		}
		if result.Summary != "The report covers\nquarterly sales results for the board." {
			t.Fatalf("summary = %q, want the trimmed plain-text reply", result.Summary)
		}
		if result.Profile != nil {
			t.Fatalf("plain-text fallback must not produce a profile, got %+v", result.Profile)
		}
	})

	t.Run("valid JSON keeps its behavior under the default template", func(t *testing.T) {
		svc := &knowledgeService{config: defaultSummaryTestConfig(), chunkRepo: summaryImageInfoChunkRepo{}}
		result, err := svc.getSummary(context.Background(), &stubSummaryChat{
			response: &types.ChatResponse{
				Content: `{"summary":"Two sentences.","gist":"A gist","topics":["a"],` +
					`"doc_type":"report","typical_question":"Why?"}`,
				FinishReason: "stop",
			},
		}, &types.Knowledge{ID: "k-1"}, chunks)
		if err != nil {
			t.Fatalf("valid JSON must keep working, got %v", err)
		}
		if result.Summary != "Two sentences." {
			t.Fatalf("summary = %q, want %q", result.Summary, "Two sentences.")
		}
		if result.Profile == nil || result.Profile.Gist != "A gist" {
			t.Fatalf("profile = %+v, want gist %q", result.Profile, "A gist")
		}
	})
}

// summaryTestChunkRepo serves the chunk reads both the summary call (image
// info lookup) and the post-failure freshness check (GetChunkByID) perform.
type summaryTestChunkRepo struct {
	interfaces.ChunkRepository
	chunks []*types.Chunk
}

func (r *summaryTestChunkRepo) ListChunksByParentIDs(
	context.Context, uint64, []string,
) ([]*types.Chunk, error) {
	return nil, nil
}

func (r *summaryTestChunkRepo) GetChunkByID(_ context.Context, _ uint64, id string) (*types.Chunk, error) {
	for _, chunk := range r.chunks {
		if chunk.ID == id {
			return chunk, nil
		}
	}
	return nil, fmt.Errorf("chunk %s not found", id)
}

type summaryTextChunkService struct {
	interfaces.ChunkService
	chunks []*types.Chunk
}

func (s *summaryTextChunkService) ListChunksByKnowledgeID(
	context.Context, string,
) ([]*types.Chunk, error) {
	return s.chunks, nil
}

// TestProcessSummaryGenerationBadJSONRetriesWithoutPersistingRawReply pins
// the issue #3776 flow end to end: under the default template a truncated
// JSON reply must surface as a retryable error — the raw reply must never be
// persisted as the description, and the summary must never be Completed.
func TestProcessSummaryGenerationBadJSONRetriesWithoutPersistingRawReply(t *testing.T) {
	badReply := `{"summary": "Quarterly sales rose across all regions and the board appro`
	textChunks := []*types.Chunk{{
		ID: "c-1", ChunkType: types.ChunkTypeText, ChunkIndex: 0,
		Content: "This document covers the quarterly sales report and its main conclusions for the board.",
	}}
	repo := &summaryColumnsRepo{t: t, knowledge: summaryKnowledge()}
	svc := &knowledgeService{
		config:       defaultSummaryTestConfig(),
		repo:         repo,
		kbService:    &reparseFailureKBService{kb: summaryKB()},
		chunkService: &summaryTextChunkService{chunks: textChunks},
		chunkRepo:    &summaryTestChunkRepo{chunks: textChunks},
		modelService: &stubModelService{chatModel: &stubSummaryChat{
			response: &types.ChatResponse{Content: badReply, FinishReason: "stop"},
		}},
	}
	// Retry metadata makes summaryTaskWillRetry true, so the failure must be
	// recorded as pending for the next Asynq attempt instead of terminal.
	ctx := types.WithTaskRetryMetadata(context.Background(), 0, 3)

	err := svc.ProcessSummaryGeneration(ctx, summaryGenerationTask(t))

	if !errors.Is(err, errInvalidSummaryOutput) {
		t.Fatalf("expected errInvalidSummaryOutput, got %v", err)
	}
	for _, write := range repo.writes {
		if write["description"] == badReply {
			t.Fatal("the raw truncated JSON reply must not be persisted as the description")
		}
		if write["summary_status"] == types.SummaryStatusCompleted {
			t.Fatal("an invalid summary reply must never mark the summary Completed")
		}
	}
	last := repo.writes[len(repo.writes)-1]
	if last["summary_status"] != types.SummaryStatusPending {
		t.Fatalf("retryable failure status = %v, want %q",
			last["summary_status"], types.SummaryStatusPending)
	}
}
