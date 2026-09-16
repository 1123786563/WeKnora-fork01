# N016 MCP custom-header actions

Vue renders the custom-header add action as a compact green text button with an add icon, and each remove action as a 24px square icon control. React now uses the same lightweight controls, localized labels, and accessible hidden delete text while preserving header mutation behavior.

Verification: MCP settings tests 14/14, Web typecheck, and `git diff --check` passed. Authenticated browser computed-style and live mutation evidence remain open.
