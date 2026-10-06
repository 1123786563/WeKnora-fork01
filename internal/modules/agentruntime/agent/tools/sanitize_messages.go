package tools

import (
	"html"

	"github.com/Tencent/WeKnora/internal/models/chat"
)

// SanitizeMessages validates and fixes a message array for LLM compatibility.
// It handles common issues that cause provider API errors:
//   - Ensures no consecutive same-role messages (some providers reject these)
//   - Verifies tool result messages have matching tool_call in the preceding assistant message
//   - Removes empty content messages that can cause API errors
//
// A message counts as empty only when it carries nothing a provider accepts:
// no text, no images (legacy Images or MultiContent parts), and no tool calls.
// A pure-image user turn used to be dropped as empty, which is how the images
// of every parallel tool call after the first vanished (#3947).
//
// Returns the sanitized message slice (may be shorter than input). The input
// slice and every slice it references are left untouched; merged messages get
// fresh backing arrays.
func SanitizeMessages(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}

	result := make([]Message, 0, len(messages))
	for i, msg := range messages {
		// Skip empty non-system messages (some providers reject these)
		if msg.Role != "system" && msg.Role != "tool" && !messageHasContent(msg) {
			continue
		}

		// Prevent consecutive same-role messages (except tool results)
		if len(result) > 0 && msg.Role != "tool" {
			prev := result[len(result)-1]
			if prev.Role == msg.Role && prev.Role != "tool" {
				result[len(result)-1] = mergeAdjacentMessages(prev, msg)
				continue
			}
		}

		// Verify tool result messages reference a valid tool call
		if msg.Role == "tool" && msg.ToolCallID != "" {
			if !hasMatchingToolCall(messages[:i], msg.ToolCallID) {
				// Preserve recoverable data without promoting external output to policy.
				msg.Role = "user"
				msg.Content = "<untrusted_tool_result name=\"" + html.EscapeString(msg.Name) +
					"\">\n" + html.EscapeString(msg.Content) + "\n</untrusted_tool_result>"
				msg.ToolCallID = ""
				msg.Name = ""
			}
		}

		result = append(result, msg)
	}

	return result
}

// mergeAdjacentMessages folds msg into prev, an adjacent same-role message.
//
// The merge has to take the tool calls along: the tool results
// that follow still reference them, and a tool message whose
// tool_call_id has no matching assistant call is rejected by
// OpenAI-compatible endpoints. Dropping them here is what used
// to turn one merged pair into a 400, and the agent then fell
// back to a tool-less summary call where the model can only
// write the call it wanted to make as text.
//
// Both sides' content must survive too. The merge targets are exactly the
// multimodal turns — tool image evidence appended as user messages, and
// compaction summaries injected next to kept user turns — and the old
// Content-only join dropped every image and MultiContent part of the second
// message (#3947). When either side is multimodal, both sides normalize into
// one MultiContent slice; the legacy Content/Images fields are cleared rather
// than duplicated, because the request builders serialize MultiContent and
// ignore the legacy fields when both are set. Plain text pairs keep the
// legacy Content join.
func mergeAdjacentMessages(prev, msg Message) Message {
	merged := prev
	if !messageIsMultimodal(prev) && !messageIsMultimodal(msg) {
		merged.Content = joinContent(prev.Content, msg.Content)
	} else {
		parts := make([]chat.MessageContentPart, 0,
			len(prev.MultiContent)+len(prev.Images)+len(msg.MultiContent)+len(msg.Images)+2)
		parts = appendContentParts(parts, prev)
		parts = appendContentParts(parts, msg)
		merged.MultiContent = parts
		merged.Content = ""
		merged.Images = nil
	}
	if len(prev.ToolCalls) > 0 || len(msg.ToolCalls) > 0 {
		// Copy instead of appending in place: prev may share its
		// backing array with the caller's slice. Every slice of a merged
		// message is owned by the result, never aliased with the input.
		calls := make([]chat.ToolCall, 0, len(prev.ToolCalls)+len(msg.ToolCalls))
		calls = append(calls, prev.ToolCalls...)
		calls = append(calls, msg.ToolCalls...)
		merged.ToolCalls = calls
	}
	return merged
}

// joinContent joins two plain-text contents with a blank line, keeping the
// result empty when both sides are empty and dropping the separator when one
// side carries no text.
func joinContent(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "\n\n" + b
	}
}

// messageIsMultimodal reports whether the message uses a multimodal
// representation at all: legacy Images or MultiContent parts (even ones that
// carry no content — the merge must not silently mix a stale MultiContent
// with the legacy Content field).
func messageIsMultimodal(m Message) bool {
	return len(m.Images) > 0 || len(m.MultiContent) > 0
}

// appendContentParts appends m's provider-visible content onto parts, in the
// order the request builders already serialize it: MultiContent parts first
// (as given), then legacy images, then legacy text. Parts that carry nothing
// (empty text, image without URL) are dropped. Appended elements are value
// copies or freshly built, so the returned slice never aliases the input.
func appendContentParts(parts []chat.MessageContentPart, m Message) []chat.MessageContentPart {
	for _, p := range m.MultiContent {
		if partHasContent(p) {
			parts = append(parts, p)
		}
	}
	for _, img := range m.Images {
		// Legacy Images carry no detail setting; "auto" is what the
		// request builders send for them, so normalizing to it keeps the
		// wire body identical to the unmerged single message.
		parts = append(parts, chat.MessageContentPart{
			Type:     "image_url",
			ImageURL: &chat.ImageURL{URL: img, Detail: "auto"},
		})
	}
	if m.Content != "" {
		parts = append(parts, chat.MessageContentPart{Type: "text", Text: m.Content})
	}
	return parts
}

// messageHasContent reports whether the message carries anything a provider
// accepts: text, images (legacy or MultiContent), or tool calls.
func messageHasContent(m Message) bool {
	return m.Content != "" || len(m.Images) > 0 || len(m.ToolCalls) > 0 ||
		multiContentHasContent(m.MultiContent)
}

// multiContentHasContent reports whether any part carries text or an image URL.
func multiContentHasContent(parts []chat.MessageContentPart) bool {
	for _, p := range parts {
		if partHasContent(p) {
			return true
		}
	}
	return false
}

// partHasContent reports whether a MultiContent part serializes to anything:
// non-empty text, or an image_url with a URL.
func partHasContent(p chat.MessageContentPart) bool {
	return p.Text != "" || (p.ImageURL != nil && p.ImageURL.URL != "")
}

// hasMatchingToolCall checks if any preceding assistant message has a tool call with the given ID.
func hasMatchingToolCall(messages []Message, toolCallID string) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "assistant" {
			for _, tc := range msg.ToolCalls {
				if tc.ID == toolCallID {
					return true
				}
			}
		}
	}
	return false
}
