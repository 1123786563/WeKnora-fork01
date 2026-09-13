# N031 mobile new-source Drive browsing

New Drive drafts now follow the Vue-style temporary-source flow. After a
valid folder token is entered, the mobile editor creates a paused temporary
data source with the root in `config.resource_ids`, loads resources through
the existing typed API, and tracks the temporary ID. Saving updates that row
through the normal path; canceling removes the temporary row and leaves the
editor.

Evidence: focused native-host screen plus resource-policy tests 12/12 pass;
full mobile suite 94/94 pass; mobile typecheck and diff check pass. Remaining
gaps are termination cleanup, connector-specific credential forms,
localized copy/error classification, and real backend/iOS/Android evidence.
N031 remains `implementing`.
