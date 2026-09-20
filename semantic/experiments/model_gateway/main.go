// Command model_gateway is a loopback-only V02 experiment entry, not A03.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	listenAddress = "127.0.0.1:18092"
	modelName     = "qwen2.5:0.5b"
	maxPrompt     = 8000
	maxOutput     = 1200
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
}

func decodePrompt(w http.ResponseWriter, r *http.Request) (string, error) {
	if r.Method != http.MethodPost {
		return "", fmt.Errorf("POST required")
	}
	defer r.Body.Close()
	var input request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPrompt+128)).Decode(&input); err != nil {
		return "", fmt.Errorf("fixed V02 prompt required: %w", err)
	}
	if input.Prompt == "" || len(input.Prompt) > maxPrompt {
		return "", fmt.Errorf("fixed V02 prompt required")
	}
	return input.Prompt, nil
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
	server := &http.Server{Addr: listenAddress, Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("V02 model gateway listening on %s", listenAddress)
	log.Fatal(server.Serve(listener))
}

func generate(w http.ResponseWriter, r *http.Request) {
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
	stream, err := client.ChatStream(ctx, []chat.Message{{Role: "user", Content: prompt}}, &chat.ChatOptions{Temperature: 0, MaxCompletionTokens: 96})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	var output strings.Builder
	var raw *types.TokenUsage
	for event := range stream {
		if event.ResponseType == types.ResponseTypeError {
			http.Error(w, event.Content, http.StatusBadGateway)
			return
		}
		if len(output.String()) < maxOutput {
			output.WriteString(event.Content)
		}
		if event.Usage != nil {
			raw = event.Usage
		}
	}
	if raw == nil || !validUsage(usage{PromptTokens: raw.PromptTokens, CompletionTokens: raw.CompletionTokens, TotalTokens: raw.TotalTokens}) {
		http.Error(w, "missing or inconsistent streaming raw usage", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response{Text: output.String(), RawUsage: usage{raw.PromptTokens, raw.CompletionTokens, raw.TotalTokens}, Model: modelName, Transport: "weknora-ollama-chatstream"})
}
