# N031 mobile RSS connector form

The native data-source editor now exposes the Vue RSS-specific feed URL and
optional custom request-header fields. It requires feed URLs before save or
connection test, keeps stored credentials redacted on edit, and writes
`settings.feed_urls` plus `credentials.auth_headers` using the existing RSS
connector contract.

Evidence: focused screen and policy tests 14/14 pass; full mobile suite 94/94
pass; mobile typecheck and diff check pass. Other connector forms, exact
localized copy, live backend validation, and iOS/Android runtime evidence
remain open. N031 remains `implementing`.
