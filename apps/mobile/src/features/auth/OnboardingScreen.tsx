import { useCallback, useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, SafeAreaView, ScrollView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import type { TenantInvitation } from '@weknora/api-client';
import { useMobileRuntime } from '../../runtime.tsx';

export function OnboardingScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [invitations, setInvitations] = useState<TenantInvitation[] | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [showInvitations, setShowInvitations] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [error, setError] = useState('');
  const [invitationError, setInvitationError] = useState('');
  const [saving, setSaving] = useState(false);
  const [respondingId, setRespondingId] = useState<number | null>(null);
  const invitationGeneration = useRef(0);

  const load = useCallback(async () => {
    setLoading(true); setFailed(false); setError('');
    try { await runtime.refreshWorkspaces(); } catch { setFailed(true); }
    finally { setLoading(false); }
  }, [runtime.refreshWorkspaces]);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => { if (runtime.tenantId) router.replace('/(app)/knowledge'); }, [router, runtime.tenantId]);

  async function loadInvitations() {
    const generation = ++invitationGeneration.current;
    setShowInvitations(true); setInvitations(null); setInvitationError(''); setError('');
    try {
      const page = await runtime.client.identity.tenants.invitations.listMine();
      if (generation === invitationGeneration.current) setInvitations(page.items.filter((item) => item.status === 'pending'));
    } catch (cause) {
      if (generation === invitationGeneration.current) { setInvitations(null); setInvitationError(cause instanceof Error ? cause.message : 'Unable to load invitations'); }
    }
  }
  async function createWorkspace() {
    const trimmed = name.trim();
    if (!trimmed) { setError('Workspace name is required'); return; }
    if (trimmed.length > 128 || description.length > 512) { setError('Workspace name or description is too long'); return; }
    setSaving(true); setError('');
    try { await runtime.client.identity.tenants.admin.create({ name: trimmed, description: description || undefined }); await runtime.refreshWorkspaces(); setShowCreate(false); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Workspace creation failed'); }
    finally { setSaving(false); }
  }
  async function respond(invitation: TenantInvitation, accept: boolean) {
    setRespondingId(invitation.id); setError('');
    try {
      if (accept) await runtime.client.identity.tenants.invitations.accept(invitation.id);
      else await runtime.client.identity.tenants.invitations.decline(invitation.id);
      setInvitations((current) => current?.filter((item) => item.id !== invitation.id) ?? []);
      if (accept) await runtime.refreshWorkspaces();
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Invitation action failed'); }
    finally { setRespondingId(null); }
  }
  return <SafeAreaView style={{ flex: 1, backgroundColor: '#f8fafc' }}><ScrollView contentContainerStyle={{ flexGrow: 1, justifyContent: 'center', padding: 20 }}>
    <View style={{ borderColor: '#eaecf0', borderRadius: 20, borderWidth: 1, backgroundColor: '#fff', padding: 24, gap: 14 }}>
      <Text accessibilityRole="header" style={{ textAlign: 'center', fontSize: 26, fontWeight: '700' }}>{runtime.canCreateTenant ? 'Create your workspace' : 'Join a workspace'}</Text>
      <Text style={{ color: '#667085', lineHeight: 22, textAlign: 'center' }}>{runtime.canCreateTenant ? 'Create a workspace to get started.' : 'You need an invitation to join a workspace.'}</Text>
      {loading ? <View accessibilityLabel="Loading workspace policy" style={{ alignItems: 'center', gap: 8 }}><ActivityIndicator /><Text>Loading…</Text></View> : failed ? <View accessibilityRole="alert" style={{ gap: 8 }}><Text style={{ color: '#b42318' }}>Unable to load workspace policy.</Text><Pressable onPress={() => void load()}><Text style={{ color: '#2864dc', textAlign: 'center' }}>Retry</Text></Pressable></View> : <>
        {!runtime.canCreateTenant ? <Text style={{ backgroundColor: '#f2f4f7', borderRadius: 10, padding: 12, textAlign: 'center' }}>Only invited users can join a workspace.</Text> : null}
        {runtime.canCreateTenant ? <Pressable testID="create-open" onPress={() => setShowCreate(true)} style={{ backgroundColor: '#2864dc', borderRadius: 10, padding: 14 }}><Text style={{ color: '#fff', fontWeight: '700', textAlign: 'center' }}>Create workspace</Text></Pressable> : null}
        <Pressable testID="invitations-open" onPress={() => void loadInvitations()} style={{ borderColor: '#2864dc', borderRadius: 10, borderWidth: 1, padding: 14 }}><Text style={{ color: '#2864dc', fontWeight: '700', textAlign: 'center' }}>Invitations</Text></Pressable>
      </>}
      {error ? <Text accessibilityRole="alert" style={{ color: '#b42318' }}>{error}</Text> : null}
      {showCreate ? <View style={{ borderTopColor: '#eaecf0', borderTopWidth: 1, gap: 10, paddingTop: 14 }}><Text style={{ fontSize: 18, fontWeight: '700' }}>Create workspace</Text><TextInput accessibilityLabel="Workspace name" value={name} onChangeText={setName} placeholder="Workspace name" maxLength={128} editable={!saving} style={inputStyle} /><TextInput accessibilityLabel="Workspace description" value={description} onChangeText={setDescription} placeholder="Description" maxLength={512} editable={!saving} style={inputStyle} /><Pressable disabled={saving} onPress={() => void createWorkspace()} style={{ backgroundColor: '#2864dc', borderRadius: 10, opacity: saving ? 0.5 : 1, padding: 12 }}><Text style={{ color: '#fff', textAlign: 'center' }}>{saving ? 'Creating…' : 'Create'}</Text></Pressable></View> : null}
      {showInvitations ? <View style={{ borderTopColor: '#eaecf0', borderTopWidth: 1, gap: 8, paddingTop: 14 }}><Text style={{ fontSize: 18, fontWeight: '700' }}>Invitations</Text>{invitationError ? <Text accessibilityRole="alert" style={{ color: '#b42318' }}>{invitationError}</Text> : invitations === null ? <ActivityIndicator accessibilityLabel="Loading invitations" /> : invitations.length === 0 ? <Text style={{ color: '#667085' }}>没有待处理</Text> : invitations.map((invitation) => <View key={invitation.id}><Text>{invitation.tenant_name || `Workspace ${invitation.tenant_id}`} · {invitation.role}</Text><View style={{ flexDirection: 'row', gap: 8 }}><Pressable disabled={respondingId !== null} testID={`accept-${invitation.id}`} onPress={() => void respond(invitation, true)}><Text style={{ color: '#067647' }}>Accept</Text></Pressable><Pressable disabled={respondingId !== null} testID={`decline-${invitation.id}`} onPress={() => void respond(invitation, false)}><Text style={{ color: '#b42318' }}>Decline</Text></Pressable></View></View>)}</View> : null}
      <Pressable testID="logout" onPress={() => void runtime.logout().then(() => router.replace('/(auth)/login')).catch((cause) => setError(cause instanceof Error ? cause.message : 'Unable to log out'))}><Text style={{ color: '#667085', textAlign: 'center' }}>Log out</Text></Pressable>
    </View>
  </ScrollView></SafeAreaView>;
}

const inputStyle = { borderColor: '#d0d5dd', borderRadius: 10, borderWidth: 1, padding: 12 } as const;
