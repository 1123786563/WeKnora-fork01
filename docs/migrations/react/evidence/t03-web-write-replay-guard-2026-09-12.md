# T03 Web write replay guard — 2026-09-12

During a transport review, the Web adapter was found to refresh and replay
any HTTP method after a 401. That was unsafe for non-idempotent operations such
as knowledge-base creation, uploads, and approval actions.

The adapter now allows automatic bearer refresh/retry for `GET`, `HEAD`, and
`OPTIONS` only. A JSON `POST` receiving 401 is returned unchanged after one
network attempt; it does not invoke the refresh callback or replay the body.
The existing binary read refresh path remains covered, and streaming keeps its
separate handshake behavior.

```text
node --import tsx --test apps/web/src/platform/http.test.ts   exit 0; 10/10
```

This closes the Web JSON write-replay defect. It does not claim that a server
can roll back a request whose bytes were already accepted before a 401, nor
does it replace the missing provider, browser role-matrix, or deployed
acceptance evidence for T03/T24.
