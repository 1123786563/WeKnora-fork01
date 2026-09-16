import * as React from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { Text, View } from 'react-native';
import { useRoute } from '@react-navigation/native';
import { ConversationScreen } from '@/weknora/conversations/ConversationScreen';
import { createProductConversationViewModel, createProductExecutionApi } from '@/weknora/conversations/view-model';
import { useProductAuth } from '@/weknora/auth/session';
import { useMobileHost } from '@/weknora/platform/host';
import { createPersistentExecutionRequestStorage } from '@/weknora/platform/execution-storage';

function ProductSessionRoute() {
  const route = useRoute();
  const params = route.params as { id?: string; spaceId?: string; agentId?: string; targetId?: string; workspaceRef?: string; resourceUserId?: string; resourceTenantId?: string } | undefined;
  const sessionId = params?.id ?? '';
  const host = useMobileHost();
  const auth = useProductAuth();
  const executionApi = React.useMemo(() => (
    host && auth.credential?.kind === 'bearer'
      ? createProductExecutionApi({ origin: host.origin, credential: auth.credential, scope: auth.scope })
      : null
  ), [auth.credential, auth.scope, host]);
  const identity = auth.scope.identity();
  const resourcesReady = Boolean(
    params?.spaceId && params.agentId && params.targetId && params.workspaceRef
    && params.resourceUserId === identity.userId
    && params.resourceTenantId === identity.tenantId,
  );
  const requestStorage = React.useMemo(() => {
    if (!identity.userId || !identity.tenantId || !host) return null;
    return createPersistentExecutionRequestStorage(AsyncStorage, `weknora:execution-request:${host.origin}:${identity.tenantId}:${identity.userId}:${sessionId}`);
  }, [host, identity.tenantId, identity.userId, sessionId]);
  const viewModel = React.useMemo(() => {
    const selected = params;
    if (!executionApi || !sessionId || !resourcesReady || !requestStorage || !selected) return null;
    return createProductConversationViewModel({
      scope: auth.scope,
      spaceId: selected.spaceId!,
      sessionId,
      agent: { id: selected.agentId!, name: selected.agentId! },
      targetId: selected.targetId!,
      workspaceRef: selected.workspaceRef!,
      budgetUpper: 0,
      executions: executionApi,
      requestStorage,
    });
  }, [auth.scope, executionApi, params?.agentId, params?.spaceId, params?.targetId, params?.workspaceRef, requestStorage, resourcesReady, sessionId]);
  if (!viewModel) return <View><Text accessibilityRole="alert">产品会话尚未准备好</Text></View>;
  return <ConversationScreen sessionId={sessionId} viewModel={viewModel} />;
}

export default React.memo(ProductSessionRoute);
