# N031 mobile data-source resource selection

## Scope

Updated the native data-source editor to load resources for an existing source
through the existing `dataSources.resources(id)` API, render selectable rows,
preserve existing `config.resource_ids`, and include the selected IDs in the
update payload.

## Evidence

- Native-host component test proves the resource returned by the client is
  rendered and can be selected.
- Focused data-source screen tests: 4/4 pass.
- Full mobile suite: 92/92 pass.
- Mobile typecheck and `git diff --check`: pass.

## Remaining parity

This does not yet cover Vue's lazy hierarchical tree, Drive root-token setup,
temporary source creation before resource browsing, localized resource copy,
or iOS/Android runtime and real-backend evidence. N031 remains `implementing`.
