// MX-030 probe · 退出序列观察器
// frozen 场景：active-credential-and-device × late-token-refresh。
// 真实 createPreferenceStore.logout：teardown（关流/语音）→ 撤销设备 → 清凭据/本 scope 缓存；
// 退出前捕获的凭据写守卫在退出后写 token 必须被拒（oldTokenRewritten=false）。
import {
  createPreferenceStore,
  type PreferenceStoreBackend,
} from '../../../apps/mobile/sources/weknora/preferences/store.ts';

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  credential: string | null;
  activeStreams: number;
  oldTokenRewritten: boolean;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'active-credential-and-device' || input.fault !== 'late-token-refresh') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  const scope = { origin: 'https://weknora.example', userId: 'u1' };
  const otherUserScope = { origin: 'https://weknora.example', userId: 'u2' };
  const storage = new Map<string, string>([
    [`${scope.origin}:u1:credential`, JSON.stringify({ kind: 'bearer', accessToken: 'active-token' })],
    [`${otherUserScope.origin}:u2:credential`, JSON.stringify({ kind: 'bearer', accessToken: 'other-user-token' })],
    [`weknora.prefs.v1:${scope.origin}:${scope.userId}`, JSON.stringify({ theme: 'dark' })],
    [`weknora.prefs.v1:${otherUserScope.origin}:u2`, JSON.stringify({ theme: 'light' })],
  ]);
  const backend: PreferenceStoreBackend = {
    get: async (key) => storage.get(key) ?? null,
    set: async (key, value) => { storage.set(key, value); },
    remove: async (key) => { storage.delete(key); },
  };

  let activeStreams = 1; // 活动流在途
  let deviceRevoked = false;
  const store = createPreferenceStore(backend, scope);

  // 退出前在途的 token 刷新捕获写守卫
  const inFlightGuard = store.captureCredentialGuard();
  if (!inFlightGuard) throw new Error('precondition: guard captured while logged in');

  await store.logout({
    teardown: async () => { activeStreams = 0; },
    revokeDevice: async () => { deviceRevoked = true; },
    clearCredentials: async (target) => {
      storage.delete(`${target.origin}:${target.userId}:credential`);
    },
    credentialWriteGuard: () => null,
  });

  if (activeStreams !== 0) throw new Error('logout must close active streams');
  if (!deviceRevoked) throw new Error('logout must revoke the device registration');

  // 迟到的 token 刷新：退出后写回必须被拒
  let oldTokenRewritten = false;
  try {
    await inFlightGuard.write('refreshed-new-token');
    oldTokenRewritten = true;
  } catch {
    oldTokenRewritten = false;
  }
  const credentialRaw = storage.get(`${scope.origin}:u1:credential`) ?? null;
  const credential = credentialRaw === null ? null : (JSON.parse(credentialRaw) as { accessToken: string }).accessToken;

  // 不清别的账户数据
  if (!storage.has(`weknora.prefs.v1:${otherUserScope.origin}:u2`)) {
    throw new Error('logout must not clear other accounts data');
  }

  return { credential, activeStreams, oldTokenRewritten };
}
