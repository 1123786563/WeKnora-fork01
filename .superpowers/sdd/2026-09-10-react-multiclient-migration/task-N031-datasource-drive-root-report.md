# N031 mobile Drive root resource flow

The native data-source editor now recognizes bare Feishu/Lark Drive folder
tokens and `/drive/folder/<token>` URLs through shared pure logic. For an
existing Drive source, it normalizes the token, persists it as
`config.resource_ids[0]`, and loads resources via the existing typed API. The
editor exposes an inline token field and loading/error state.

Evidence: focused screen and policy tests 10/10 pass; full mobile suite 94/94
pass; mobile typecheck and diff check pass. This remains partial parity:
new-source temporary creation, Vue's connector-specific credential forms,
localized copy/error classification, and real backend/iOS/Android evidence
are still missing. N031 remains `implementing`.
