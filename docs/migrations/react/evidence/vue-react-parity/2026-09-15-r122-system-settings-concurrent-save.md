# System settings concurrent-save state

Date: 2026-09-15  
Scope: `apps/web/src/settings/SystemGlobalSettingsPanel.tsx`

System settings now track saving keys as a set instead of one global key. Concurrent updates to different settings keep each control disabled only while its own request is pending, and an earlier request cannot prematurely clear a later setting's busy state.

Validation: Web typecheck passed; settings suite passed 179/179; `git diff --check` passed.
