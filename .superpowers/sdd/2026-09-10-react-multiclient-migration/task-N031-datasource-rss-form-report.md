# N031 mobile RSS connector form

The native data-source editor now exposes RSS feed URLs and optional custom
request headers. Feed URLs are required before save or connection test;
existing saved credentials remain redacted. The save/test paths use the
existing backend contract: `settings.feed_urls` and
`credentials.auth_headers`.

Evidence: focused screen and policy tests 14/14 pass; full mobile suite 94/94
pass; mobile typecheck and diff check pass. Other connector fields, exact
localized copy, live backend validation, and iOS/Android runtime evidence
remain open. N031 remains `implementing`.
