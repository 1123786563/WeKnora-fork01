import { Slot } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { MobileRuntimeProvider } from '../src/runtime.tsx';

export default function RootLayout() {
  return <MobileRuntimeProvider><StatusBar style="auto" /><Slot /></MobileRuntimeProvider>;
}
