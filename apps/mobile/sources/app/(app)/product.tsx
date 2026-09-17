import * as React from 'react';
import { Redirect } from 'expo-router';
import { useProductAuth } from '@/weknora/auth/session';
import { useMobileHost } from '@/weknora/platform/host';
import { ProductShell } from '@/weknora/navigation/ProductShell';
import { HomeScreen } from '@/weknora/screens/HomeScreen';
import { createOverviewApi, type ClientRequest } from '@weknora/api-client';

/**
 * 产品入口（MX-013，D-023 接线落地）：认证+引导完成后进入四 Tab 产品壳。
 * 深链/直接访问不绕过 Provider——未认证回登录、无 host 回服务器选择（根布局守卫仍在）。
 * 其余 Tab 的真实 screen 由后续任务逐个注入插槽（当前诚实空态）。
 */
export default function ProductEntry() {
  const host = useMobileHost();
  const auth = useProductAuth();

  if (!host) return <Redirect href="/(app)/server" />;
  if (auth.loading) return null;
  if (!auth.credential) return <Redirect href="/(app)/login" />;

  const token = auth.credential.kind === 'bearer' ? auth.credential.accessToken : '';
  const loader = createOverviewApi(async (input: ClientRequest) => {
    const response = await fetch(`${host.origin}${input.path}`, {
      method: input.method,
      headers: {
        'content-type': 'application/json',
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        ...(input.headers ?? {}),
      },
    });
    return (await response.json()) as unknown;
  });
  const identity = auth.scope.identity();

  return (
    <ProductShell
      initialTab="workbench"
      tabs={{
        workbench: (
          <HomeScreen
            loader={loader}
            identity={{ origin: host.origin, userId: identity.userId, tenantId: identity.tenantId }}
            offline={() => false}
            onOpenRun={() => undefined}
            onOpenApprovals={() => undefined}
          />
        ),
      }}
    />
  );
}
