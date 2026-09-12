# T04 integrations scope tenant follow-up — 2026-09-12

The Web integrations page previously read `weknora_selected_tenant_id`
directly from browser storage. That value is a legacy persistence input and
can lag behind the tenant selected by the current authenticated scope.

The page now receives the tenant from `scopeRuntime.current().scope.tenantId`
and parses it only when it is a positive safe integer. API Principal reads,
updates, and test-token creation all use that same scoped value; absent or
malformed scope values fail closed without issuing a tenant API request.

```text
node --import tsx --test apps/web/src/integrations/tenant.test.ts   exit 0; 1/1
pnpm test:web                                                      exit 0; 105/105
pnpm typecheck:web                                                 exit 0
pnpm build:web                                                      exit 0
node scripts/check-react-boundaries.mjs                             exit 0
```

This closes the stale-local-storage tenant selection defect in the React Web
integration route. It does not claim the full browser role/tenant matrix or
provider callback acceptance.
