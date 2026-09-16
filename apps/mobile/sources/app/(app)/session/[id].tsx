import * as React from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { Text, View } from 'react-native';
import { useRoute } from '@react-navigation/native';
import { SessionView } from '@/-session/SessionView';
import { ConversationScreen } from '@/weknora/conversations/ConversationScreen';
import { createProductConversationViewModel, createProductExecutionApi } from '@/weknora/conversations/view-model';
import { resolveProductSessionResources, type ProductSessionResourceSelection, type VerifiedProductSessionResources } from '@/weknora/conversations/resources';
import { useProductAuth } from '@/weknora/auth/session';
import { useMobileHost } from '@/weknora/platform/host';
import { createPersistentExecutionRequestStorage } from '@/weknora/platform/execution-storage';
import { projectExecutionSnapshot } from '@/weknora/conversations/execution-projection';

type Params = ProductSessionResourceSelection & { id?: string; resourceUserId?: string; resourceTenantId?: string };

function ProductSessionRoute() {
  const route = useRoute();
  const params = route.params as Partial<Params> | undefined;
  const sessionId = params?.id ?? '';
  const host = useMobileHost();
  const auth = useProductAuth();
  const identity = auth.scope.identity();
  const selection = params?.spaceId && params.agentId && params.targetId && params.workspaceRef
    ? { spaceId: params.spaceId, agentId: params.agentId, targetId: params.targetId, workspaceRef: params.workspaceRef }
    : null;
  const productRoute = selection !== null;
  const [resources, setResources] = React.useState<VerifiedProductSessionResources | null>(null);
  const [resourceError, setResourceError] = React.useState<string | null>(null);
  React.useEffect(() => {
    if (!selection || !auth.authSession || !identity.userId || !identity.tenantId) return;
    const controller = new AbortController();
    setResources(null);
    setResourceError(null);
    void resolveProductSessionResources(auth.authSession, selection, identity, controller.signal)
      .then(setResources)
      .catch((error) => { if (!controller.signal.aborted) setResourceError(error instanceof Error ? error.message : 'PRODUCT_RESOURCES_UNAVAILABLE'); });
    return () => controller.abort();
  }, [auth.authSession, identity.tenantId, identity.userId, selection?.agentId, selection?.spaceId, selection?.targetId, selection?.workspaceRef]);
  const executionApi = React.useMemo(() => (
    host && auth.credential?.kind === 'bearer' && auth.authSession
      ? createProductExecutionApi({ origin: host.origin, credential: auth.credential, scope: auth.scope, authSession: auth.authSession })
      : null
  ), [auth.authSession, auth.credential, auth.scope, host]);
  const requestStorage = React.useMemo(() => {
    if (!identity.userId || !identity.tenantId || !host) return null;
    return createPersistentExecutionRequestStorage(AsyncStorage, `weknora:execution-request:${host.origin}:${identity.tenantId}:${identity.userId}:${sessionId}`);
  }, [host, identity.tenantId, identity.userId, sessionId]);
  const viewModel = React.useMemo(() => {
    if (!resources || !executionApi || !sessionId || !requestStorage) return null;
    return createProductConversationViewModel({
      scope: auth.scope,
      spaceId: resources.spaceId,
      sessionId,
      agent: { id: resources.agentId, name: resources.agentName },
      targetId: resources.targetId,
      workspaceRef: resources.workspaceRef,
      budgetUpper: 0,
      executions: executionApi,
      requestStorage,
      projection: executionApi.snapshot ? {
        load: async (runID, signal) => projectExecutionSnapshot(await executionApi.snapshot!(runID, signal)),
      } : undefined,
    });
  }, [auth.scope, executionApi, requestStorage, resources, sessionId]);
  // Sessions without explicit product resource metadata remain the retained Happy route.
  // Product sessions never receive defaults: they wait for all server ownership checks.
  if (!productRoute) return <SessionView id={sessionId} />;
  if (resourceError) return <View><Text accessibilityRole="alert">产品资源不可用：{resourceError}</Text></View>;
  if (!viewModel) return <View><Text accessibilityRole="alert">正在验证产品会话资源…</Text></View>;
  return <ConversationScreen sessionId={sessionId} viewModel={viewModel} />;
}

export default React.memo(ProductSessionRoute);
