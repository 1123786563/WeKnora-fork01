import { Platform } from 'react-native';
import { getExecutionStorage, registerExecutionStorageFactory, type ExecutionStorageFactory } from './execution-storage';

/**
 * Native host integration point. The Expo/native build supplies an encrypted
 * SQLite factory on the global bridge; web and test builds intentionally do
 * not claim durable execution recovery.
 */
export function registerNativeExecutionStorage(): () => void {
  if (Platform.OS === 'web') return () => undefined;
  const factory = (globalThis as typeof globalThis & { __WEKNORA_EXECUTION_STORAGE__?: ExecutionStorageFactory }).__WEKNORA_EXECUTION_STORAGE__;
  if (!factory) return () => undefined;
  return registerExecutionStorageFactory(factory);
}

export function hasNativeExecutionStorageProvider(): boolean {
  return Platform.OS === 'web' || typeof (globalThis as typeof globalThis & { __WEKNORA_EXECUTION_STORAGE__?: unknown }).__WEKNORA_EXECUTION_STORAGE__ === 'function';
}

export { getExecutionStorage };
