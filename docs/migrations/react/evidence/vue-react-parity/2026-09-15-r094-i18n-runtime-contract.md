# React/Vue parity evidence — i18n bootstrap runtime contract

Date: 2026-09-15

The lightweight i18n bootstrap module now has direct contract coverage for the complete five-locale set, locale rejection, and localized loading copy. This protects the route-level lazy-loading boundary from accidental catalog or fallback drift.

- Shared tests: 467/467 passed.
- Web typecheck: passed.
- `git diff --check`: passed.
