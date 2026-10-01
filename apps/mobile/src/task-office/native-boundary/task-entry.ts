import type { TaskHandle, TaskListPage, TaskOffice } from '@weknora/mobile-core';
import type { RuntimeSurface } from '@weknora/mobile-core';

export interface AuthorizedTaskEntryPorts {
  /** Resolve snapshot, lease presence and tenant-scoped office at each interaction. */
  current(): {
    surface: RuntimeSurface;
    /** Runtime scopeLease() is minted only for a verified authorized scope. */
    hasScopeLease: boolean;
    office?: Pick<TaskOffice, 'open' | 'tasks'>;
  };
}

export class NativeTaskEntryError extends Error {
  constructor(readonly code: 'TASK_ENTRY_AUTH_REQUIRED' | 'TASK_ENTRY_INVALID_TASK' | 'TASK_ENTRY_UNAVAILABLE') {
    super(code === 'TASK_ENTRY_AUTH_REQUIRED' ? 'An authorized scope is required to open this Task.' : code === 'TASK_ENTRY_INVALID_TASK' ? 'A Task and Run identifier are required.' : 'Task Office is unavailable.');
    this.name = 'NativeTaskEntryError';
  }
}

/** Thin native boundary: it can open only a user-selected existing Task and never starts one. */
export function createAuthorizedTaskEntry(ports: AuthorizedTaskEntryPorts): {
  listExisting(): Promise<TaskListPage>;
  openExisting(input: { taskId: string; runId: string }): TaskHandle;
} {
  const currentOffice = (): Pick<TaskOffice, 'open' | 'tasks'> => {
    const current = ports.current();
    if (current.surface !== 'authorized' || !current.hasScopeLease) throw new NativeTaskEntryError('TASK_ENTRY_AUTH_REQUIRED');
    if (current.office === undefined) throw new NativeTaskEntryError('TASK_ENTRY_UNAVAILABLE');
    return current.office;
  };
  return {
    async listExisting() {
      return currentOffice().tasks({ limit: 30 });
    },
    openExisting(input) {
      const taskId = input.taskId.trim();
      const runId = input.runId.trim();
      if (taskId === '' || runId === '') throw new NativeTaskEntryError('TASK_ENTRY_INVALID_TASK');
      return currentOffice().open({ taskId, runId });
    },
  };
}

export type NativeCapability = { status: 'installed-untested' } | { status: 'unavailable'; reason: string };
export type NativeTaskCapabilities = Record<'navigation' | 'fileSelection' | 'fileDownload' | 'systemShare' | 'notifications' | 'secureStorage', NativeCapability>;
export type NativeTaskCapabilityPresence = Record<keyof NativeTaskCapabilities, boolean>;

export function probeNativeTaskCapabilities(presence: NativeTaskCapabilityPresence): NativeTaskCapabilities {
  return Object.fromEntries(Object.entries(presence).map(([name, available]) => [
    name,
    available ? { status: 'installed-untested' } : { status: 'unavailable', reason: 'native adapter is not installed' },
  ])) as NativeTaskCapabilities;
}

/** Detects installed Expo/native APIs lazily, so importing the probe in Node tests has no native side effects. */
export function detectNativeTaskCapabilities(): NativeTaskCapabilities {
  // Metro rejects dynamic require(name) ("Invalid call at line 59: require(name)") —
  // static literals keep build-time resolution and the try/catch preserves the
  // Node-test "absent module → undefined" semantics.
  const load = (name: string): Record<string, unknown> | undefined => {
    try {
      switch (name) {
        case 'expo-router': return require('expo-router') as Record<string, unknown>;
        case 'expo-document-picker': return require('expo-document-picker') as Record<string, unknown>;
        case 'expo-file-system': return require('expo-file-system') as Record<string, unknown>;
        case 'react-native': return require('react-native') as Record<string, unknown>;
        case 'expo-notifications': return require('expo-notifications') as Record<string, unknown>;
        case 'expo-secure-store': return require('expo-secure-store') as Record<string, unknown>;
        default: return undefined;
      }
    } catch { return undefined; }
  };
  const router = load('expo-router');
  const picker = load('expo-document-picker');
  const fileSystem = load('expo-file-system');
  const reactNative = load('react-native');
  const notifications = load('expo-notifications');
  const secureStore = load('expo-secure-store');
  return probeNativeTaskCapabilities({
    navigation: router !== undefined,
    fileSelection: picker !== undefined && typeof picker.getDocumentAsync === 'function',
    fileDownload: fileSystem !== undefined && typeof (fileSystem.File as { downloadFileAsync?: unknown } | undefined)?.downloadFileAsync === 'function',
    systemShare: reactNative !== undefined && typeof reactNative.Share === 'object' && reactNative.Share !== null,
    notifications: notifications !== undefined && typeof notifications.getPermissionsAsync === 'function',
    secureStorage: secureStore !== undefined && typeof secureStore.getItemAsync === 'function' && typeof secureStore.setItemAsync === 'function',
  });
}
