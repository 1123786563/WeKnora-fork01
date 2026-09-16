# R195 organization preview actions

- Scope: organization preview detail state.
- Vue baseline: preview details expose a clickable short space-ID chip with copy feedback; already-member previews show a primary `查看共享空间` action that opens the organization settings surface while retaining the preview context.
- Change: React now renders the short-ID copy chip, adds the already-member view action, hydrates the settings editor from preview data, loads member/request/share feeds, and raises the settings overlay above the join overlay.
- Validation: organization focused tests 26/26, Web typecheck, Web full regression 895/895, and `git diff --check` pass.
- Boundary: the existing test fixture covers approval-gated preview/join; a protected backend already-member preview and clipboard/browser confirmation were not available in this pass.
