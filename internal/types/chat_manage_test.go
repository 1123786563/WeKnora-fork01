package types

import "testing"

// TestQueryIntentNeedsKBRetrieval locks the full intent → retrieval table so a
// future intent edit cannot silently flip retrieval for an unrelated intent.
//
// summarize is deliberately false (#3918): the intent summarizes the
// conversation itself, so KB retrieval would only leak unrelated chunks into
// the summary and waste embedding/retrieve/rerank calls. Requests that are
// about knowledge-base documents classify as kb_search instead.
func TestQueryIntentNeedsKBRetrieval(t *testing.T) {
	cases := []struct {
		name   string
		intent QueryIntent
		want   bool
	}{
		{"kb_search retrieves", IntentKBSearch, true},
		{"clarification retrieves", IntentClarification, true},
		{"empty intent retrieves for safety", "", true},

		{"summarize does not retrieve", IntentSummarize, false},
		{"greeting does not retrieve", IntentGreeting, false},
		{"chitchat does not retrieve", IntentChitchat, false},
		{"follow_up does not retrieve", IntentFollowUp, false},
		{"image_only does not retrieve", IntentImageOnly, false},
		{"doc_only does not retrieve", IntentDocOnly, false},

		// IntentWebSearch is excluded here on purpose: whether it retrieves
		// depends on WebSearchEnabled, decided by ChatManage.NeedsRetrieval.
		{"web_search is not a KB intent", IntentWebSearch, false},
		// Only the normalized empty value is the retrieve-anyway safety valve;
		// an arbitrary non-empty label reads as "no KB retrieval" (the
		// pipeline never sees such a value — NormalizeQueryIntent maps
		// unknown labels to "").
		{"arbitrary label is not KB retrieval", QueryIntent("summarize_all"), false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if got := c.intent.NeedsKBRetrieval(); got != c.want {
				t.Fatalf("NeedsKBRetrieval(%q) = %v, want %v", c.intent, got, c.want)
			}
		})
	}
}

// TestChatManageNeedsRetrieval pins the aggregation the pipeline plugins
// consume: every non-web intent delegates to NeedsKBRetrieval, and
// IntentWebSearch retrieves only while web search is enabled.
func TestChatManageNeedsRetrieval(t *testing.T) {
	cases := []struct {
		name             string
		intent           QueryIntent
		webSearchEnabled bool
		want             bool
	}{
		{"kb_search retrieves", IntentKBSearch, false, true},
		{"clarification retrieves", IntentClarification, false, true},
		{"unknown intent retrieves for safety", "", false, true},

		{"summarize skips retrieval (#3918)", IntentSummarize, false, false},
		{"greeting skips retrieval", IntentGreeting, false, false},
		{"chitchat skips retrieval", IntentChitchat, false, false},
		{"follow_up skips retrieval", IntentFollowUp, false, false},
		{"image_only skips retrieval", IntentImageOnly, false, false},
		{"doc_only skips retrieval", IntentDocOnly, false, false},

		{"web_search retrieves when enabled", IntentWebSearch, true, true},
		{"web_search skipped when disabled", IntentWebSearch, false, false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			cm := &ChatManage{
				PipelineRequest: PipelineRequest{WebSearchEnabled: c.webSearchEnabled},
				PipelineState:   PipelineState{Intent: c.intent},
			}
			if got := cm.NeedsRetrieval(); got != c.want {
				t.Fatalf("NeedsRetrieval(intent=%q, webSearchEnabled=%v) = %v, want %v",
					c.intent, c.webSearchEnabled, got, c.want)
			}
		})
	}
}
