# R175 share dialog select wrapper

- Scope: `apps/web/src/knowledge-bases/KnowledgeBaseShareDialog.tsx`.
- Change: the hidden required organization select backing the custom picker now uses shared `Select`, preserving native form validation, keyboard semantics and the visible custom organization picker.
- Validation: Web typecheck and diff check passed; existing organization and shared UI suites remain green.
- Boundary: protected share/unshare runtime and same-session Vue visual comparison remain open.
