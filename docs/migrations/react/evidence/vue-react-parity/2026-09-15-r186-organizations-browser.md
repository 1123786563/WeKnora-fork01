# R186 organizations browser parity

- Conditions: Chrome, same authenticated seeded account (`paritytester`), zh-CN, 1355x720 viewport, React `:5181` and Vue `:5173`.
- Visual check: both pages use the same left platform rail plus narrow shared-space rail; the React empty-state illustration, heading, description and action row align with the Vue composition at the captured viewport.
- React accessibility/runtime: platform and shared-space navigation expose links, the three scope controls expose buttons, and the empty state exposes join/create actions. Activating `我创建的` updates the URL to `?scope=created` and changes the empty-state copy to the created-scope variant.
- Vue runtime: the same route renders the shared-space empty state under the authenticated seeded session, but its accessibility tree exposes the scope rail as presentation containers, so no stronger control-level comparison is claimed.
- Boundary: no organization was available to exercise protected create/join/member mutations; responsive widths, other locales, and native/Wails rendering remain open.
