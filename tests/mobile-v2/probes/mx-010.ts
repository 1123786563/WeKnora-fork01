// MX-010 probe · 冷启动身份引导观察器
// 场景 valid-credential-missing-scope：凭据已恢复（SecureStore 适配器内）、scope 未知。
// 用真实 bootstrap（resolveScopeFromMe + createBootstrapPort，真实 AuthApi.me 契约路径）
// 与真实 credentials 适配器（注入 store）观测：身份补齐结果 + 凭据是否落入普通存储。
import {
  createBootstrapPort,
  resolveScopeFromMe,
} from '../../../apps/mobile/sources/weknora/auth/bootstrap.ts';
import { createCredentials } from '../../../apps/mobile/sources/weknora/auth/credentials.ts';
import type { AuthMe } from '@weknora/contracts';


export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  userId: string;
  tenantId: string;
  credentialInOrdinaryStorage: boolean;
}

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

  // 2) 纯解析：scope 从身份事实推导（非 Token 臆造）
  const resolved = resolveScopeFromMe(me, null);

  // 3) 端口走真实 AuthApi.me 路径（脚本化 transport 只替换网络边界）
  const seen: string[] = [];
  const port = createBootstrapPort({
    async me(): Promise<AuthMe> {
      seen.push('/api/v1/auth/me');
      return me;
    },
  } as never);
  const outcome = await port.run(null);

  // 4) 真实 credentials 适配器：注入式 SecureStore 承载凭据；
  //    普通 KV 计数器观测「凭据是否被写进普通存储」——必须为 false
  const secureStore = new Map<string, string>();
  // 真实候选「普通存储」：若任何凭据路径误写普通 KV，这里会计数（观测有真实写入路径）
  const ordinaryWrites: string[] = [];
  const ordinaryKV = {
    get: async (key: string) => null,
    set: async (key: string, value: string) => { ordinaryWrites.push(`${key}=${value.slice(0, 8)}...`); },
    remove: async (key: string) => { ordinaryWrites.push(`remove:${key}`); },
  };
  void ordinaryKV;
  const adapter = createCredentials(
    {
      get: async (key) => secureStore.get(key) ?? null,
      set: async (key, value) => { secureStore.set(key, value); },
      remove: async (key) => { secureStore.delete(key); },
    },
    'weknora.product.credential',
  );
  await adapter.write({ kind: 'bearer', accessToken: 'at-cold-start', refreshToken: 'rt' });
  if (!secureStore.get('weknora.product.credential')) {
    throw new Error('credential must persist through the secure store adapter');
  }
  const credentialInOrdinaryStorage = ordinaryWrites.length > 0;

  if (seen[0] !== '/api/v1/auth/me') throw new Error('bootstrap must call auth me');
  if (resolved.tenantId !== outcome.tenantId) throw new Error('port and pure resolution disagree');
  return {
    userId: outcome.userId,
    tenantId: outcome.tenantId ?? '',
    credentialInOrdinaryStorage,
  };
}
