package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodePromptRejectsEmptyAndOversizedBodiesBeforeModelCall(t *testing.T) {
	for _, body := range []string{`{}`, `{"prompt":""}`, `{"prompt":"` + strings.Repeat("x", maxPrompt+1) + `"}`} {
		req := httptest.NewRequest(http.MethodPost, "/v1/semantica-v02/generate", strings.NewReader(body))
		if _, err := decodePrompt(httptest.NewRecorder(), req); err == nil {
			t.Fatalf("decodePrompt accepted %d-byte invalid request", len(body))
		}
	}
}

func TestDecodePromptAcceptsOnlyTheFixedRequestShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/semantica-v02/generate", strings.NewReader(`{"prompt":"只回答图中的证据。"}`))
	prompt, err := decodePrompt(httptest.NewRecorder(), req)
	if err != nil || prompt != "只回答图中的证据。" {
		t.Fatalf("decodePrompt() = %q, %v", prompt, err)
	}
}

func TestRawUsageRequiresNonNegativeExactTotal(t *testing.T) {
	if !validUsage(usage{PromptTokens: 3, CompletionTokens: 5, TotalTokens: 8}) {
		t.Fatal("valid streaming counters rejected")
	}
	for _, candidate := range []usage{
		{PromptTokens: -1, CompletionTokens: 5, TotalTokens: 4},
		{PromptTokens: 3, CompletionTokens: -1, TotalTokens: 2},
		{PromptTokens: 3, CompletionTokens: 5, TotalTokens: 7},
	} {
		if validUsage(candidate) {
			t.Fatalf("invalid usage accepted: %+v", candidate)
		}
	}
}
