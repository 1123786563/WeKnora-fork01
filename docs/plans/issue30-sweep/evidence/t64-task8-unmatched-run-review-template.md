# Task8 unmatched active Run disposition

Use this template only in the access-controlled deployment evidence store. Do not put tenant, Agent, session, or Run identifiers or signed reports in Git.

- Opaque evidence reference:
- Database identity hash:
- Schema version:
- Preflight query SHA-256:
- Frozen result-set SHA-256:
- Review owner:
- Review timestamp (UTC):

| Opaque row reference | Disposition | Evidence reference | Reviewer note |
| --- | --- | --- | --- |
| `<opaque-reference>` | `<classify or quarantine>` | `<restricted reference>` | `<sanitized rationale>` |

The release operator must verify the signed report and compare the live frozen-state result-set hash immediately before migration. Any unresolved row or hash mismatch blocks rollout.
