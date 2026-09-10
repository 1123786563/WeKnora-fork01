# T23 mobile Lite capability evidence — 2026-09-11

## Environment

- iPhone 17 Pro simulator, iOS 26.5, native host `com.weknora.mobile`.
- Worktree Expo Metro bundle and the isolated SQLite Lite backend on
  `127.0.0.1:18082`.
- Authenticated owner account and tenant from the T20/T21 evidence; no
  production data or credentials.

## Native capability-driven management flow

The simulator opened the native `management/index` screen after the server
capability request completed. The page rendered:

- `Server edition: lite`.
- Core capabilities: knowledge bases/files, chat/SSE recovery/citations,
  attachments, and tool approvals.
- Read-only capabilities: members/roles/audit and Agents/models/MCP/Skills,
  with the server-owned tenant/role explanation.
- Read-only Wiki/FAQ with the explicit editing boundary.
- Unsupported Sandbox terminal, offline write queue, and Embed/IM
  administration, each with a concrete reason instead of an omitted row.

The native Identity screen then rendered the server capability projection,
including `organizations unavailable · not_supported_in_lite` and
`settings.sandbox.docker unavailable · docker_backend_disabled`, while keeping
the supported Lite resources visible. The Configuration screen rendered the
server-backed Agents inventory and marked each agent `Read-only on mobile`,
alongside Models, MCP services, and Skills headings.

This proves capability-gated routing and safe read-only/unsupported disclosure
against the real Lite backend. It does not prove viewer/admin negative tests,
member/audit data mutations, configuration writes, Wiki/FAQ writes, Sandbox
ticket/WS, or Embed/IM administration; T23 remains `review`.
