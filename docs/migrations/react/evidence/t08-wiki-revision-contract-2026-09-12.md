# T08 Wiki revision contract follow-up — 2026-09-12

The shared Wiki API parser previously accepted any JavaScript `number` for
revision pagination and version fields. That allowed fractions, negatives,
and unsafe integers to cross the API boundary.

Revision list pagination and current version now require safe non-negative
integers; each revision version requires a safe positive integer. Optional
content, edit-source, and timestamp fields are validated as strings in both
revision-list and revision-detail responses.

```text
node --import tsx --test packages/api-client/src/wiki/pages.test.ts   exit 0; 7/7
pnpm test:shared                                                     exit 0; 188/188
pnpm typecheck:shared                                                exit 0
pnpm test:web                                                         exit 0; 106/106
pnpm typecheck:web                                                    exit 0
node scripts/check-react-boundaries.mjs                               exit 0
```

This closes the client-side Wiki revision numeric-validation gap. Live
conflict/revert/import/export and full browser/provider acceptance remain
separate T08 evidence requirements.
