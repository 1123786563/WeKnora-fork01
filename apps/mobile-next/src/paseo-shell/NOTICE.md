# Paseo attribution

`PaseoScreenTitle.tsx` is adapted from Paseo's
`packages/app/src/components/headers/screen-title.tsx` at
`d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b`, licensed under Apache-2.0.

The adaptation preserves the title's `children`, `numberOfLines`, `testID`, and
`style` interface. It replaces `react-native-unistyles` theme access with the
mobile app's generated tokens and React Native `StyleSheet`, and the surrounding
shell uses a supplied route callback rather than Paseo navigation state.
