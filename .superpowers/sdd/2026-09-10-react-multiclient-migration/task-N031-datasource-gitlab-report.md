# N031 mobile GitLab connector form

The native data-source editor now mirrors the Vue GitLab project configuration
surface: project ID, optional ref, newline/comma-separated paths, add/remove
rows, edit hydration from `settings.projects`, and a required-project guard.
The save path serializes the existing backend connector shape and does not add
an endpoint.

Evidence: focused screen and resource-policy tests 13/13 pass; full mobile
suite 94/94 pass; mobile typecheck and diff check pass. Remaining gaps include
exact localized strings, other connector-specific credential forms, live
backend validation, and iOS/Android runtime evidence. N031 remains
`implementing`.
