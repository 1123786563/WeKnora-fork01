import { Slot } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { MobileRuntimeProvider } from '../src/runtime.tsx';
import { createMobileAppPersonalNode } from '../sources/platform/personalNodeApp.ts';

export default function RootLayout() {
  const personalNode = createMobileAppPersonalNode();
  return <MobileRuntimeProvider personalNode={personalNode}><StatusBar style="auto" /><Slot /></MobileRuntimeProvider>;
}
