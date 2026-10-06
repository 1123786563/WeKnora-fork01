// Command model_gateway is a loopback-only V02 experiment entry, not A03.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/models/chat"
	ollamautils "github.com/Tencent/WeKnora/internal/models/utils/ollama"
	"github.com/Tencent/WeKnora/internal/types"
)

const (
	listenAddress    = "127.0.0.1:18092"
	modelName        = "qwen2.5:0.5b"
	maxPrompt        = 8000
	maxOutput        = 1200
	defaultMaxTokens = 96
	// V03 is an explicit extraction-only profile.  V02 keeps its 96-token,
	// 1200-byte safety envelope unchanged.
	extractionMaxOutput = 8192
	extractionMaxTokens = 512
)

type request struct {
	Prompt string `json:"prompt"`
}
type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
type response struct {
	Text      string `json:"text"`
	RawUsage  usage  `json:"raw_usage"`
	Model     string `json:"model"`
	Transport string `json:"transport"`
	Completed bool   `json:"completed"`
	Truncated bool   `json:"truncated"`
}

func decodePrompt(w http.ResponseWriter, r *http.Request) (string, error) {
	if r.Method != http.MethodPost {
		return "", fmt.Errorf("POST required")
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return "", fmt.Errorf("application/json Content-Type required")
	}
	defer r.Body.Close()
	var input request
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPrompt+128))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return "", fmt.Errorf("fixed V02 prompt required: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return "", fmt.Errorf("fixed V02 request must contain exactly one JSON object")
	}
	if input.Prompt == "" || len(input.Prompt) > maxPrompt {
		return "", fmt.Errorf("fixed V02 prompt required")
	}
	return input.Prompt, nil
}

func appendCapped(output *strings.Builder, written int, chunk string, limit int) (int, bool) {
	for _, runeValue := range chunk {
		runeBytes := len(string(runeValue))
		if written+runeBytes > limit {
			return written, true
		}
		output.WriteRune(runeValue)
		written += runeBytes
	}
	return written, false
}

func validUsage(value usage) bool {
	return value.PromptTokens >= 0 && value.CompletionTokens >= 0 && value.TotalTokens == value.PromptTokens+value.CompletionTokens
}

func main() {
	if raw := os.Getenv("OLLAMA_BASE_URL"); raw != "" && raw != "http://127.0.0.1:11434" {
		log.Fatal("OLLAMA_BASE_URL must be empty or http://127.0.0.1:11434 for V02")
	}
	if err := os.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:11434"); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/semantica-v02/generate", generate)
	mux.HandleFunc("/v1/semantica-v03/extract", generateExtraction)
	server := &http.Server{Addr: listenAddress, Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("V02 model gateway listening on %s", listenAddress)
	log.Fatal(server.Serve(listener))
}

func generate(w http.ResponseWriter, r *http.Request) {
	generateWithProfile(w, r, maxOutput, defaultMaxTokens, "V02")
}

func generateExtraction(w http.ResponseWriter, r *http.Request) {
	generateWithProfile(w, r, extractionMaxOutput, extractionMaxTokens, "V03")
}

func generateWithProfile(w http.ResponseWriter, r *http.Request, outputLimit int, tokenLimit int, profile string) {
	prompt, err := decodePrompt(w, r)
	if err != nil {
		http.Error(w, "fixed V02 prompt required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	service, err := ollamautils.GetOllamaService()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	client, err := chat.NewOllamaChat(&chat.ChatConfig{Source: types.ModelSourceLocal, ModelName: modelName, ModelID: "semantica-v02-local"}, service)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	stream, err := client.ChatStream(ctx, []chat.Message{{Role: "user", Content: prompt}}, &chat.ChatOptions{Temperature: 0, MaxCompletionTokens: tokenLimit})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	var output strings.Builder
	written := 0
	truncated := false
	var raw *types.TokenUsage
	for event := range stream {
		if event.ResponseType == types.ResponseTypeError {
			http.Error(w, event.Content, http.StatusBadGateway)
			return
		}
		if !truncated {
			written, truncated = appendCapped(&output, written, event.Content, outputLimit)
			if truncated {
				cancel()
			}
		}
		if event.Usage != nil {
			raw = event.Usage
		}
	}
	if truncated {
		http.Error(w, profile+" model output exceeded fixed cap; result is truncated", http.StatusRequestEntityTooLarge)
		return
	}
	if raw == nil || !validUsage(usage{PromptTokens: raw.PromptTokens, CompletionTokens: raw.CompletionTokens, TotalTokens: raw.TotalTokens}) {
		http.Error(w, "missing or inconsistent streaming raw usage", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response{Text: output.String(), RawUsage: usage{raw.PromptTokens, raw.CompletionTokens, raw.TotalTokens}, Model: modelName, Transport: "weknora-ollama-chatstream", Completed: true, Truncated: false})
}
