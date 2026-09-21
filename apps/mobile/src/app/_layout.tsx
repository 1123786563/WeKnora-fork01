import { Stack } from 'expo-router';

/** The router hosts one Runtime-selected mobile surface at a time. */
export default function RootLayout() {
  return <Stack screenOptions={{ headerShown: false }} />;
}
