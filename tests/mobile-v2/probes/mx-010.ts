// MX-010 probe · 冷启动身份引导观察器
// 场景 valid-credential-missing-scope：凭据已恢复（SecureStore 适配器内）、scope 未知。
// 用真实 bootstrap（resolveScopeFromMe + createBootstrapPort，真实 AuthApi.me 契约路径）
// 与真实 credentials 适配器（注入 store）观测：身份补齐结果 + 凭据存储通道隔离。
// 存储通道隔离为双层真实观察：①动态——凭据经适配器写入/读回仅发生在注入的 SecureStore 通道；
// ②源级——凭据模块图（session.tsx/credentials.ts）不含任何普通存储 API（源文件实读检查）。
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  createBootstrapPort,
  resolveScopeFromMe,
} from '../../../apps/mobile/sources/weknora/auth/bootstrap.ts';
import { createCredentials } from '../../../apps/mobile/sources/weknora/auth/credentials.ts';
import type { AuthMe } from '@weknora/contracts';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  userId: string;
  tenantId: string;
  credentialInOrdinaryStorage: boolean;
}

const ORDINARY_STORAGE_APIS = ['AsyncStorage', 'localStorage', 'react-native-mmkv', 'MMKV'];

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'valid-credential-missing-scope') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }
  // 1) 真实 /auth/me 响应形状（AuthMe.memberships 为服务端成员关系行；本空间使用字符串 tenant_id）
  const me: AuthMe = {
    user: { id: 'u1' },
    tenant: null,
    memberships: [{ tenant_id: 't1', tenant_name: '默认空间', role: 'owner', status: 'active' }],
    tenant_required: false,
  } as unknown as AuthMe;

  // 2) 纯解析 + 真实端口（脚本化 transport 只替换网络边界）
  const resolved = resolveScopeFromMe(me, null);
  const seen: string[] = [];
  const port = createBootstrapPort({
    async me(): Promise<AuthMe> {
      seen.push('/api/v1/auth/me');
      return me;
    },
  } as never);
  const outcome = await port.run(null);

  // 3) 动态观察：凭据写入与读回全部经由注入的 SecureStore 通道
  const secureStore = new Map<string, string>();
  const adapter = createCredentials(
    {
      get: async (key) => secureStore.get(key) ?? null,
      set: async (key, value) => { secureStore.set(key, value); },
      remove: async (key) => { secureStore.delete(key); },
    },
    'weknora.product.credential',
  );
  await adapter.write({ kind: 'bearer', accessToken: 'at-cold-start', refreshToken: 'rt' });
  const roundTrip = await adapter.read();
  if (roundTrip.kind !== 'bearer') throw new Error('credential round-trip through the secure channel failed');
  if (!secureStore.has('weknora.product.credential')) throw new Error('credential must live in the injected secure store');

  // 4) 源级观察：凭据模块图不含普通存储 API（读真实源文件）
  const credentialSources = await Promise.all([
    readFile(path.join(repoRoot, 'apps/mobile/sources/weknora/auth/credentials.ts'), 'utf8'),
    readFile(path.join(repoRoot, 'apps/mobile/sources/weknora/auth/session.tsx'), 'utf8'),
  ]);
  const ordinaryHits = ORDINARY_STORAGE_APIS.filter((api) => credentialSources.some((source) => source.includes(api)));
  const credentialInOrdinaryStorage = ordinaryHits.length > 0;

  if (seen[0] !== '/api/v1/auth/me') throw new Error('bootstrap must call auth me');
  if (resolved.tenantId !== outcome.tenantId) throw new Error('port and pure resolution disagree');
  return {
    userId: outcome.userId,
    tenantId: outcome.tenantId ?? '',
    credentialInOrdinaryStorage,
  };
}
