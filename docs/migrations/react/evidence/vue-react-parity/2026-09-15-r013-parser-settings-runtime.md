# R013 Web parser settings runtime evidence

Date: 2026-09-15

- Authenticated React Chrome reached `/platform/settings?section=parser`.
- The page showed localized Chinese parser title, endpoint/API Key fields, endpoint placeholder wiring, and the safety hint: `当前 API Key 不会从服务端返回，也不会回填到此表单。`
- The parser test action was reachable; the save action remained disabled while the form was unchanged.
- The Vue comparison tab had no authenticated session, so same-session pixel parity remains open.

This is partial protected AX/runtime evidence; it does not certify a successful parser connection or backend mutation.
