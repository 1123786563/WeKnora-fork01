# T12 Web chat approval actions (2026-09-12)

## Scope

The Web chat now exposes the approval state already produced by the shared chat reducer and connects the user actions to the existing typed API client.

## Implementation evidence

- Commits `2c440f0` and `236f003` add Web chat action cards for pending and resolved tool approvals and MCP OAuth prompts, plus a session-scoped follow-up form.
- Pending tool approvals expose explicit `Approve` and `Reject` actions backed by `client.chat.approvals.resolveTool`.
- Pending MCP OAuth prompts expose `Authorize` and `Cancel`. Authorize starts the typed MCP authorization URL flow, opens the provider popup, polls the attempt-bound status, and resolves the paused agent run only after the server reports authorization. Cancel uses the dedicated cancellation route.
- The cards remain visible while the stream is paused and show resolved state after the corresponding SSE event. Failures are surfaced in the action area and are not converted into success.
- A session with an active chat entry exposes `Follow-up while running`; successful submissions call `client.chat.steer.enqueue` with `delivery: "after"` and `channel: "web"`, while failures preserve the draft.
- The existing shared reducer continues to own event deduplication and lifecycle state; the Web view receives only its public approval/OAuth projections.

## Verification

- `pnpm --filter @weknora/web test`: exit 0, 91/91.
- `pnpm test:shared`: exit 0, 179/179.
- `pnpm typecheck:shared`: exit 0.
- `pnpm typecheck:web`: exit 0.
- `pnpm build:web`: exit 0, 126 modules transformed.
- `node scripts/check-react-boundaries.mjs`: exit 0.
- `git diff --check`: exit 0.

The test and bundle evidence proves the Web action surface and typed wiring. No live MCP provider, OAuth callback, paused agent turn, or browser popup completion is claimed here; T12 remains `review` pending those runtime gates.
