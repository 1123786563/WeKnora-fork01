# T13 React Markdown browser fixture evidence — 2026-09-12

## Scope and environment

This is a browser-level security/rendering check for the development-only
Markdown fixture. It uses the current `codex/react-multiclient` worktree at
`localhost:5174` and a disposable local HTTP mock on `127.0.0.1:18091` that
only returns fixture `auth/login` and `auth/me` responses. No production
account or backend data was used.

## Observed browser behavior

- Logged into the fixture with the React Web login form and opened
  `/platform/dev/markdown`.
- The rendered section exposed a real heading, list, safe external link,
  escaped `<img ... onerror=...>` text, a Mermaid diagram, and escaped
  `<script>alert('unsafe')</script>` text.
- After the lazy renderer settled, the DOM contained exactly one SVG and zero
  `script` elements inside the rendered section.
- The Mermaid source code block was replaced (`data-markdown-diagram` count
  `0`), the SVG `onload` attribute was absent (`null`), and the accessibility
  tree exposed `Mermaid diagram`.

## Evidence boundary

This proves the React development fixture reaches the browser DOM, the
sanitizer removes the tested SVG event attribute, and raw HTML does not become
an executable script in this fixture. It does not prove protection against
every browser payload, live production citation navigation, protected artifact
download/preview, long-message performance, or native WebView behavior.
T13 remains `review`.
