import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Alert, FlatList, Pressable, SafeAreaView, ScrollView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import type { AuditLog, TenantInvitation, TenantMember, TenantRole } from '@weknora/api-client';
import { formatMessage } from '@weknora/i18n';
import { useMobileRuntime } from '../../runtime.tsx';
import { administrationAuditActionKey, administrationAuditActorKey, administrationAuditOutcomeKey, canManageTenant, isRemovableMember, validateInvite } from './administration.ts';

const INVITE_ROLES: readonly TenantRole[] = ['admin', 'contributor', 'viewer'];
const MEMBER_ROLES: readonly TenantRole[] = ['admin', 'contributor', 'viewer'];

function tenantNumber(value: string | null): number | null {
  const id = value === null ? NaN : Number(value);
  return Number.isSafeInteger(id) && id > 0 ? id : null;
}

export function AdministrationScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const tenantId = tenantNumber(runtime.tenantId);
  const t = (key: string, values: Record<string, string | number> = {}) => formatMessage(runtime.locale, key, values);
  const roleLabel = (value: string) => ['owner', 'admin', 'contributor', 'viewer'].includes(value) ? t(`mobileAdministration.role.${value}`) : value;
  const statusLabel = (value: string) => ['active', 'pending'].includes(value) ? t(`mobileAdministration.status.${value}`) : value;
  const auditLabel = (kind: 'action' | 'outcome' | 'actor', value: string) => {
    const key = kind === 'action' ? administrationAuditActionKey(value) : kind === 'outcome' ? administrationAuditOutcomeKey(value) : administrationAuditActorKey(value);
    return key ? t(key) : value;
  };
  const role = useMemo(() => runtime.workspaces.find((item) => String(item.id) === runtime.tenantId)?.role, [runtime.tenantId, runtime.workspaces]);
  const writable = canManageTenant(role);
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [invitations, setInvitations] = useState<TenantInvitation[]>([]);
  const [audit, setAudit] = useState<AuditLog[]>([]);
  const [email, setEmail] = useState('');
  const [inviteRole, setInviteRole] = useState<TenantRole>('viewer');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const loadGeneration = useRef(0);

  const load = useCallback(async () => {
    if (tenantId === null) { setLoading(false); setError(t('mobileAdministration.noWorkspace')); return; }
    const generation = ++loadGeneration.current;
    setLoading(true); setError('');
    const results = await Promise.allSettled([
      runtime.client.identity.tenants.members.list(tenantId, { page: 1, pageSize: 100 }),
      runtime.client.identity.tenants.invitations.listTenant(tenantId, { page: 1, pageSize: 100 }),
      runtime.client.identity.tenants.auditLog.list(tenantId, { limit: 50 }),
    ]);
    if (generation !== loadGeneration.current) return;
    const failures: unknown[] = [];
    if (results[0].status === 'fulfilled') setMembers(results[0].value.items); else failures.push(results[0].reason);
    if (results[1].status === 'fulfilled') setInvitations(results[1].value.items); else failures.push(results[1].reason);
    if (results[2].status === 'fulfilled') setAudit(results[2].value.items); else failures.push(results[2].reason);
    if (failures.length) setError(failures[0] instanceof Error ? failures[0].message : t('mobileAdministration.someDataUnavailable'));
    if (generation === loadGeneration.current) setLoading(false);
  }, [runtime.client, runtime.locale, tenantId]);

  useEffect(() => { void load(); }, [load]);

  async function invite() {
    if (tenantId === null || saving) return;
    const validation = validateInvite(email, inviteRole);
    if (validation.length) { setError(validation.map((message) => message === 'Email is required' ? t('mobileAdministration.emailRequired') : message === 'Owner invitations are not allowed' ? t('mobileAdministration.ownerInviteForbidden') : message).join('. ')); return; }
    setSaving(true); setError('');
    try { await runtime.client.identity.tenants.invitations.create(tenantId, { email: email.trim(), role: inviteRole }); setEmail(''); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : t('mobileAdministration.unableToSend')); }
    finally { setSaving(false); }
  }

  async function updateRole(member: TenantMember, nextRole: TenantRole) {
    if (tenantId === null || !writable || member.role === 'owner' || member.role === nextRole) return;
    setError('');
    try { await runtime.client.identity.tenants.members.updateRole(tenantId, member.user_id, nextRole); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : t('mobileAdministration.unableToUpdateRole')); }
  }

  function remove(member: TenantMember) {
    if (tenantId === null || !writable || !isRemovableMember(member)) return;
    Alert.alert(t('mobileAdministration.removeTitle'), t('mobileAdministration.removeMessage', { name: member.username }), [
      { text: t('mobileAdministration.cancel'), style: 'cancel' },
      { text: t('mobileAdministration.remove'), style: 'destructive', onPress: () => void (async () => {
        setError('');
        try { await runtime.client.identity.tenants.members.remove(tenantId, member.user_id); await load(); }
        catch (cause) { setError(cause instanceof Error ? cause.message : t('mobileAdministration.unableToRemove')); }
      })() },
    ]);
  }

  async function revoke(invitation: TenantInvitation) {
    if (tenantId === null || !writable) return;
    setError('');
    try { await runtime.client.identity.tenants.invitations.revoke(tenantId, invitation.id); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : t('mobileAdministration.unableToRevoke')); }
  }

  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{t('mobileAdministration.back')}</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 22, fontWeight: '700' }}>{t('mobileAdministration.title')}</Text><Pressable onPress={() => void load()}><Text style={{ color: '#2864dc' }}>{t('mobileAdministration.refresh')}</Text></Pressable></View>
    <Text style={{ color: '#667085', marginBottom: 10 }}>{role ? t('mobileAdministration.role', { role: roleLabel(role) }) : t('mobileAdministration.roleUnavailable')} · {t('mobileAdministration.tenant', { tenant: runtime.tenantId || 'unknown' })}</Text>
    <Pressable accessibilityRole="button" onPress={() => router.push('/management/api-keys')} style={{ borderColor: '#2864dc', borderWidth: 1, padding: 10, borderRadius: 8, alignItems: 'center', marginBottom: 10 }}><Text style={{ color: '#2864dc', fontWeight: '600' }}>{t('mobileAdministration.manageApiKeys')}</Text></Pressable>
    {!writable ? <Text style={{ color: '#667085', marginBottom: 8 }}>{t('mobileAdministration.readOnly')}</Text> : null}
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel={t('mobileAdministration.loading')} /> : <ScrollView keyboardShouldPersistTaps="handled">
      <Text style={{ fontSize: 17, fontWeight: '700', marginBottom: 6 }}>{t('mobileAdministration.members', { count: members.length })}</Text>
      <FlatList scrollEnabled={false} data={members} keyExtractor={(item) => item.user_id} ListEmptyComponent={<Text style={{ color: '#667085', marginBottom: 12 }}>{t('mobileAdministration.noMembers')}</Text>} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 10 }}><View style={{ flexDirection: 'row', justifyContent: 'space-between' }}><View style={{ flex: 1 }}><Text style={{ fontWeight: '600' }}>{item.username}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{item.email} · {roleLabel(item.role)} · {statusLabel(item.status)}</Text></View>{writable && isRemovableMember(item) ? <Pressable onPress={() => remove(item)}><Text style={{ color: '#b42318' }}>{t('mobileAdministration.remove')}</Text></Pressable> : null}</View>{writable && item.role !== 'owner' ? <View style={{ flexDirection: 'row', gap: 8, marginTop: 7 }}>{MEMBER_ROLES.map((nextRole) => <Pressable key={nextRole} onPress={() => void updateRole(item, nextRole)} style={{ paddingHorizontal: 8, paddingVertical: 5, borderRadius: 12, backgroundColor: item.role === nextRole ? '#dbeafe' : '#f2f4f7' }}><Text>{roleLabel(nextRole)}</Text></Pressable>)}</View> : null}</View>} />
      {writable ? <><Text style={{ fontSize: 17, fontWeight: '700', marginTop: 18, marginBottom: 6 }}>{t('mobileAdministration.invite')}</Text><TextInput accessibilityLabel={t('mobileAdministration.inviteEmail')} keyboardType="email-address" autoCapitalize="none" value={email} onChangeText={setEmail} placeholder="person@example.com" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><View style={{ flexDirection: 'row', gap: 8, marginBottom: 8 }}>{INVITE_ROLES.map((nextRole) => <Pressable key={nextRole} onPress={() => setInviteRole(nextRole)} style={{ paddingHorizontal: 9, paddingVertical: 6, borderRadius: 12, backgroundColor: inviteRole === nextRole ? '#dcfce7' : '#f2f4f7' }}><Text>{roleLabel(nextRole)}</Text></Pressable>)}</View><Pressable accessibilityRole="button" disabled={saving} onPress={() => void invite()} style={{ backgroundColor: '#2864dc', padding: 11, borderRadius: 8, alignItems: 'center', opacity: saving ? 0.5 : 1 }}><Text style={{ color: '#fff', fontWeight: '600' }}>{saving ? t('mobileAdministration.sending') : t('mobileAdministration.sendInvitation')}</Text></Pressable></> : null}
      <Text style={{ fontSize: 17, fontWeight: '700', marginTop: 18, marginBottom: 6 }}>{t('mobileAdministration.openInvitations')}</Text><FlatList scrollEnabled={false} data={invitations.filter((item) => item.status === 'pending')} keyExtractor={(item) => String(item.id)} ListEmptyComponent={<Text style={{ color: '#667085' }}>{t('mobileAdministration.noPendingInvitations')}</Text>} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 9, flexDirection: 'row', justifyContent: 'space-between' }}><View><Text>{item.invitee_email || item.invitee_user_id}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{roleLabel(item.role)} · {t('mobileAdministration.expires', { date: item.expires_at })}</Text></View>{writable ? <Pressable onPress={() => void revoke(item)}><Text style={{ color: '#b42318' }}>{t('mobileAdministration.revoke')}</Text></Pressable> : null}</View>} />
      <Text style={{ fontSize: 17, fontWeight: '700', marginTop: 18, marginBottom: 6 }}>{t('mobileAdministration.auditLog')}</Text><FlatList scrollEnabled={false} data={audit} keyExtractor={(item) => String(item.id)} ListEmptyComponent={<Text style={{ color: '#667085' }}>{t('mobileAdministration.noAuditEntries')}</Text>} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 9 }}><Text style={{ fontWeight: '600' }}>{auditLabel('action', item.action)} · {auditLabel('outcome', item.outcome)}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{auditLabel('actor', item.actor_role)} · {item.created_at}</Text></View>} />
    </ScrollView>}
  </SafeAreaView>;
}
