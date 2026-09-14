# Vue ↔ React parity evidence — mobile Wiki graph preview

- Added a dependency-free native graph preview: deterministic two-column node layout, center-node emphasis, localized expand-neighbor action, and retained detailed node metadata below the preview.
- The preview caps visual nodes at 40 to keep native layout responsive while the existing API result and filtering semantics remain unchanged.
- Added deterministic layout regression coverage.
- Validation: mobile suite 189/189, mobile typecheck, and `git diff --check` passed.
- Runtime limitation: authenticated device graph data and full Vue computed-style comparison remain pending.
