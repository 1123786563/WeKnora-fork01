import { useCallback, useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, Alert, FlatList, Pressable, SafeAreaView, ScrollView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import type { Organization, OrganizationJoinRequest, OrganizationMember, OrganizationRole } from '@weknora/api-client';
import { useMobileRuntime } from '../../runtime.tsx';
import { canManageOrganization, validateOrganizationDraft } from './organizations.ts';

const ORGANIZATION_ROLES: readonly OrganizationRole[] = ['admin', 'editor', 'viewer'];

export function OrganizationsScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const api = runtime.client.identity.organizations;
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [selected, setSelected] = useState<Organization | null>(null);
  const [members, setMembers] = useState<OrganizationMember[]>([]);
  const [requests, setRequests] = useState<OrganizationJoinRequest[]>([]);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const writable = useMemo(() => selected ? canManageOrganization(selected) : false, [selected]);

  const load = useCallback(async () => {
    setLoading(true); setError('');
    try {
      const result = await api.list();
      setOrganizations(result.items);
      if (selected) {
        const next = result.items.find((item) => item.id === selected.id);
        if (next) setSelected(next);
      }
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load organizations'); }
    finally { setLoading(false); }
  }, [api, selected]);

  useEffect(() => { void load(); }, [load]);

  const select = useCallback(async (organization: Organization) => {
    setSelected(organization); setError('');
    const results = await Promise.allSettled([api.members.list(organization.id), api.joinRequests.list(organization.id)]);
    if (results[0].status === 'fulfilled') setMembers(results[0].value.items); else setError(results[0].reason instanceof Error ? results[0].reason.message : 'Unable to load organization members');
    const joinRequestsResult = results[1];
    if (joinRequestsResult.status === 'fulfilled') setRequests(joinRequestsResult.value.items); else { const reason = joinRequestsResult.reason; setError((current) => current || (reason instanceof Error ? reason.message : 'Unable to load join requests')); }
  }, [api]);

  async function create() {
    const validation = validateOrganizationDraft(name, description);
    if (validation.length) { setError(validation.join('. ')); return; }
    setSaving(true); setError('');
    try { const created = await api.create({ name: name.trim(), description }); setName(''); setDescription(''); await load(); await select(created); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to create organization; server state was kept'); }
    finally { setSaving(false); }
  }

  async function updateMemberRole(member: OrganizationMember, role: OrganizationRole) {
    if (!selected || !writable || member.role === role) return;
    setError('');
    try { await api.members.updateRole(selected.id, member.tenant_id, { role }); await select(selected); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to update organization member role'); }
  }

  function removeMember(member: OrganizationMember) {
    if (!selected || !writable) return;
    Alert.alert('Remove organization member?', member.tenant_name || member.username, [
      { text: 'Cancel', style: 'cancel' },
      { text: 'Remove', style: 'destructive', onPress: () => void (async () => {
        setError('');
        try { await api.members.remove(selected.id, member.tenant_id); await select(selected); }
        catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to remove organization member'); }
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
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to review join request'); }
  }

  return <SafeAreaView style={{ flex: 1, padding: 16 }}><View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 22, fontWeight: '700' }}>Organizations</Text><Pressable onPress={() => void load()}><Text style={{ color: '#2864dc' }}>Refresh</Text></Pressable></View>{error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}{loading ? <ActivityIndicator accessibilityLabel="Loading organizations" /> : <ScrollView keyboardShouldPersistTaps="handled"><Text style={{ fontSize: 17, fontWeight: '700', marginBottom: 6 }}>Organizations</Text><FlatList scrollEnabled={false} data={organizations} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085', marginBottom: 12 }}>No organizations returned.</Text>} renderItem={({ item }) => <Pressable onPress={() => void select(item)} style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 10, backgroundColor: selected?.id === item.id ? '#eff6ff' : 'transparent' }}><Text style={{ fontWeight: '600' }}>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{String(item.my_role || 'member')} · owner tenant {item.owner_tenant_id}</Text></Pressable>} /><Text style={{ fontSize: 17, fontWeight: '700', marginTop: 18, marginBottom: 6 }}>Create organization</Text><TextInput accessibilityLabel="Organization name" value={name} onChangeText={setName} placeholder="Name" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel="Organization description" value={description} onChangeText={setDescription} placeholder="Description" multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 70, marginBottom: 8 }} /><Pressable accessibilityRole="button" disabled={saving} onPress={() => void create()} style={{ backgroundColor: '#2864dc', padding: 11, borderRadius: 8, alignItems: 'center', opacity: saving ? 0.5 : 1 }}><Text style={{ color: '#fff', fontWeight: '600' }}>{saving ? 'Creating…' : 'Create organization'}</Text></Pressable>{selected ? <><Text style={{ fontSize: 17, fontWeight: '700', marginTop: 18, marginBottom: 6 }}>{selected.name} members</Text><Text style={{ color: '#667085', marginBottom: 6 }}>{writable ? 'Organization admin controls' : 'Read-only organization membership'}</Text><FlatList scrollEnabled={false} data={members} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085' }}>No members returned.</Text>} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 9 }}><View style={{ flexDirection: 'row', justifyContent: 'space-between' }}><View><Text style={{ fontWeight: '600' }}>{item.tenant_name || item.username}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{item.email} · {item.role}</Text></View>{writable ? <Pressable onPress={() => removeMember(item)}><Text style={{ color: '#b42318' }}>Remove</Text></Pressable> : null}</View>{writable ? <View style={{ flexDirection: 'row', gap: 8, marginTop: 6 }}>{ORGANIZATION_ROLES.map((nextRole) => <Pressable key={nextRole} onPress={() => void updateMemberRole(item, nextRole)} style={{ paddingHorizontal: 8, paddingVertical: 5, borderRadius: 12, backgroundColor: item.role === nextRole ? '#dbeafe' : '#f2f4f7' }}><Text>{nextRole}</Text></Pressable>)}</View> : null}</View>} /><Text style={{ fontSize: 17, fontWeight: '700', marginTop: 18, marginBottom: 6 }}>Pending join requests</Text><FlatList scrollEnabled={false} data={requests.filter((item) => item.status === 'pending')} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085' }}>No pending join requests.</Text>} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 9, flexDirection: 'row', justifyContent: 'space-between' }}><View style={{ flex: 1 }}><Text style={{ fontWeight: '600' }}>{item.username}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{item.requested_role} · {item.message || 'No message'}</Text></View>{writable ? <View style={{ flexDirection: 'row', gap: 8 }}><Pressable onPress={() => void reviewRequest(item, true)}><Text style={{ color: '#067647' }}>Approve</Text></Pressable><Pressable onPress={() => void reviewRequest(item, false)}><Text style={{ color: '#b42318' }}>Decline</Text></Pressable></View> : null}</View>} /></> : null}</ScrollView>}</SafeAreaView>;
}
