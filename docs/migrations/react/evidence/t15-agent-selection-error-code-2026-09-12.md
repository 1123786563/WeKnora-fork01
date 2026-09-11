# T15 Agent selection and structured deletion errors (2026-09-12)

## Scope

This follow-up closes two Web-side contract gaps in the T15 configuration slice:

- the Agent selector is now rendered at the actual chat entry and is connected to the stream request;
- numeric backend lifecycle error codes remain structured instead of being downgraded to `HTTP_400`.

## Implementation evidence

- Commit `6a70877` adds `apps/web/src/chat/agent-selection.ts` and wires `ChatRoutePage` to `client.configuration.agents.listWithState({ creator: 'all' })`.
- A valid enabled selection sends `POST /api/v1/agent-chat/<session>` with `agent_enabled: true`, `agent_id`, `query`, and `channel: "web"`.
- Clearing the selection sends the existing knowledge-chat request shape and path. A URL `agentId` is accepted only when the returned Agent exists and is not in `disabled_own_agent_ids`; disabled rows are rendered disabled in the chat selector.
- `packages/api-client/src/errors.ts` now preserves finite numeric error codes as strings. The regression fixture for `{ error: { code: 2300, ... } }` observes `ApiError.code === "2300"` and retains structured details for the model-in-use UI.
- The configuration operation panel now labels its selector as an operation selector for sharing/hide/show, while directing conversation selection to the chat entry.

## Verification

- `pnpm --filter @weknora/web test`: exit 0, 91/91.
- `pnpm test:shared`: exit 0, 179/179.
- `pnpm typecheck:shared`: exit 0.
- `pnpm typecheck:web`: exit 0.
- `pnpm build:web`: exit 0, 126 modules transformed.
- `node scripts/check-react-boundaries.mjs`: exit 0.
- `git diff --check`: exit 0.

These are static, Node-render, and production-bundle proofs. No new live provider/model call, real Agent-tool turn, OAuth flow, or browser acceptance is claimed by this follow-up. T15 therefore remains `review`.
