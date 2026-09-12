package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxBodyBytes   = 8 << 20
	maxEventBytes  = 1 << 20
	requestTimeout = 30 * time.Second
)

// Client implements the HTTP boundary exposed by the pinned OpenCode server.
type Client struct {
	base *url.URL
	http *http.Client
}

func NewClient(baseURL string, h *http.Client) (*Client, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse OpenCode base URL: %w", err)
	}
	if (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.Path != "" || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("OpenCode base URL must be an http(s) origin")
	}
	if h == nil {
		h = http.DefaultClient
	}
	copyClient := *h
	originalRedirect := copyClient.CheckRedirect
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !strings.EqualFold(req.URL.Host, base.Host) {
			return errors.New("OpenCode redirect crossed host boundary")
		}
		if originalRedirect != nil {
			return originalRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &Client{base: base, http: &copyClient}, nil
}

func (c *Client) CreateSession(ctx context.Context) (string, error) {
	var response struct {
		ID string `json:"id"`
	}
	if err := c.jsonRequest(ctx, http.MethodPost, "/session", nil, http.StatusOK, &response); err != nil {
		return "", err
	}
	if response.ID == "" {
		return "", errors.New("OpenCode create session response has empty id")
	}
	return response.ID, nil
}

func (c *Client) Prompt(ctx context.Context, sessionID, messageID, prompt string) error {
	if !validMessageID(messageID) {
		return errors.New("message id does not match the pinned OpenCode format")
	}
	payload := struct {
		MessageID string `json:"messageID"`
		Parts     []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"parts"`
	}{MessageID: messageID}
	payload.Parts = append(payload.Parts, struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}{Type: "text", Text: prompt})
	return c.jsonRequest(ctx, http.MethodPost, escapedPath("session", sessionID, "prompt_async"), payload, http.StatusNoContent, nil)
}

func (c *Client) Events(ctx context.Context) (io.ReadCloser, error) {
	request, err := c.newRequest(ctx, http.MethodGet, "/event", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "text/event-stream")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("OpenCode events request: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, responseError(response, http.StatusOK)
	}
	return &sseLimitReadCloser{ReadCloser: response.Body}, nil
}

func (c *Client) Messages(ctx context.Context, sessionID string) ([]Message, error) {
	var wire []struct {
		Info struct {
			ID       string `json:"id"`
			ParentID string `json:"parentID"`
			Role     string `json:"role"`
			Finish   string `json:"finish"`
			Time     struct {
				Completed int64 `json:"completed"`
			} `json:"time"`
		} `json:"info"`
		Parts json.RawMessage `json:"parts"`
	}
	if err := c.jsonRequest(ctx, http.MethodGet, escapedPath("session", sessionID, "message"), nil, http.StatusOK, &wire); err != nil {
		return nil, err
	}
	messages := make([]Message, 0, len(wire))
	for _, item := range wire {
		parts, err := DecodeParts(item.Parts)
		if err != nil {
			return nil, fmt.Errorf("decode message %q parts: %w", item.Info.ID, err)
		}
		messages = append(messages, Message{
			ID: item.Info.ID, ParentID: item.Info.ParentID, Role: item.Info.Role,
			Finish: item.Info.Finish, CompletedAt: item.Info.Time.Completed, Parts: parts,
		})
	}
	return messages, nil
}

func (c *Client) Abort(ctx context.Context, sessionID string) error {
	return c.boolRequest(ctx, http.MethodPost, escapedPath("session", sessionID, "abort"), nil)
}

func (c *Client) Status(ctx context.Context, sessionID string) (string, error) {
	var statuses map[string]struct {
		Type string `json:"type"`
	}
	if err := c.jsonRequest(ctx, http.MethodGet, "/session/status", nil, http.StatusOK, &statuses); err != nil {
		return "", err
	}
	status, ok := statuses[sessionID]
	if !ok {
		return "idle", nil
	}
	switch status.Type {
	case "idle", "busy", "retry":
		return status.Type, nil
	default:
		return "", fmt.Errorf("unknown OpenCode session status %q", status.Type)
	}
}

func (c *Client) ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error {
	return c.boolRequest(ctx, http.MethodPost, escapedPath("question", requestID, "reply"), struct {
		Answers [][]string `json:"answers"`
	}{Answers: answers})
}

func (c *Client) RejectQuestion(ctx context.Context, requestID string) error {
	return c.boolRequest(ctx, http.MethodPost, escapedPath("question", requestID, "reject"), nil)
}

func (c *Client) ReplyPermission(ctx context.Context, sessionID, requestID, reply string) error {
	if reply != "once" && reply != "reject" {
		return errors.New("permission reply must be once or reject")
	}
	return c.boolRequest(ctx, http.MethodPost, escapedPath("session", sessionID, "permissions", requestID), struct {
		Response string `json:"response"`
	}{Response: reply})
}

func (c *Client) boolRequest(ctx context.Context, method, path string, body any) error {
	var accepted bool
	if err := c.jsonRequest(ctx, method, path, body, http.StatusOK, &accepted); err != nil {
		return err
	}
	if !accepted {
		return errors.New("OpenCode rejected request")
	}
	return nil
}

func (c *Client) jsonRequest(ctx context.Context, method, path string, body any, wantStatus int, result any) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	var encoded io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode OpenCode request: %w", err)
		}
		encoded = bytes.NewReader(raw)
	}
	request, err := c.newRequest(ctx, method, path, encoded)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("OpenCode %s %s: %w", method, path, err)
	}
	if response.StatusCode != wantStatus {
		return responseError(response, wantStatus)
	}
	defer response.Body.Close()
	if result == nil {
		_, err = readBounded(response.Body)
		return err
	}
	raw, err := readBounded(response.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, result); err != nil {
		return fmt.Errorf("decode OpenCode response: %w", err)
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, method, c.base.Scheme+"://"+c.base.Host+path, body)
}

func responseError(response *http.Response, want int) error {
	defer response.Body.Close()
	raw, err := readBounded(response.Body)
	if err != nil {
		return fmt.Errorf("OpenCode response status %d, want %d: %w", response.StatusCode, want, err)
	}
	if len(raw) > 4096 {
		raw = raw[:4096]
	}
	return fmt.Errorf("OpenCode response status %d, want %d: %s", response.StatusCode, want, strings.TrimSpace(string(raw)))
}

func readBounded(reader io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxBodyBytes {
		return nil, errors.New("OpenCode response body exceeds 8 MiB")
	}
	return raw, nil
}

func escapedPath(segments ...string) string {
	var builder strings.Builder
	for _, segment := range segments {
		builder.WriteByte('/')
		builder.WriteString(url.PathEscape(segment))
	}
	return builder.String()
}

type sseLimitReadCloser struct {
	io.ReadCloser
	frameBytes int
	lastByte   byte
}

func (r *sseLimitReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	for i, b := range p[:n] {
		r.frameBytes++
		if r.frameBytes > maxEventBytes {
			return i, errors.New("OpenCode SSE event exceeds 1 MiB")
		}
		if r.lastByte == '\n' && b == '\n' {
			r.frameBytes = 0
		}
		r.lastByte = b
	}
	return n, err
}
