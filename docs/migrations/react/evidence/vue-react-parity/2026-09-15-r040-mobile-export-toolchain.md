# Mobile export toolchain evidence

- Command attempted: `pnpm --filter @weknora/mobile exec expo export --platform web --output-dir /tmp/weknora-mobile-export`
- Result: Expo rejected Web export because `react-dom@19.2.0` and `react-native-web@^0.21.0` are not installed.
- Decision: no dependencies were added; the mobile package targets native runtime and adding Web dependencies would change scope.
- Existing evidence remains separate: Mobile typecheck and 189/189 tests pass. iOS/Android native compile and device launch remain unverified.
