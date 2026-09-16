# React/Vue parity evidence — mobile chat load race guards

Date: 2026-09-15

Mobile chat session, knowledge-base, message-history, attachment, and steer-queue loads now use independent generation tokens plus active-session checks. A late response from a prior session or refresh cannot overwrite the current conversation state.

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check`: passed.

Native live chat/session interaction evidence remains pending.
