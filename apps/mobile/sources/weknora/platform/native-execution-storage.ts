import { Platform } from 'react-native';
import { getExecutionStorage, hasExecutionStorageFactory, registerExecutionStorageFactory, type ExecutionStorageFactory } from './execution-storage';

/**
 * Native host integration point. The Expo/native build supplies an encrypted
 * SQLite factory on the global bridge; web and test builds intentionally do
 * not claim durable execution recovery.
 */
export function registerNativeExecutionStorageFactory(factory: ExecutionStorageFactory): () => void {
  if (Platform.OS === 'web') return () => undefined;
  return registerExecutionStorageFactory(factory);
}

export function hasNativeExecutionStorageProvider(): boolean {
  return Platform.OS === 'web' || hasExecutionStorageFactory();
}

export { getExecutionStorage };
