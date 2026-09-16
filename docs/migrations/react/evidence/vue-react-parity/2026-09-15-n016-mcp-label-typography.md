# N016 MCP editor field-label typography

Vue MCP editor labels use 13px medium text with 6px label/control spacing. React now scopes the same typography to `.wk-mcp-form` labels while excluding checkbox labels, removing broad form utility selectors that could leak into nested controls.

Verification: MCP settings tests 14/14, Web typecheck, and `git diff --check` passed. Authenticated browser computed-style and native drawer evidence remain open.
