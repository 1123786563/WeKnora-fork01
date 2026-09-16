import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Alert, FlatList, Pressable, SafeAreaView, ScrollView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import type { Organization, OrganizationJoinRequest, OrganizationMember, OrganizationRole, OrganizationShare } from '@weknora/api-client';
import { formatMessage } from '@weknora/i18n';
import { useMobileRuntime } from '../../runtime.tsx';
import { canCreateOrganization, canManageOrganization, shareResourceId, shareResourceLabel, validateOrganizationDraft } from './organizations.ts';

const ORGANIZATION_ROLES: readonly OrganizationRole[] = ['admin', 'editor', 'viewer'];

export function OrganizationsScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const api = runtime.client.identity.organizations;
  const t = (key: string, values: Record<string, string | number> = {}) => formatMessage(runtime.locale, key, values);
  const roleLabel = (role: unknown) => role === 'admin' || role === 'editor' || role === 'viewer' ? t(`organization.role.${role}`) : String(role || t('organization.role.viewer'));
  const permissionLabel = (permission: unknown) => permission === 'editor' ? t('organization.share.permissionEditable') : t('organization.share.permissionReadonly');
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [selected, setSelected] = useState<Organization | null>(null);
  const [members, setMembers] = useState<OrganizationMember[]>([]);
  const [requests, setRequests] = useState<OrganizationJoinRequest[]>([]);
  const [knowledgeBaseShares, setKnowledgeBaseShares] = useState<OrganizationShare[]>([]);
  const [agentShares, setAgentShares] = useState<OrganizationShare[]>([]);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const loadGeneration = useRef(0);
  const selectionGeneration = useRef(0);

  const writable = useMemo(() => selected ? canManageOrganization(selected) : false, [selected]);
  const canCreate = useMemo(() => canCreateOrganization(runtime.workspaces.find((workspace) => String(workspace.id) === runtime.tenantId)?.role), [runtime.tenantId, runtime.workspaces]);

  const load = useCallback(async () => {
    const generation = ++loadGeneration.current;
    setLoading(true); setError('');
    try {
      const deployment = await runtime.client.administration.capabilities();
      if (generation !== loadGeneration.current) return;
      const capability = deployment.capabilities.organizations;
      if (capability && !capability.supported) {
        setOrganizations([]); setSelected(null); setMembers([]); setRequests([]); setKnowledgeBaseShares([]); setAgentShares([]);
        setError(capability.reason || t('common.error'));
        return;
      }
      const result = await api.list();
      if (generation !== loadGeneration.current) return;
      setOrganizations(result.items);
      setSelected((current) => current ? result.items.find((item) => item.id === current.id) || null : null);
    } catch (cause) {
      if (generation === loadGeneration.current) setError(cause instanceof Error ? cause.message : t('common.error'));
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }, [api, runtime.client, runtime.locale]);

  useEffect(() => { void load(); }, [load]);

  const select = useCallback(async (organization: Organization) => {
    const generation = ++selectionGeneration.current;
    setSelected(organization); setError('');
    setMembers([]); setRequests([]); setKnowledgeBaseShares([]); setAgentShares([]);
    const results = await Promise.allSettled([
      api.members.list(organization.id),
      api.joinRequests.list(organization.id),
      api.knowledgeBaseShares.listForOrganization(organization.id),
      api.agentShares.listForOrganization(organization.id),
    ]);
    if (generation !== selectionGeneration.current) return;
    if (results[0].status === 'fulfilled') setMembers(results[0].value.items); else setError(results[0].reason instanceof Error ? results[0].reason.message : t('common.error'));
    const joinRequestsResult = results[1];
    if (joinRequestsResult.status === 'fulfilled') setRequests(joinRequestsResult.value.items); else { const reason = joinRequestsResult.reason; setError((current) => current || (reason instanceof Error ? reason.message : t('common.error'))); }
    const knowledgeBaseSharesResult = results[2];
    if (knowledgeBaseSharesResult.status === 'fulfilled') setKnowledgeBaseShares(knowledgeBaseSharesResult.value.items); else { const reason = knowledgeBaseSharesResult.reason; setError((current) => current || (reason instanceof Error ? reason.message : t('common.error'))); }
    const agentSharesResult = results[3];
    if (agentSharesResult.status === 'fulfilled') setAgentShares(agentSharesResult.value.items); else { const reason = agentSharesResult.reason; setError((current) => current || (reason instanceof Error ? reason.message : t('common.error'))); }
  }, [api, runtime.locale]);

  async function create() {
    if (!canCreate) { setError(t('organization.rbac.cannotCreate')); return; }
    const validation = validateOrganizationDraft(name, description);
    if (validation.length) { setError(validation.map((message) => message === 'Name is required' ? t('organization.nameRequired') : message).join('. ')); return; }
    setSaving(true); setError('');
    try { const created = await api.create({ name: name.trim(), description }); setName(''); setDescription(''); await load(); await select(created); }
    catch (cause) { setError(cause instanceof Error ? cause.message : t('organization.createFailed')); }
    finally { setSaving(false); }
  }

  async function updateMemberRole(member: OrganizationMember, role: OrganizationRole) {
    if (!selected || !writable || member.role === role) return;
    setError('');
    try { await api.members.updateRole(selected.id, member.tenant_id, { role }); await select(selected); }
    catch (cause) { setError(cause instanceof Error ? cause.message : t('organization.roleUpdateFailed')); }
  }

  function removeMember(member: OrganizationMember) {
    if (!selected || !writable) return;
    Alert.alert(t('organization.detail.removeMemberConfirm', { name: member.tenant_name || member.username }), member.tenant_name || member.username, [
      { text: t('common.cancel'), style: 'cancel' },
      { text: t('organization.detail.removeMember'), style: 'destructive', onPress: () => void (async () => {
        setError('');
        try { await api.members.remove(selected.id, member.tenant_id); await select(selected); }
        catch (cause) { setError(cause instanceof Error ? cause.message : t('organization.memberRemoveFailed')); }
      })() },
    ]);
  }

  async function reviewRequest(request: OrganizationJoinRequest, approved: boolean) {
    if (!selected || !writable) return;
    setError('');
    try {
      const requested = ORGANIZATION_ROLES.includes(request.requested_role as OrganizationRole) ? request.requested_role as OrganizationRole : 'viewer';
      await api.joinRequests.review(selected.id, request.id, { approved, ...(approved ? { role: requested } : {}) });
      await select(selected);
    } catch (cause) { setError(cause instanceof Error ? cause.message : t('organization.settings.reviewFailed')); }
  }

  function removeKnowledgeBaseShare(share: OrganizationShare) {
    if (!selected || !writable) return;
    const knowledgeBaseId = shareResourceId(share, 'knowledge-base');
    if (!knowledgeBaseId) { setError(t('common.error')); return; }
    Alert.alert(t('organization.settings.removeShareConfirm', { name: shareResourceLabel(share, 'knowledge-base', t) }), shareResourceLabel(share, 'knowledge-base', t), [
      { text: t('common.cancel'), style: 'cancel' },
      { text: t('organization.settings.removeShareFromOrg'), style: 'destructive', onPress: () => void (async () => {
        setError('');
        try { await api.knowledgeBaseShares.remove(knowledgeBaseId, share.id); await select(selected); }
        catch (cause) { setError(cause instanceof Error ? cause.message : t('organization.settings.removeShareFailed')); }
      })() },
    ]);
  }

  function removeAgentShare(share: OrganizationShare) {
    if (!selected || !writable) return;
    const agentId = shareResourceId(share, 'agent');
    if (!agentId) { setError(t('common.error')); return; }
    Alert.alert(t('organization.settings.removeAgentShareConfirm', { name: shareResourceLabel(share, 'agent', t) }), shareResourceLabel(share, 'agent', t), [
      { text: t('common.cancel'), style: 'cancel' },
      { text: t('organization.settings.removeShareFromOrg'), style: 'destructive', onPress: () => void (async () => {
        setError('');
        try { await api.agentShares.remove(agentId, share.id); await select(selected); }
        catch (cause) { setError(cause instanceof Error ? cause.message : t('organization.settings.removeShareFailed')); }
      })() },
    ]);
  }

  return <SafeAreaView style={{ flex: 1, padding: 16 }}><View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{t('mobileManagement.back')}</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 22, fontWeight: '700' }}>{t('organization.title')}</Text><Pressable onPress={() => void load()}><Text style={{ color: '#2864dc' }}>{t('mobileApiKeys.refresh')}</Text></Pressable></View>{error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}{loading ? <ActivityIndicator accessibilityLabel={t('organization.sharedResources.loading')} /> : <ScrollView keyboardShouldPersistTaps="handled"><Text style={{ fontSize: 17, fontWeight: '700', marginBottom: 6 }}>{t('organization.title')}</Text><FlatList scrollEnabled={false} data={organizations} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085', marginBottom: 12 }}>{t('organization.empty')}</Text>} renderItem={({ item }) => <Pressable onPress={() => void select(item)} style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 10, backgroundColor: selected?.id === item.id ? '#eff6ff' : 'transparent' }}><Text style={{ fontWeight: '600' }}>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{roleLabel(item.my_role || 'viewer')} · {t('organization.owner')}: {item.owner_tenant_id}</Text></Pressable>} />{canCreate ? <><Text style={{ fontSize: 17, fontWeight: '700', marginTop: 18, marginBottom: 6 }}>{t('organization.createOrg')}</Text><TextInput accessibilityLabel={t('organization.name')} value={name} onChangeText={setName} placeholder={t('organization.namePlaceholder')} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel={t('organization.description')} value={description} onChangeText={setDescription} placeholder={t('organization.descriptionPlaceholder')} multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 70, marginBottom: 8 }} /><Pressable accessibilityRole="button" disabled={saving} onPress={() => void create()} style={{ backgroundColor: '#2864dc', padding: 11, borderRadius: 8, alignItems: 'center', opacity: saving ? 0.5 : 1 }}><Text style={{ color: '#fff', fontWeight: '600' }}>{saving ? t('common.loading') : t('organization.createOrg')}</Text></Pressable></> : <Text style={{ color: '#667085', marginTop: 18 }}>{t('organization.rbac.cannotCreate')}</Text>}{selected ? <><Text style={{ fontSize: 17, fontWeight: '700', marginTop: 18, marginBottom: 6 }}>{selected.name} {t('organization.members.listTitle')}</Text><Text style={{ color: '#667085', marginBottom: 6 }}>{writable ? t('organization.settings.membersDesc') : t('organization.settings.permissionsIconHint')}</Text><FlatList scrollEnabled={false} data={members} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085' }}>{t('organization.noMembers')}</Text>} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 9 }}><View style={{ flexDirection: 'row', justifyContent: 'space-between' }}><View><Text style={{ fontWeight: '600' }}>{item.tenant_name || item.username}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{item.email} · {roleLabel(item.role)}</Text></View>{writable ? <Pressable onPress={() => removeMember(item)}><Text style={{ color: '#b42318' }}>{t('organization.detail.removeMember')}</Text></Pressable> : null}</View>{writable ? <View style={{ flexDirection: 'row', gap: 8, marginTop: 6 }}>{ORGANIZATION_ROLES.map((nextRole) => <Pressable key={nextRole} onPress={() => void updateMemberRole(item, nextRole)} style={{ paddingHorizontal: 8, paddingVertical: 5, borderRadius: 12, backgroundColor: item.role === nextRole ? '#dbeafe' : '#f2f4f7' }}><Text>{t(`organization.role.${nextRole}`)}</Text></Pressable>)}</View> : null}</View>} /><Text style={{ fontSize: 17, fontWeight: '700', marginTop: 18, marginBottom: 6 }}>{t('organization.sharedResources.kbListTitle')}</Text><FlatList scrollEnabled={false} data={knowledgeBaseShares} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085' }}>{t('organization.settings.noSharedKB')}</Text>} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 9, flexDirection: 'row', justifyContent: 'space-between' }}><View style={{ flex: 1 }}><Text style={{ fontWeight: '600' }}>{shareResourceLabel(item, 'knowledge-base', t)}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{permissionLabel(item.permission)} · {String(item.shared_by_username || t('organization.share.sharedFrom'))}</Text></View>{writable ? <Pressable onPress={() => removeKnowledgeBaseShare(item)}><Text style={{ color: '#b42318' }}>{t('organization.settings.removeShareFromOrg')}</Text></Pressable> : null}</View>} /><Text style={{ fontSize: 17, fontWeight: '700', marginTop: 18, marginBottom: 6 }}>{t('organization.sharedResources.agentListTitle')}</Text><FlatList scrollEnabled={false} data={agentShares} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085' }}>{t('organization.settings.noSharedAgents')}</Text>} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 9, flexDirection: 'row', justifyContent: 'space-between' }}><View style={{ flex: 1 }}><Text style={{ fontWeight: '600' }}>{shareResourceLabel(item, 'agent', t)}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{permissionLabel(item.permission)} · {String(item.shared_by_username || t('organization.share.sharedFrom'))}</Text></View>{writable ? <Pressable onPress={() => removeAgentShare(item)}><Text style={{ color: '#b42318' }}>{t('organization.settings.removeShareFromOrg')}</Text></Pressable> : null}</View>} /><Text style={{ fontSize: 17, fontWeight: '700', marginTop: 18, marginBottom: 6 }}>{t('organization.joinRequests.listTitle')}</Text><FlatList scrollEnabled={false} data={requests.filter((item) => item.status === 'pending')} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085' }}>{t('organization.settings.noPendingRequests')}</Text>} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 9, flexDirection: 'row', justifyContent: 'space-between' }}><View style={{ flex: 1 }}><Text style={{ fontWeight: '600' }}>{item.username}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{roleLabel(item.requested_role)} · {item.message || t('common.empty')}</Text></View>{writable ? <View style={{ flexDirection: 'row', gap: 8 }}><Pressable onPress={() => void reviewRequest(item, true)}><Text style={{ color: '#067647' }}>{t('organization.settings.approve')}</Text></Pressable><Pressable onPress={() => void reviewRequest(item, false)}><Text style={{ color: '#b42318' }}>{t('organization.settings.reject')}</Text></Pressable></View> : null}</View>} /></> : null}</ScrollView>}</SafeAreaView>;
}
