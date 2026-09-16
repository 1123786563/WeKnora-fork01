# N031 mobile data-source resource tree

The native editor now renders resource rows with expand/collapse controls.
When an expandable row has not been loaded, it calls the existing typed
`dataSources.resources(sourceId, parentId)` API, merges de-duplicated children,
and keeps the child rows selectable. Loading and request errors remain visible
in the editor.

Evidence: focused native-host and resource-policy tests 8/8 pass; full mobile suite 93/93 pass;
mobile typecheck and `git diff --check` pass. This is not full parity yet:
Vue's minimal-cover selection semantics, expand/collapse-all controls, Drive
root-token setup, new-source temporary browsing, localization, and real
backend/iOS/Android evidence remain outstanding. N031 stays `implementing`.
