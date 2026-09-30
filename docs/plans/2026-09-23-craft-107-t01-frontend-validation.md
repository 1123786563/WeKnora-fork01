# T01 / #120 Frontend validation

Status: **DONE_WITH_CONCERNS**

## Scope and revision

- Validated the seven-file T01 checkpoint in `/Users/wuyongjun/.codex/worktrees/craft-107-t01/WeKnora-fork01` at `43dcff3454d8f7ec70a37a8c06ba3087b7947b98`.
- Manifest SHA-256: `5dfe2bfbd39c46415dd5d3a65d757836dab9f75a564f8286320ecdffc31327ef` (matches the assigned manifest).
- Role/model/reasoning metadata is not exposed in this agent context; not inferred.
- No source or test files were modified. The report is the only output written.

## Checks run

| Exact command | Result |
| --- | --- |
| `pnpm exec tsx --test packages/views/src/craft/files.input.test.tsx` | Exit 0; 1 passed, 0 failed. Exercises static markup only. |
| `pnpm typecheck:web` | Exit 0; no TypeScript diagnostics. |
| `rg -n "CraftInputDecisionPanel|onDecide\\(" --glob '!**/node_modules/**' .` | Found the component and test only; no composition or consumer of the decision panel exists. |
| `rg -n "CraftWorkbenchFeature|slot|feature" packages/views/src/craft/workbench.tsx packages/views/src/craft/workbench-features.tsx` | Workbench has generic `header`, `conversation`, and `aside` slots; their context carries session ID, write permission, and selected version only. |

The T01 report records the same focused test and web typecheck as passing in its checkpoint verification, but did not test actual interaction or browser composition. The two checks above were rerun against the assigned checkpoint revision.

## Acceptance observations

- The standalone panel discloses the input name and distinguishes “accepted” from “not understood”. It presents native Continue and Cancel buttons, disables both while a decision request is pending, and shows a localized acknowledgement after the callback resolves.
- Callback rejection leaves the decision actionable and displays an error using `role="alert"`.
- The focused test only checks server-rendered static text and button labels. It does not exercise click behavior, pending/error transitions, or assertions that Cancel avoids Run and Continue submits exactly once.
- **Acceptance gap:** `CraftInputDecisionPanel` has no caller in the repository. `CraftWorkbench` does not accept input views or an input decision callback; generic feature slots cannot provide either in their context. Consequently the unknown-input disclosure and choice are not part of the live workbench, and browser behavior for the user journey is unverified. The T01 implementation report itself marks controller web wiring as outstanding. Continue/cancel-to-Run behavior is covered by backend evidence, not by a mounted frontend journey.
- Accessibility is partial: native buttons and `role="alert"` are present; acknowledged decisions and pending state have no live-region/status semantics. No browser or assistive-technology check is possible for an unmounted panel.
- Responsive behavior is unverified. No browser-level rendering, narrow viewport, keyboard, or focus-flow evidence exists for this isolated component.

## Risks and disposition

The component-level static render and TypeScript checks pass, but T01 frontend acceptance cannot be considered complete until the controller/workbench composition supplies actual input state and a decision callback, and an interaction-level test confirms no Run after Cancel and exactly one Run after acknowledged Continue. No code was changed during this validation.
