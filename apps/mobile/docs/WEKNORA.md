# WeKnora Direct connection

Conduit can chat with a self-hosted [WeKnora](https://github.com/Tencent/WeKnora)
backend (Tencent's open-source knowledge-base RAG service) through a first-class
Direct provider adapter (`weknora`).

## Setup

1. In WeKnora, create a **workspace API key** (Tenant settings → API keys) with
   these capabilities: `chat`, `retrieve`, `read_agents`, and optionally
   `manage_models` (only needed to list chat models in discovery — without it,
   discovery degrades to knowledge bases + agents). A full-access key also
   works. Platform-level keys must additionally send an `X-Tenant-ID` header
   (add it as a custom header in the connection editor).
2. In Conduit: Settings → Direct connections → add, provider type **WeKnora**.
   - Base URL: your WeKnora root, e.g. `http://192.168.1.10:8080` (the `/api/v1`
     prefix is appended automatically; a base URL that already ends in
     `/api/v1` also works).
   - Authentication: API key header (default). Paste the `sk-…` token; the
     adapter sends it as `X-API-Key`. Alternatively choose Bearer and paste a
     JWT login token (expires after 24 h).

## Models

Model discovery lists three kinds of entries:

| Model id | Meaning | Chat endpoint |
|---|---|---|
| `kb:<kb_id>` | One knowledge base — RAG chat scoped to it | `POST /api/v1/knowledge-chat/:session_id` |
| `agent:<agent_id>` | A smart-reasoning agent (server-side tools) | `POST /api/v1/agent-chat/:session_id` |
| `model:<model_id>` | A configured chat model (`KnowledgeQA`/`VLLM`) — plain LLM chat | `POST /api/v1/knowledge-chat/:session_id` |

A bare manual model id (no prefix) is treated as a chat model id. Manual model
ids (`kb:…`/`agent:…`/`model:…`) skip discovery entirely.

## How conversations map to WeKnora sessions

WeKnora chats are server-side sessions; the Direct contract is stateless. The
adapter keeps an in-memory binding per conversation: it matches a request to a
server session by the exact sequence of earlier user messages, then sends only
the latest user message (history lives on the server).

Consequences:

- Normal follow-ups reuse one WeKnora session.
- Edited, regenerated, or branched turns no longer match, so the adapter
  creates a fresh session and replays the prior conversation as a bounded
  `[Conversation context]` prefix inside the query.
- Bindings are in-memory only: after an app restart the first turn creates a
  new WeKnora session.

## Streaming

- SSE frames (`event: message`) map as: `answer` → content deltas, `thinking`
  → reasoning deltas, `tool_call`/`tool_result` → formatted reasoning lines
  (WeKnora tools run server-side; Conduit's local tool runtime is not used),
  `complete` → usage + done, `error` → stream error.
- `references` frames become tappable sources when the reference itself is a
  web URL (`knowledge_source` starting with `http(s)://`); all other
  references are appended to the answer as a numbered markdown source list.
- Images attached to the latest user message are sent as `images` (up to 8).
- Web search (when enabled for the connection/model) maps to WeKnora's
  `web_search_enabled`.
- Stopping a reply calls `POST /api/v1/sessions/:id/stop` with the assistant
  message id captured from the first `agent_query` frame. Concurrent turns on
  the same session are serialized client-side; a server-side HTTP 409 is
  surfaced with a friendly message.
- Requests ask for `resource_urls=public` so images resolve as direct links,
  falling back to handle mode once for restricted API keys.

## Known limitations (v1)

- No client-side tools / image generation (server-side agent tools are shown
  as process lines in the reasoning stream).
- `continue-stream` reconnect after network loss, steer, and tool-approval
  frames are not handled yet.
- Chat-scoped API keys cannot list models (`manage_models` gate) — discovery
  silently omits that group.
