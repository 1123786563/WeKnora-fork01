# R317 Platform rail icon fidelity (2026-09-15)

The platform rail now accepts multi-path inline SVG geometry and ports the Vue asset paths for chat, knowledge base and agent entries. This removes the generic outline approximations while retaining the existing accessible labels, active-state colors and navigation behavior.

Verification:

- Platform shell focused suite: 12/12 passed.
- Web typecheck: passed.
- Web full suite: 911/911 passed; failed/cancelled/skipped: 0.

The icon comparison was checked in the authenticated Chrome shell; responsive, other-locale and native screenshots remain open.
