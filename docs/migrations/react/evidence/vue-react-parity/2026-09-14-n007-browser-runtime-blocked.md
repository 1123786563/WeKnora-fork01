# N007 browser runtime gate

Date: 2026-09-14

## Probe

- Browser viewport: 1355x720 CSS pixels.
- `http://127.0.0.1:3000/platform/knowledge-bases` loaded a Next.js 404 shell (`404`, `This page could not be found.`), so it is not the React migration runtime.
- `http://127.0.0.1:8080/` loaded the local service shell and returned `{"error":"Unauthorized: missing authentication"}` in the response body. The accessible tree exposed only the unauthenticated `美观输出` checkbox form; no authenticated tenant or knowledge-base route was reachable.

## Consequence

The upload-confirm dialog cannot be opened in this browser session. Vue/React same-viewport computed-style screenshots, selector interaction, and real-backend upload confirmation remain `blocked-env` pending an authenticated browser session with a tenant and knowledge base. This is an environment gate, not acceptance evidence for N007.
