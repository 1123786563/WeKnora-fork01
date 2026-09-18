import { Slot } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { MobileRuntimeProvider } from '../src/runtime.tsx';
import { createMobileAppPersonalNode } from '../sources/platform/personalNodeApp.ts';

export function createRootPersonalNode() {
  return createMobileAppPersonalNode();
}

export default function RootLayout() {
  const personalNode = createRootPersonalNode();
  return <MobileRuntimeProvider personalNode={personalNode}><StatusBar style="auto" /><Slot /></MobileRuntimeProvider>;
}
