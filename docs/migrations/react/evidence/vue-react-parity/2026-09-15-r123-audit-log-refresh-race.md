# Audit log refresh race guard

Date: 2026-09-15  
Scope: `apps/web/src/settings/SystemAuditLogPanel.tsx`

Audit-log refreshes now carry a generation token. A reset invalidates prior pagination responses, so a late page cannot append stale rows or clear the loading/error state belonging to the current refresh.

Validation: Web typecheck passed; settings tests passed 179/179; `git diff --check` passed.
