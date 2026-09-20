package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDecodePromptRejectsEmptyAndOversizedBodiesBeforeModelCall(t *testing.T) {
	for _, body := range []string{`{}`, `{"prompt":""}`, `{"prompt":"` + strings.Repeat("x", maxPrompt+1) + `"}`, `{"prompt":"ok","model":"other"}`, `{"prompt":"ok"} {"prompt":"second"}`} {
		req := httptest.NewRequest(http.MethodPost, "/v1/semantica-v02/generate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if _, err := decodePrompt(httptest.NewRecorder(), req); err == nil {
			t.Fatalf("decodePrompt accepted %d-byte invalid request", len(body))
		}
	}
}

func TestDecodePromptAcceptsOnlyTheFixedRequestShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/semantica-v02/generate", strings.NewReader(`{"prompt":"只回答图中的证据。"}`))
	req.Header.Set("Content-Type", "application/json")
	prompt, err := decodePrompt(httptest.NewRecorder(), req)
	if err != nil || prompt != "只回答图中的证据。" {
		t.Fatalf("decodePrompt() = %q, %v", prompt, err)
	}
}

func TestDecodePromptRejectsMissingOrWrongContentType(t *testing.T) {
	for _, contentType := range []string{"", "text/plain"} {
		req := httptest.NewRequest(http.MethodPost, "/v1/semantica-v02/generate", strings.NewReader(`{"prompt":"ok"}`))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if _, err := decodePrompt(httptest.NewRecorder(), req); err == nil {
			t.Fatalf("decodePrompt accepted Content-Type %q", contentType)
		}
	}
}

func TestAppendCappedNeverExceedsLimitAndReportsTruncation(t *testing.T) {
	var output strings.Builder
	written, truncated := appendCapped(&output, 0, strings.Repeat("界", maxOutput))
	if !truncated || written > maxOutput || len(output.String()) > maxOutput {
		t.Fatalf("oversized chunk was not safely truncated: written=%d bytes=%d truncated=%t", written, len(output.String()), truncated)
	}
	if !utf8.ValidString(output.String()) {
		t.Fatal("output cap split a UTF-8 rune")
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
