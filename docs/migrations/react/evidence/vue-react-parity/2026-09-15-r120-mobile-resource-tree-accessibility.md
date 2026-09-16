# Mobile data-source resource tree accessibility

Date: 2026-09-15  
Scope: `apps/mobile/src/features/knowledge/DataSourcesScreen.tsx`

Data-source resource rows now expose native accessibility semantics: expansion controls have localized labels, and selectable resources expose `checkbox` role plus a checked state derived from the existing cover/partial selection model. Visual selection and API payload behavior are unchanged.

Validation: mobile typecheck passed; mobile data-source/i18n focused tests passed 13/13; `git diff --check` passed.
