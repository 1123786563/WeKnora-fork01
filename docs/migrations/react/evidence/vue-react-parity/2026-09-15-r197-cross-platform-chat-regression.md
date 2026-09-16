# R197 cross-platform regression after chat layout fix

- Validation: Desktop tests 2/2 and typecheck pass; Embed tests 7/7 and typecheck pass; Mobile tests 190/190 and typecheck pass; Desktop renderer build and React Web/Embed bundle build pass.
- The shared chat layout change introduces no cross-package compile or unit regression. Vite emits the existing large-chunk advisory in Web/Desktop/Embed builds.
- Boundary: Wails host startup, real Embed container behavior, iOS/Android native screenshots and live backend streams still require runtime environments.
