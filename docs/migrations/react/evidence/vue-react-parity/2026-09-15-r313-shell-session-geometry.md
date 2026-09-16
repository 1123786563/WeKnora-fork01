# R313 Platform shell session geometry (2026-09-15)

## Runtime comparison

With the same authenticated account, route (`/platform/organizations`), locale (`zh-CN`) and 1355×720 Chrome viewport, the React shell was compared against the Vue shell at `localhost:5181` and `localhost:5173`.

- React and Vue keep the same 260px primary rail and 55px secondary rail.
- React header height was reduced from 50px to 42px so the first platform navigation row starts at the same vertical position as Vue.
- The shell session region no longer adds a second top margin or visible heading. Its `今天` group and first row now align with Vue; shell-only button padding is reduced to match the Vue row rhythm while the shared chat sidebar remains unchanged.
- Navigation, session rows, tenant identity and empty shared-space content remain present after the adjustment.

## Verification

- Live paired Chrome screenshots were inspected after refresh on both ports.
- Platform shell focused tests: 12/12 passed.
- `pnpm test:web`: 911/911 passed; failed/cancelled/skipped: 0.
- `pnpm run typecheck:web`: passed.

## Remaining evidence gap

This runtime comparison covers the authenticated desktop empty-state shell. Non-empty sessions, mobile/responsive breakpoints, other locales, protected mutations and Wails/native launch evidence remain open.
