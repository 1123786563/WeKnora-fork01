// Shared-space list page ported from the Vue baseline
// frontend/src/views/organization/OrganizationList.vue (+ ListSpaceSidebar.vue,
// SpaceAvatar.vue and the invite-preview modal). Layout, section chips, card
// anatomy, empty state and the create / join flows mirror the Vue page while
// all server wiring keeps using @weknora/api-client identity.organizations.
import { useEffect, useMemo, useRef, useState } from 'react';
import { readReactPlatformState } from '../platform/legacy-session.ts';
import type { Organization, OrganizationJoinRequest, OrganizationMember, WeKnoraClient } from '@weknora/api-client';
import { formatMessage, isLocale, supportedLocales } from '@weknora/i18n';
import { clampApplicationNote, inviteJoinMode, requestedRoleOf } from './join.ts';
import { buildInviteLink, copyText, sharedResourceRow } from './settings-actions.ts';
import { organizationRoleLabel } from './summary.ts';
import './organizations.css';
import emptyIllustration from './empty-organizations.svg';

function currentLocale(): string {
  const candidate = navigator.language;
  if (isLocale(candidate)) return candidate;
  const base = candidate.split('-')[0];
  return supportedLocales.find((locale) => locale.split('-')[0] === base) ?? 'en-US';
}
function t(locale: string, key: string, values?: Record<string, string | number>): string {
  return formatMessage(isLocale(locale) ? locale : 'en-US', key, values);
}

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }
function strOf(value: unknown): string { return typeof value === 'string' ? value : ''; }
function numOf(value: unknown): number { return typeof value === 'number' ? value : 0; }
function boolOf(value: unknown): boolean { return value === true; }

type SpaceSelection = 'all' | 'created' | 'joined';
type OrgSectionKey = 'created' | 'joined';
/** Tenant membership role, mirroring scopeRuntime.role() / Vue authStore. */
export type OrganizationSpaceRole = 'owner' | 'admin' | 'contributor' | 'viewer';

// R017 (?scope=): deep link + URL sync use the KB-list convention (App.tsx
// readScopeFromUrl/writeScopeToUrl). Values all|created|joined match the Vue
// spaceSelection union; 'all' removes the param so the canonical URL stays
// /platform/organizations.
function readScopeFromUrl(): SpaceSelection {
  const value = new URLSearchParams(window.location.search).get('scope');
  return value === 'created' || value === 'joined' ? value : 'all';
}

function writeScopeToUrl(selection: SpaceSelection): void {
  const url = new URL(window.location.href);
  if (selection === 'all') url.searchParams.delete('scope');
  else url.searchParams.set('scope', selection);
  window.history.replaceState({}, document.title, url.pathname + url.search + url.hash);
}

// shouldShowOrgRelationTag ported from frontend/src/utils/card-list-badge.ts:
// suppress the corner role tag when a section header already communicates it.
function shouldShowOrgRelationTag(opts: { selection: SpaceSelection; isOwner: boolean; myRole: string }): boolean {
  if (opts.selection === 'created') return false;
  if (opts.selection === 'joined' && !opts.myRole) return false;
  if (opts.selection === 'all' && opts.isOwner) return false;
  if (opts.selection === 'all' && !opts.isOwner && !opts.myRole) return false;
  return true;
}

/* Minimal inline icon set (TDesign glyph equivalents, stroke = currentColor). */
function IconGlyph(props: { d: string; size?: number; viewBox?: string; fill?: boolean; className?: string }) {
  return (
    <svg width={props.size ?? 16} height={props.size ?? 16} viewBox={props.viewBox ?? '0 0 16 16'} fill={props.fill ? 'currentColor' : 'none'} xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={props.className}>
      <path d={props.d} stroke={props.fill ? 'none' : 'currentColor'} strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
const IconUser = ({ size = 14 }: { size?: number }) => (<IconGlyph size={size} d="M8 7.2a2.6 2.6 0 1 0 0-5.2 2.6 2.6 0 0 0 0 5.2Zm-5.4 6.6c.6-2.6 2.8-4.2 5.4-4.2s4.8 1.6 5.4 4.2" />);
const IconUsergroup = ({ size = 14 }: { size?: number }) => (<IconGlyph size={size} d="M6 7a2.4 2.4 0 1 0 0-4.8A2.4 2.4 0 0 0 6 7Zm-4.6 6.4c.5-2.4 2.4-3.9 4.6-3.9 1 0 1.9.3 2.7.8M10.5 3.4a2.2 2.2 0 1 1 1.1 4.2m.5 1.9c1.6.4 2.9 1.6 3.3 3.3" />);
const IconUsergroupAdd = ({ size = 14 }: { size?: number }) => (<IconGlyph size={size} d="M6 7a2.4 2.4 0 1 0 0-4.8A2.4 2.4 0 0 0 6 7Zm-4.6 6.4c.5-2.4 2.4-3.9 4.6-3.9.9 0 1.8.2 2.5.7M12 6.5v4M10 8.5h4" />);
const IconFolder = ({ size = 14 }: { size?: number }) => (<IconGlyph size={size} d="M1.8 4.2c0-.7.6-1.3 1.3-1.3h3l1.4 1.6h5.4c.7 0 1.3.6 1.3 1.3v6c0 .7-.6 1.3-1.3 1.3H3.1c-.7 0-1.3-.6-1.3-1.3v-7.6Z" />);
const IconLayers = ({ size = 16 }: { size?: number }) => (<IconGlyph size={size} d="M8 1.8 14 5 8 8.2 2 5l6-3.2ZM2.6 8.4 8 11.2l5.4-2.8M2.6 11.4 8 14.2l5.4-2.8" />);
const IconEnter = ({ size = 16 }: { size?: number }) => (<IconGlyph size={size} d="M9.5 2.5h4v11h-4M6 5.5 3 8.5l3 3M3 8.5h7.5" />);
const IconChevron = ({ size = 14, direction }: { size?: number; direction: 'down' | 'right' }) => (<IconGlyph size={size} d={direction === 'down' ? 'M4 6l4 4 4-4' : 'M6 4l4 4-4 4'} />);
const IconClose = ({ size = 20 }: { size?: number }) => (<IconGlyph size={size} d="M4 4l8 8M12 4l-8 8" viewBox="0 0 16 16" />);
const IconBack = ({ size = 18 }: { size?: number }) => (<IconGlyph size={size} d="M10 3 5 8l5 5" />);
const IconSearch = ({ size = 14 }: { size?: number }) => (<IconGlyph size={size} d="M7 12A5 5 0 1 0 7 2a5 5 0 0 0 0 10Zm6.5 1.5L10.4 10.4" />);
const IconCheckCircle = ({ size = 18 }: { size?: number }) => (<IconGlyph size={size} d="M8 14A6 6 0 1 0 8 2a6 6 0 0 0 0 12Zm-2.6-6.2L7.5 9.9l3.2-3.8" />);
const IconInfoCircle = ({ size = 20 }: { size?: number }) => (<IconGlyph size={size} d="M8 14A6 6 0 1 0 8 2a6 6 0 0 0 0 12ZM8 7.4V11M8 5.2v.2" />);
const IconSetting = ({ size = 15 }: { size?: number }) => (<IconGlyph size={size} d="M8 10.2a2.2 2.2 0 1 0 0-4.4 2.2 2.2 0 0 0 0 4.4Zm5.6-2.2a5.6 5.6 0 0 0-.1-1l1.2-1-1.4-2.4-1.4.6a5.6 5.6 0 0 0-1.7-1L9.9 1.8H6.1L5.8 3.2a5.6 5.6 0 0 0-1.7 1l-1.4-.6-1.4 2.4 1.2 1a5.6 5.6 0 0 0 0 2l-1.2 1 1.4 2.4 1.4-.6a5.6 5.6 0 0 0 1.7 1l.3 1.4h3.8l.3-1.4a5.6 5.6 0 0 0 1.7-1l1.4.6 1.4-2.4-1.2-1c.1-.3.1-.7.1-1Z" />);
const IconLogout = ({ size = 15 }: { size?: number }) => (<IconGlyph size={size} d="M6.5 2.5h-3v11h3M10 5l3 3-3 3M13 8H6" />);
const IconDelete = ({ size = 15 }: { size?: number }) => (<IconGlyph size={size} d="M2.5 4.5h11M6 4.5v-2h4v2M4 4.5l.7 9h6.6l.7-9M6.6 7v4.2M9.4 7v4.2" />);
const IconMore = ({ size = 16 }: { size?: number }) => (<IconGlyph size={size} d="M3.2 8h.1M8 8h.1M12.8 8h.1" viewBox="0 0 16 16" />);
// Agent spark icon, ported from frontend/src/assets/img/agent-green.svg.
const IconAgent = ({ size = 14 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 20 20" fill="currentColor" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="stat-agent-icon">
    <path d="M10 3l.8 3.2c.1.5.5.9 1 1L15 8l-3.2.8c-.5.1-.9.5-1 1L10 13l-.8-3.2c-.1-.5-.5-.9-1-1L5 8l3.2-.8c.5-.1.9-.5 1-1L10 3Z" />
    <path d="M15.5 4l.3 1.2c.05.25.25.45.5.5l1.2.3-1.2.3c-.25.05-.45.25-.5.5l-.3 1.2-.3-1.2c-.05-.25-.25-.45-.5-.5l-1.2-.3 1.2-.3c.25-.05.45-.25.5-.5l.3-1.2Z" />
    <path d="M4.5 13l.3 1.2c.05.25.25.45.5.5l1.2.3-1.2.3c-.25.05-.45.25-.5.5l-.3 1.2-.3-1.2c-.05-.25-.25-.45-.5-.5l-1.2-.3 1.2-.3c.25-.05.45-.25.5-.5l.3-1.2Z" />
  </svg>
);
// Create-organization glyph, ported from frontend/src/assets/img/organization-green.svg.
const IconOrgCreate = ({ size = 16 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <path d="M10 10C8.8 7.5 7.8 3.8 4.8 3.8C2.2 3.8 0.8 6.8 0.8 10C0.8 13.2 2.2 16.2 4.8 16.2C7.8 16.2 8.8 12.5 10 10C11.2 7.5 12.5 5.5 14.5 5.5C16.5 5.5 18 7.5 18 10C18 12.5 16.5 14.5 14.5 14.5C12.5 14.5 11.2 12.5 10 10Z" stroke="#07C05F" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
  </svg>
);

/* SpaceAvatar ported from frontend/src/components/SpaceAvatar.vue. */
const AVATAR_GRADIENTS: Array<[string, string]> = [
  ['#07c05f', '#059669'], ['#11998e', '#38ef7d'], ['#43e97b', '#38f9d7'], ['#02aab0', '#00cdac'],
  ['#36d1dc', '#5b86e5'], ['#4facfe', '#00f2fe'], ['#667eea', '#764ba2'], ['#4776e6', '#8e54e9'],
  ['#56ab2f', '#a8e063'], ['#00b09b', '#96c93d'], ['#5ee7df', '#b490ca'], ['#614385', '#516395'],
];
function avatarHash(name: string): number {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = ((hash << 5) - hash) + name.charCodeAt(i);
    hash = hash & hash;
  }
  return Math.abs(hash);
}
function SpaceAvatar(props: { name: string; avatar?: unknown; size?: 'small' | 'medium' | 'large' }) {
  const size = props.size ?? 'medium';
  const dimension = size === 'small' ? 22 : size === 'large' ? 48 : 32;
  const avatarText = strOf(props.avatar).trim();
  const isEmoji = avatarText.startsWith('emoji:') && avatarText.length > 6;
  const name = props.name?.trim() ?? '';
  const firstChar = name ? name.charAt(0) : '?';
  const letter = /[a-zA-Z]/.test(firstChar) ? firstChar.toUpperCase() : firstChar;
  const gradient = AVATAR_GRADIENTS[avatarHash(name) % AVATAR_GRADIENTS.length];
  const className = 'space-avatar' + (size === 'small' ? ' space-avatar-small' : size === 'large' ? ' space-avatar-large' : '');
  const style: Record<string, string> = isEmoji
    ? { background: 'linear-gradient(135deg, #f1f5f9 0%, #e2e8f0 100%)', width: dimension + 'px', height: dimension + 'px' }
    : { background: 'linear-gradient(135deg, ' + gradient[0] + ' 0%, ' + gradient[1] + ' 100%)', width: dimension + 'px', height: dimension + 'px' };
  return (
    <div className={className} style={style}>
      {isEmoji ? (
        <span style={{ fontSize: Math.round(dimension * 0.5) + 'px' }}>{avatarText.slice(6).trim()}</span>
      ) : (
        <>
          <svg className="space-avatar-decoration" viewBox="0 0 56 40" width={dimension} height={Math.round(dimension * 40 / 56)} preserveAspectRatio="xMaxYMax meet" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
            <circle cx="10" cy="12" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
            <circle cx="28" cy="8" r="5" stroke="currentColor" strokeWidth="1.8" fill="none" opacity="0.7" />
            <circle cx="46" cy="14" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
            <path d="M14 13 L24 10 M32 10 L42 13" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" opacity="0.4" />
            <circle cx="28" cy="28" r="6" stroke="currentColor" strokeWidth="1.2" fill="none" opacity="0.35" />
            <path d="M28 14 L28 22 M20 18 L26 24 M36 18 L30 24" stroke="currentColor" strokeWidth="1" strokeLinecap="round" opacity="0.3" />
          </svg>
          <span className="space-avatar-letter" style={{ textShadow: '0 1px 2px ' + gradient[1] + '80, 0 0 8px ' + gradient[0] + '30' }}>{letter}</span>
        </>
      )}
    </div>
  );
}

/* Constellation card decoration, ported verbatim from OrganizationList.vue. */
function CardDecoration() {
  return (
    <div className="card-decoration">
      <svg width="56" height="40" viewBox="0 0 56 40" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
        <circle cx="10" cy="12" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
        <circle cx="28" cy="8" r="5" stroke="currentColor" strokeWidth="1.8" fill="none" opacity="0.7" />
        <circle cx="46" cy="14" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
        <path d="M14 13 L24 10 M32 10 L42 13" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" opacity="0.4" />
        <circle cx="28" cy="28" r="6" stroke="currentColor" strokeWidth="1.2" fill="none" opacity="0.35" />
        <path d="M28 14 L28 22 M20 18 L26 24 M36 18 L30 24" stroke="currentColor" strokeWidth="1" strokeLinecap="round" opacity="0.3" />
      </svg>
    </div>
  );
}

function FeatureBadge(props: { tone: 'stat-member' | 'stat-kb' | 'stat-agent'; title: string; count: number }) {
  return (
    <div className={'feature-badge ' + props.tone} title={props.title}>
      {props.tone === 'stat-member' ? <IconUser /> : props.tone === 'stat-kb' ? <IconFolder /> : <IconAgent />}
      <span className="badge-count">{props.count}</span>
    </div>
  );
}

function skeletonCard(key: string) {
  return (
    <div key={key} className="org-card org-card-skeleton">
      <div className="card-header">
        <div className="card-header-left">
          <div className="org-skel-block" style={{ width: '36px', height: '36px', borderRadius: '8px' }} />
          <div className="org-skel-block" style={{ width: '50%', height: '20px' }} />
        </div>
      </div>
      <div style={{ flex: 1, marginTop: '12px' }}>
        <div className="org-skel-block" style={{ width: '100%', height: '14px', marginBottom: '8px' }} />
        <div className="org-skel-block" style={{ width: '70%', height: '14px' }} />
      </div>
      <div className="card-bottom">
        <div className="org-skel-block" style={{ width: '60px', height: '22px' }} />
        <div className="org-skel-block" style={{ width: '60px', height: '22px' }} />
      </div>
    </div>
  );
}

type ToastState = { tone: 'success' | 'error'; text: string } | null;

export function OrganizationsPage({ client, inviteCode, role }: { client: WeKnoraClient; inviteCode?: string; role?: OrganizationSpaceRole }) {
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState<ToastState>(null);
  const [selection, setSelectionState] = useState<SpaceSelection>(readScopeFromUrl);
  const [collapsedSections, setCollapsedSections] = useState<Set<OrgSectionKey>>(new Set());
  const [moreMenuOrgId, setMoreMenuOrgId] = useState<string | null>(null);

  // Settings / create modal (Vue OrganizationSettingsModal parity).
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settingsMode, setSettingsMode] = useState<'create' | 'edit'>('edit');
  const [settingsOrg, setSettingsOrg] = useState<Organization | null>(null);
  const [settingsSection, setSettingsSection] = useState('basic');
  const [formName, setFormName] = useState('');
  const [formDescription, setFormDescription] = useState('');
  const [saving, setSaving] = useState(false);
  const [members, setMembers] = useState<OrganizationMember[]>([]);
  const [requests, setRequests] = useState<OrganizationJoinRequest[]>([]);
  const [sharedResources, setSharedResources] = useState<Array<Record<string, unknown>>>([]);
  const [inviteLink, setInviteLink] = useState('');
  const [upgradeRole, setUpgradeRole] = useState<'admin' | 'editor' | 'viewer'>('editor');
  const [upgradeNote, setUpgradeNote] = useState('');

  // Join modal (Vue invite-preview modal parity).
  const [joinOpen, setJoinOpen] = useState(false);
  const [joinStep, setJoinStep] = useState<'invite' | 'search'>('invite');
  const [joinInputCode, setJoinInputCode] = useState('');
  const [joinCode, setJoinCode] = useState('');
  const [joinPreview, setJoinPreview] = useState<Record<string, unknown> | null>(null);
  const [joinPreviewLoading, setJoinPreviewLoading] = useState(false);
  const [joinPreviewError, setJoinPreviewError] = useState('');
  const [joining, setJoining] = useState(false);
  const [requestRole, setRequestRole] = useState<'admin' | 'editor' | 'viewer'>('viewer');
  const [requestNote, setRequestNote] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [searchItems, setSearchItems] = useState<Array<Record<string, unknown>>>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [activeInviteCode, setActiveInviteCode] = useState(inviteCode);
  const [confirmState, setConfirmState] = useState<{ kind: 'leave' | 'delete'; org: Organization } | null>(null);
  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const locale = currentLocale();
  const organizationsApi = client.identity.organizations;

  function showToast(tone: 'success' | 'error', text: string) { setToast({ tone, text }); }

  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(null), 3000);
    return () => clearTimeout(timer);
  }, [toast]);

  // R017 RBAC (Vue OrganizationList.vue:499-504): every write affordance on
  // this page (创建/加入/删除) requires the current TENANT role ≥ admin —
  // canManageOrg = hasRole('admin') || canAccessAllTenants. An explicit role
  // prop (scopeRuntime.role() wiring, same pattern as SettingsPage) wins;
  // otherwise the page resolves it from auth/me: the active-tenant membership
  // role (selected tenant, falling back to the home tenant like Vue's
  // currentTenantRole) plus the can_access_all_tenants superuser flag. UI
  // rendering only — the server route guard remains the real boundary.
  const [resolvedCanManage, setResolvedCanManage] = useState<boolean | null>(null);
  useEffect(() => {
    if (role) {
      setResolvedCanManage(role === 'admin' || role === 'owner');
      return;
    }
    let active = true;
    void client.auth?.me?.().then((me) => {
      if (!active) return;
      const record = me.user as Record<string, unknown>;
      const selected = readReactPlatformState(window.localStorage)?.tenantId ?? null;
      const homeTenant = me.tenant && me.tenant.id !== null && me.tenant.id !== undefined ? String(me.tenant.id) : '';
      const tenantId = selected ?? homeTenant;
      let currentRole = '';
      for (const item of me.memberships ?? []) {
        if (!item || typeof item !== 'object') continue;
        const row = item as Record<string, unknown>;
        const id = row.tenant_id ?? row.tenantId;
        if (tenantId && String(id) === tenantId && typeof row.role === 'string') { currentRole = row.role; break; }
      }
      setResolvedCanManage(currentRole === 'admin' || currentRole === 'owner' || record.can_access_all_tenants === true);
    }).catch(() => {
      // Identity unavailable (embedded/test mounts): keep the legacy
      // permissive UI; the server still rejects unauthorized writes.
      if (active) setResolvedCanManage(true);
    });
    return () => { active = false; };
  }, [client, role]);
  const canManageOrg = resolvedCanManage ?? true;
  const writeGuardTitle = t(locale, 'organization.rbac.needTenantAdminTip');

  // Vue ListSpaceSidebar v-model="spaceSelection": rail clicks mirror the
  // selection into ?scope= so shell sub-filter and deep links stay in sync.
  const setSelection = (next: SpaceSelection) => {
    setSelectionState(next);
    writeScopeToUrl(next);
  };

  function clearInviteFromUrl() {
    const url = new URL(window.location.href);
    url.searchParams.delete('invite_code');
    window.history.replaceState({}, document.title, url.pathname + url.search + url.hash);
  }

  useEffect(() => { setActiveInviteCode(inviteCode); }, [inviteCode]);

  // An incoming invite code (URL ?invite_code=…) opens the join modal and
  // previews the target organization, mirroring the Vue onMounted behavior.
  useEffect(() => {
    if (!activeInviteCode) return;
    let active = true;
    setJoinOpen(true);
    setJoinCode(activeInviteCode);
    setJoinPreview(null);
    setJoinPreviewError('');
    setJoinPreviewLoading(true);
    void organizationsApi.preview(activeInviteCode).then((preview) => { if (active) setJoinPreview(preview); })
      .catch((reason) => { if (active) setJoinPreviewError(errorText(reason, t(locale, 'organization.invite.invalidCode'))); })
      .finally(() => { if (active) setJoinPreviewLoading(false); });
    return () => { active = false; };
  }, [activeInviteCode, organizationsApi]);

  async function load() {
    setLoading(true);
    try {
      const result = await organizationsApi.list();
      setOrganizations(result.items);
      if (settingsOrg) {
        const next = result.items.find((item) => item.id === settingsOrg.id);
        if (next) setSettingsOrg(next);
      }
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.deleteFailed'))); }
    finally { setLoading(false); }
  }
  useEffect(() => { void load(); /* eslint-disable-line react-hooks/exhaustive-deps */ }, [client]);

  async function loadOrganizationDetail(id: string) {
    // Same three feeds the legacy React page loaded on select(); failures keep
    // server state untouched and surface as toasts.
    const [membersResult, requestsResult, sharesResult] = await Promise.allSettled([
      organizationsApi.members.list(id),
      organizationsApi.joinRequests.list(id),
      organizationsApi.knowledgeBaseShares.listForOrganization(id),
    ]);
    if (membersResult.status === 'fulfilled') setMembers(membersResult.value.items); else showToast('error', errorText(membersResult.reason, t(locale, 'organization.memberRemoveFailed')));
    if (requestsResult.status === 'fulfilled') setRequests(requestsResult.value.items);
    if (sharesResult.status === 'fulfilled') setSharedResources(sharesResult.value.items);
  }

  function openCreateModal() {
    setSettingsMode('create'); setSettingsOrg(null); setSettingsSection('basic');
    setFormName(''); setFormDescription('');
    setSettingsOpen(true);
  }

  function openSettingsModal(org: Organization) {
    setSettingsMode('edit'); setSettingsOrg(org); setSettingsSection('basic');
    setFormName(org.name); setFormDescription(strOf(org.description));
    setInviteLink(''); setMembers([]); setRequests([]); setSharedResources([]);
    setSettingsOpen(true);
    void loadOrganizationDetail(org.id);
  }

  function closeSettings() { setSettingsOpen(false); setSettingsOrg(null); }

  function openJoinModal() {
    setJoinOpen(true); setJoinStep('invite'); setJoinInputCode(''); setJoinPreview(null);
    setJoinPreviewError(''); setRequestRole('viewer'); setRequestNote('');
    setSearchQuery(''); setSearchItems([]);
  }

  function closeJoin() {
    setJoinOpen(false); setJoinPreview(null); setJoinInputCode(''); setJoinCode(''); setJoinPreviewError('');
    setJoinStep('invite'); setSearchQuery(''); setSearchItems([]); setRequestNote(''); setRequestRole('viewer');
    setActiveInviteCode(undefined);
    clearInviteFromUrl();
  }

  async function doPreviewFromInput() {
    const code = joinInputCode.trim();
    if (!code) { showToast('error', t(locale, 'organization.inviteCodeRequired')); return; }
    setJoinCode(code); setJoinPreviewError(''); setJoinPreview(null); setJoinPreviewLoading(true);
    try { setJoinPreview(await organizationsApi.preview(code)); }
    catch (reason) { setJoinPreviewError(errorText(reason, t(locale, 'organization.invite.invalidCode'))); }
    finally { setJoinPreviewLoading(false); }
  }

  function runSearch(query: string) {
    setSearchLoading(true);
    void organizationsApi.search(query, 20).then((result) => setSearchItems(result.items))
      .catch(() => setSearchItems([]))
      .finally(() => setSearchLoading(false));
  }

  function onSearchQueryChange(value: string) {
    setSearchQuery(value);
    if (searchTimer.current) clearTimeout(searchTimer.current);
    searchTimer.current = setTimeout(() => runSearch(value.trim()), 300);
  }

  function openSearchTab() { setJoinStep('search'); runSearch(searchQuery.trim()); }

  function previewSearchableOrg(row: Record<string, unknown>) {
    setJoinPreview(row);
    setRequestRole('viewer'); setRequestNote('');
  }

  const previewJoinMode = inviteJoinMode(joinPreview);
  const previewIsAlreadyMember = boolOf(joinPreview?.is_already_member);

  async function confirmJoin() {
    if (!joinPreview || previewIsAlreadyMember) return;
    setJoining(true);
    try {
      if (!joinCode) {
        // Joined from the search tab: no invite code, join by organization id.
        await organizationsApi.joinById(strOf(joinPreview.id), previewJoinMode === 'request' ? { message: clampApplicationNote(requestNote) || undefined, role: requestRole } : {});
      } else if (previewJoinMode === 'request') {
        await organizationsApi.submitJoinRequest({ invite_code: joinCode, role: requestRole, ...(requestNote.trim() ? { message: clampApplicationNote(requestNote) } : {}) });
      } else {
        await organizationsApi.join({ invite_code: joinCode });
      }
      setJoinOpen(false); setJoinPreview(null); setJoinCode(''); setActiveInviteCode(undefined);
      clearInviteFromUrl();
      await load();
      showToast('success', t(locale, previewJoinMode === 'request' ? 'organization.invite.requestSubmitted' : 'organization.invite.joinSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.invite.joinFailed'))); }
    finally { setJoining(false); }
  }

  async function submitCreate(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!formName.trim()) return;
    setSaving(true);
    try {
      await organizationsApi.create({ name: formName.trim(), description: formDescription });
      setSettingsOpen(false); setFormName(''); setFormDescription('');
      await load();
      showToast('success', t(locale, 'organization.createSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.createFailed'))); }
    finally { setSaving(false); }
  }

  async function submitBasic(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!settingsOrg || !formName.trim()) return;
    setSaving(true);
    try {
      await organizationsApi.update(settingsOrg.id, { name: formName.trim(), description: formDescription });
      await load();
      showToast('success', t(locale, 'organization.roleUpdated'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.roleUpdateFailed'))); }
    finally { setSaving(false); }
  }

  async function updateMemberRole(member: OrganizationMember, nextRole: 'admin' | 'editor' | 'viewer') {
    if (!settingsOrg) return;
    try {
      await organizationsApi.members.updateRole(settingsOrg.id, member.tenant_id, { role: nextRole });
      await loadOrganizationDetail(settingsOrg.id);
      showToast('success', t(locale, 'organization.roleUpdated'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.roleUpdateFailed'))); }
  }

  async function removeMember(member: OrganizationMember) {
    if (!settingsOrg) return;
    if (!window.confirm(t(locale, 'organization.detail.removeMemberConfirm', { name: member.tenant_name ?? member.username }))) return;
    try {
      await organizationsApi.members.remove(settingsOrg.id, member.tenant_id);
      await loadOrganizationDetail(settingsOrg.id);
      showToast('success', t(locale, 'organization.memberRemoved'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.memberRemoveFailed'))); }
  }

  async function reviewRequest(request: OrganizationJoinRequest, approved: boolean) {
    if (!settingsOrg) return;
    try {
      await organizationsApi.joinRequests.review(settingsOrg.id, request.id, { approved, role: requestedRoleOf(request) });
      await loadOrganizationDetail(settingsOrg.id);
      showToast('success', t(locale, approved ? 'organization.settings.approveSuccess' : 'organization.settings.rejectSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.settings.reviewFailed'))); }
  }

  async function confirmLeaveOrDelete() {
    if (!confirmState) return;
    const { kind, org } = confirmState;
    try {
      if (kind === 'leave') {
        await organizationsApi.leave(org.id);
        showToast('success', t(locale, 'organization.leaveSuccess'));
      } else {
        await organizationsApi.remove(org.id);
        showToast('success', t(locale, 'organization.deleteSuccess'));
      }
      setConfirmState(null);
      setSettingsOpen(false); setSettingsOrg(null);
      await load();
    } catch (reason) {
      setConfirmState(null);
      showToast('error', errorText(reason, t(locale, kind === 'leave' ? 'organization.leaveFailed' : 'organization.deleteFailed')));
    }
  }

  async function generateInviteLink() {
    if (!settingsOrg) return;
    try {
      const { inviteCode: code } = await organizationsApi.generateInviteCode(settingsOrg.id);
      const link = buildInviteLink(code, { origin: window.location.origin, pathname: window.location.pathname, search: window.location.search });
      setInviteLink(link);
      const copied = await copyText(link);
      if (copied) showToast('success', t(locale, 'common.copied'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.invite.previewFailed'))); }
  }

  async function unshareKnowledgeBase(row: ReturnType<typeof sharedResourceRow>) {
    if (!settingsOrg) return;
    if (!window.confirm(t(locale, 'organization.settings.removeShareConfirm', { name: row.name }))) return;
    try {
      await organizationsApi.knowledgeBaseShares.remove(row.knowledgeBaseId, row.shareId);
      await loadOrganizationDetail(settingsOrg.id);
      showToast('success', t(locale, 'organization.settings.removeShareSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.settings.removeShareFailed'))); }
  }

  async function submitUpgradeRequest(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!settingsOrg) return;
    try {
      await organizationsApi.requestRoleUpgrade(settingsOrg.id, { requested_role: upgradeRole, ...(upgradeNote.trim() ? { message: clampApplicationNote(upgradeNote) } : {}) });
      setUpgradeNote('');
      showToast('success', t(locale, 'organization.upgrade.submitSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.upgrade.submitFailed'))); }
  }

  const isOwnerOf = (org: Organization) => boolOf(org.is_owner);
  const sectionOf = (org: Organization): OrgSectionKey => (isOwnerOf(org) ? 'created' : 'joined');
  const createdCount = organizations.filter(isOwnerOf).length;
  const joinedCount = organizations.length - createdCount;
  const ordered = useMemo(() => {
    if (selection === 'created') return organizations.filter(isOwnerOf);
    if (selection === 'joined') return organizations.filter((org) => !isOwnerOf(org));
    // 全部 view: owners first, matching the Vue grouping headers.
    return [...organizations].sort((a, b) => {
      if (isOwnerOf(a) === isOwnerOf(b)) return 0;
      return isOwnerOf(a) ? -1 : 1;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [organizations, selection]);

  const toggleSection = (key: OrgSectionKey) => {
    setCollapsedSections((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key); else next.add(key);
      return next;
    });
  };

  function sectionHeader(key: OrgSectionKey) {
    const collapsed = collapsedSections.has(key);
    return (
      <div key={'header-' + key} className="org-section-header" role="button" tabIndex={0}
        onClick={() => toggleSection(key)}
        onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); toggleSection(key); } }}>
        {key === 'created' ? <IconUsergroupAdd /> : <IconUsergroup />}
        <span>{t(locale, key === 'created' ? 'organization.createdByMe' : 'organization.joinedByMe')}</span>
        <span className="org-section-count">{key === 'created' ? createdCount : joinedCount}</span>
        <span className="org-section-toggle"><IconChevron direction={collapsed ? 'right' : 'down'} /></span>
      </div>
    );
  }

  const cardRows: React.ReactNode[] = [];
  let createdHeaderPlaced = false;
  let joinedHeaderPlaced = false;
  ordered.forEach((org, index) => {
    const owner = isOwnerOf(org);
    if (selection === 'all') {
      if (owner && !createdHeaderPlaced) { cardRows.push(sectionHeader('created')); createdHeaderPlaced = true; }
      if (!owner && !joinedHeaderPlaced) { cardRows.push(sectionHeader('joined')); joinedHeaderPlaced = true; }
      if (collapsedSections.has(sectionOf(org))) return;
    }
    const role = strOf(org.my_role);
    const memberCount = numOf(org.member_count);
    const shareCount = numOf(org.share_count);
    const agentShareCount = numOf(org.agent_share_count);
    const pendingCount = numOf(org.pending_join_request_count);
    const description = strOf(org.description);
    const showRelation = shouldShowOrgRelationTag({ selection, isOwner: owner, myRole: role });
    const relationClass = owner ? 'owner' : role;
    cardRows.push(
      <div key={org.id || index} className="org-card" role="button" tabIndex={0}
        onClick={() => openSettingsModal(org)}
        onKeyDown={(event) => { if (event.key === 'Enter') openSettingsModal(org); }}>
        <CardDecoration />
        <div className="card-header">
          <div className="card-header-left">
            <div className="org-avatar"><SpaceAvatar name={org.name} avatar={org.avatar} size="small" /></div>
            <div className="card-title-block"><span className="card-title" title={org.name}>{org.name}</span></div>
          </div>
          <div className={'more-wrap' + (moreMenuOrgId === org.id ? ' active-more' : '')}
            role="button" tabIndex={0} aria-label={t(locale, 'common.edit')}
            onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(moreMenuOrgId === org.id ? null : org.id); }}
            onKeyDown={(event) => { if (event.key === 'Enter') { event.stopPropagation(); setMoreMenuOrgId(moreMenuOrgId === org.id ? null : org.id); } }}>
            <IconMore />
            {moreMenuOrgId === org.id ? (
              <div className="card-more-popup popup-menu-host">
                <div className="popup-menu" onClick={(event) => event.stopPropagation()}>
                  <div className="popup-menu-item" onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(null); openSettingsModal(org); }}>
                    <IconSetting /><span>{t(locale, 'organization.settings.editTitle')}</span>
                  </div>
                  {!owner ? (
                    <div className="popup-menu-item delete" onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(null); setConfirmState({ kind: 'leave', org }); }}>
                      <IconLogout /><span>{t(locale, 'organization.leave')}</span>
                    </div>
                  ) : canManageOrg ? (
                    // Vue: v-if="org.is_owner && canManageOrg" — deleting an
                    // owned space also requires the tenant admin+ role.
                    <div className="popup-menu-item delete" onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(null); setConfirmState({ kind: 'delete', org }); }}>
                      <IconDelete /><span>{t(locale, 'common.delete')}</span>
                    </div>
                  ) : null}
                </div>
              </div>
            ) : null}
          </div>
        </div>
        <div className="card-content">
          <div className="card-description">{description || t(locale, 'organization.noDescription')}</div>
        </div>
        <div className="card-bottom">
          <div className="card-bottom-left">
            <div className="feature-badges">
              <FeatureBadge tone="stat-member" title={t(locale, 'organization.memberCount')} count={memberCount} />
              <FeatureBadge tone="stat-kb" title={t(locale, 'organization.invite.knowledgeBases')} count={shareCount} />
              <FeatureBadge tone="stat-agent" title={t(locale, 'organization.invite.agents')} count={agentShareCount} />
            </div>
            {pendingCount > 0 ? (
              <span className="pending-requests-badge" title={t(locale, 'organization.settings.pendingJoinRequestsBadge')}>{pendingCount} {t(locale, 'organization.settings.pendingReview')}</span>
            ) : null}
          </div>
          {showRelation ? (
            <div className="card-bottom-right">
              <div className={'relation-role-tag ' + relationClass}>
                {owner ? <IconUsergroupAdd /> : <IconUsergroup />}
                <span>{owner ? t(locale, 'organization.owner') : role ? t(locale, 'organization.role.' + role) : t(locale, 'organization.joinedByMe')}</span>
              </div>
            </div>
          ) : null}
        </div>
      </div>
    );
  });

  const emptyTitle = selection === 'created' ? t(locale, 'organization.emptyCreated') : selection === 'joined' ? t(locale, 'organization.emptyJoined') : t(locale, 'organization.empty');
  const emptyDesc = selection === 'created' ? t(locale, 'organization.emptyCreatedDesc') : selection === 'joined' ? t(locale, 'organization.emptyJoinedDesc') : t(locale, 'organization.emptyDesc');

  const roleOptions: Array<['admin' | 'editor' | 'viewer', string]> = [['viewer', 'organization.role.viewer'], ['editor', 'organization.role.editor'], ['admin', 'organization.role.admin']];
  const previewName = strOf(joinPreview?.name);
  const previewDescription = strOf(joinPreview?.description);
  const searchRowFull = (row: Record<string, unknown>) => numOf(row.member_limit) > 0 && numOf(row.member_count) >= numOf(row.member_limit);

  return (
    <main className="wk-page wk-org-page">
      <div className="org-list-container">
        <aside className="org-space-rail" aria-label={t(locale, 'organization.title')}>
          {/* Vue ListSpaceSidebar collapsed-strip tooltips carry the live
              counts (tooltipText(name, count) → "name (count)"). */}
          <button type="button" className={'org-rail-item' + (selection === 'all' ? ' is-active' : '')} title={t(locale, 'common.all') + ' (' + organizations.length + ')'} onClick={() => setSelection('all')}>
            <span className="org-rail-icon"><IconLayers /></span>
            <span>{t(locale, 'common.all')}</span>
          </button>
          <button type="button" className={'org-rail-item' + (selection === 'created' ? ' is-active' : '')} title={t(locale, 'organization.createdByMe') + ' (' + createdCount + ')'} onClick={() => setSelection('created')}>
            <span className="org-rail-icon"><IconUsergroupAdd /></span>
            <span>{t(locale, 'organization.createdByMe')}</span>
          </button>
          <button type="button" className={'org-rail-item' + (selection === 'joined' ? ' is-active' : '')} title={t(locale, 'organization.joinedByMe') + ' (' + joinedCount + ')'} onClick={() => setSelection('joined')}>
            <span className="org-rail-icon"><IconUsergroup /></span>
            <span>{t(locale, 'organization.joinedByMe')}</span>
          </button>
        </aside>
        <div className="org-list-content">
          <header className="org-header">
            <div className="org-header-title">
              <div className="org-title-row">
                <h2 className="org-title">{t(locale, 'organization.title')}</h2>
                <div className="org-header-actions">
                  {/* Vue header-actions: disabled={!canManageOrg} with the
                      joinOrg/createOrg tooltip swapped for the rbac tip. */}
                  <button type="button" className="org-header-action-btn" aria-label={t(locale, 'organization.joinOrg')} title={canManageOrg ? t(locale, 'organization.joinOrg') : writeGuardTitle} disabled={!canManageOrg} onClick={openJoinModal}>
                    <IconEnter />
                  </button>
                  <button type="button" className="org-header-action-btn" aria-label={t(locale, 'organization.createOrg')} title={canManageOrg ? t(locale, 'organization.createOrg') : writeGuardTitle} disabled={!canManageOrg} onClick={openCreateModal}>
                    <IconOrgCreate />
                  </button>
                </div>
              </div>
              <p className="org-header-subtitle">{t(locale, 'organization.subtitle')}</p>
            </div>
          </header>
          <div className="org-list-main">
            {loading && organizations.length === 0 ? (
              <div className="org-card-wrap">{[1, 2, 3, 4].map((n) => skeletonCard('skel-' + n))}</div>
            ) : ordered.length === 0 ? (
              <div className="empty-state">
                <img className="empty-img" src={emptyIllustration} alt="" />
                <span className="empty-txt">{emptyTitle}</span>
                <span className="empty-desc">{emptyDesc}</span>
                <div className="empty-state-actions">
                  <button type="button" className="org-btn outline" title={canManageOrg ? undefined : writeGuardTitle} disabled={!canManageOrg} onClick={openJoinModal}>
                    <IconEnter />{t(locale, 'organization.joinOrg')}
                  </button>
                  <button type="button" className="org-btn primary" title={canManageOrg ? undefined : writeGuardTitle} disabled={!canManageOrg} onClick={openCreateModal}>
                    <IconOrgCreate />{t(locale, 'organization.createOrg')}
                  </button>
                </div>
              </div>
            ) : (
              <div className="org-card-wrap">{cardRows}</div>
            )}
          </div>
        </div>
      </div>

      {/* Create / edit settings modal. */}
      {settingsOpen ? (
        <div className="org-modal-overlay" onClick={closeSettings}>
          <div className="org-settings-modal" role="dialog" aria-label={t(locale, settingsMode === 'create' ? 'organization.createOrg' : 'organization.settings.editTitle')} onClick={(event) => event.stopPropagation()}>
            <button type="button" className="org-close-btn" aria-label={t(locale, 'common.close')} onClick={closeSettings}><IconClose /></button>
            <div className="org-settings-body">
              {settingsMode === 'edit' && settingsOrg ? (
                <nav className="org-settings-nav">
                  <h2 className="org-settings-nav-title">{t(locale, 'organization.settings.editTitle')}</h2>
                  {([
                    ['basic', 'organization.editor.navBasic'],
                    ['members', 'organization.members.listTitle'],
                    ['requests', 'organization.joinRequests.listTitle'],
                    ['shares', 'organization.sharedResources.kbListTitle'],
                    ['invite', 'organization.settings.inviteLink'],
                  ] as Array<[string, string]>).map(([key, labelKey]) => (
                    <button key={key} type="button" className={'org-settings-nav-item' + (settingsSection === key ? ' active' : '')} onClick={() => setSettingsSection(key)}>{t(locale, labelKey)}</button>
                  ))}
                </nav>
              ) : null}
              <div className="org-settings-content">
                <div className="org-settings-main">
                  {settingsMode === 'create' ? (
                    <form onSubmit={submitCreate}>
                      <h2 className="org-section-title">{t(locale, 'organization.createOrg')}</h2>
                      <p className="org-section-desc">{t(locale, 'organization.editor.basicDesc')}</p>
                      <div className="org-form-item">
                        <label className="org-form-label" htmlFor="organization-name">{t(locale, 'organization.name')} *</label>
                        <p className="org-form-desc">{t(locale, 'organization.editor.nameTip')}</p>
                        <input id="organization-name" name="organization-name" className="org-input" value={formName} onChange={(event) => setFormName(event.target.value)} required />
                      </div>
                      <div className="org-form-item">
                        <label className="org-form-label" htmlFor="organization-description">{t(locale, 'organization.description')}</label>
                        <p className="org-form-desc">{t(locale, 'organization.editor.descriptionTip')}</p>
                        <textarea id="organization-description" name="organization-description" className="org-textarea" rows={3} value={formDescription} onChange={(event) => setFormDescription(event.target.value)} />
                      </div>
                      <button type="submit" className="org-btn primary" disabled={saving}>{t(locale, 'organization.createOrg')}</button>
                    </form>
                  ) : settingsSection === 'basic' ? (
                    <>
                      <form onSubmit={submitBasic}>
                        <h2 className="org-section-title">{t(locale, 'organization.editor.basicTitle')}</h2>
                        <p className="org-section-desc">{t(locale, 'organization.editor.basicDesc')}</p>
                        <div className="org-form-item">
                          <label className="org-form-label" htmlFor="organization-name">{t(locale, 'organization.name')} *</label>
                          <input id="organization-name" name="organization-name" className="org-input" value={formName} onChange={(event) => setFormName(event.target.value)} required />
                        </div>
                        <div className="org-form-item">
                          <label className="org-form-label" htmlFor="organization-description">{t(locale, 'organization.description')}</label>
                          <textarea id="organization-description" name="organization-description" className="org-textarea" rows={3} value={formDescription} onChange={(event) => setFormDescription(event.target.value)} />
                        </div>
                        <button type="submit" className="org-btn primary" disabled={saving}>{t(locale, 'common.save')}</button>
                      </form>
                      <form onSubmit={submitUpgradeRequest} style={{ marginTop: '24px', borderTop: '1px dashed var(--org-stroke)', paddingTop: '16px' }}>
                        <h3 className="org-section-title">{t(locale, 'organization.upgrade.requestUpgrade')}</h3>
                        <div className="org-form-item">
                          <label className="org-form-label" htmlFor="upgrade-role">{t(locale, 'organization.upgrade.selectRole')}</label>
                          <select id="upgrade-role" className="org-select" value={upgradeRole} onChange={(event) => setUpgradeRole(event.target.value as 'admin' | 'editor' | 'viewer')}>
                            {roleOptions.map(([value, labelKey]) => <option key={value} value={value}>{t(locale, labelKey)}</option>)}
                          </select>
                        </div>
                        <div className="org-form-item">
                          <label className="org-form-label" htmlFor="upgrade-note">{t(locale, 'organization.upgrade.reason')}</label>
                          <textarea id="upgrade-note" className="org-textarea" rows={2} maxLength={500} value={upgradeNote} onChange={(event) => setUpgradeNote(clampApplicationNote(event.target.value))} placeholder={t(locale, 'organization.upgrade.reasonPlaceholder')} />
                        </div>
                        <button type="submit" className="org-btn outline">{t(locale, 'organization.upgrade.submitBtn')}</button>
                      </form>
                    </>
                  ) : settingsSection === 'members' ? (
                    <>
                      <h2 className="org-section-title">{t(locale, 'organization.members.listTitle')}</h2>
                      <p className="org-section-desc">{t(locale, 'organization.settings.membersDesc')}</p>
                      {members.length === 0 ? <p className="org-empty-inline">{t(locale, 'organization.noMembers')}</p> : members.map((member) => (
                        <div key={member.id} className="org-member-row">
                          <div className="org-member-copy">
                            <strong>{member.tenant_name ?? member.username}</strong>
                            <span>{member.email} · {t(locale, 'organization.role.' + member.role)}</span>
                          </div>
                          <div className="org-row-actions">
                            <select className="org-select" aria-label={t(locale, 'organization.members.columns.role')} value={member.role} onChange={(event) => void updateMemberRole(member, event.target.value as 'admin' | 'editor' | 'viewer')}>
                              {roleOptions.map(([value, labelKey]) => <option key={value} value={value}>{t(locale, labelKey)}</option>)}
                            </select>
                            <button type="button" className="org-btn neutral" onClick={() => void removeMember(member)}>{t(locale, 'common.remove')}</button>
                          </div>
                        </div>
                      ))}
                    </>
                  ) : settingsSection === 'requests' ? (
                    <>
                      <h2 className="org-section-title">{t(locale, 'organization.joinRequests.listTitle')}</h2>
                      {requests.filter((request) => request.status === 'pending').length === 0 ? <p className="org-empty-inline">{t(locale, 'organization.settings.noPendingRequests')}</p> : requests.filter((request) => request.status === 'pending').map((request) => (
                        <div key={request.id} className="org-request-row">
                          <div className="org-member-copy">
                            <strong>{request.username}</strong>
                            <span>{t(locale, 'organization.joinRequests.columns.requestedRole')}: {t(locale, 'organization.role.' + request.requested_role)}</span>
                          </div>
                          <div className="org-row-actions">
                            <button type="button" className="org-btn outline" onClick={() => void reviewRequest(request, true)}>{t(locale, 'organization.settings.approve')}</button>
                            <button type="button" className="org-btn neutral" onClick={() => void reviewRequest(request, false)}>{t(locale, 'organization.settings.reject')}</button>
                          </div>
                        </div>
                      ))}
                    </>
                  ) : settingsSection === 'shares' ? (
                    <>
                      <h2 className="org-section-title">{t(locale, 'organization.sharedResources.kbListTitle')}</h2>
                      {sharedResources.length === 0 ? <p className="org-empty-inline">{t(locale, 'organization.settings.noSharedKB')}</p> : sharedResources.map((resource, index) => {
                        const row = sharedResourceRow(resource);
                        return (
                          <div key={row.shareId || index} className="org-member-row">
                            <div className="org-member-copy">
                              <strong>{row.name}</strong>
                              <span>{row.permission || t(locale, 'organization.sharedResources.columns.permission')}</span>
                            </div>
                            {row.canUnshare ? (
                              <div className="org-row-actions">
                                <button type="button" className="org-btn neutral" onClick={() => void unshareKnowledgeBase(row)}>{t(locale, 'organization.share.unshareAction')}</button>
                              </div>
                            ) : null}
                          </div>
                        );
                      })}
                    </>
                  ) : (
                    <>
                      <h2 className="org-section-title">{t(locale, 'organization.settings.inviteLink')}</h2>
                      <p className="org-section-desc">{t(locale, 'organization.settings.inviteMembersDesc')}</p>
                      <div className="org-row-actions" style={{ marginBottom: '12px' }}>
                        <button type="button" className="org-btn primary" onClick={() => void generateInviteLink()}>{t(locale, 'organization.settings.inviteMembers')}</button>
                      </div>
                      {inviteLink ? (
                        <div className="org-invite-code-block">
                          <code>{inviteLink}</code>
                          <button type="button" className="org-btn outline" onClick={() => { void copyText(inviteLink).then((copied) => { if (copied) showToast('success', t(locale, 'common.copied')); }); }}>{t(locale, 'common.copy')}</button>
                        </div>
                      ) : null}
                    </>
                  )}
                </div>
                <div className="org-settings-footer">
                  <button type="button" className="org-btn neutral" onClick={closeSettings}>{t(locale, 'common.cancel')}</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {/* Join modal (invite code / search / preview). */}
      {joinOpen ? (
        <div className="org-modal-overlay" onClick={closeJoin}>
          <div className={'org-join-modal' + (!joinPreview && !joinPreviewLoading && joinStep === 'search' ? ' is-wide' : '')} role="dialog" aria-label={t(locale, joinPreview ? 'organization.invite.previewTitle' : 'organization.joinOrg')} onClick={(event) => event.stopPropagation()}>
            <button type="button" className="org-close-btn" aria-label={t(locale, 'common.close')} onClick={closeJoin}><IconClose /></button>
            <div className="org-join-header">
              {joinPreview && !joinCode ? (
                <button type="button" className="org-join-back" aria-label={t(locale, 'organization.join.backToSearch')} onClick={() => { setJoinPreview(null); setJoinStep('search'); }}><IconBack /></button>
              ) : null}
              <h2 className="org-join-title">{joinPreview ? t(locale, 'organization.invite.previewTitle') : t(locale, 'organization.joinOrg')}</h2>
            </div>
            <div className="org-join-body">
              {joinPreviewLoading ? (
                <div className="org-empty-inline" style={{ textAlign: 'center', padding: '48px 0' }}>{t(locale, 'organization.invite.loading')}</div>
              ) : joinPreview ? (
                <>
                  <div className="org-preview-hero">
                    <SpaceAvatar name={previewName} avatar={joinPreview.avatar} size="large" />
                    <h3 className="org-preview-name">{previewName}</h3>
                    <p className="org-preview-desc">{previewDescription || t(locale, 'organization.noDescription')}</p>
                    <div className="feature-badges org-preview-badges">
                      <FeatureBadge tone="stat-member" title={t(locale, 'organization.memberCount')} count={numOf(joinPreview.member_count)} />
                      <FeatureBadge tone="stat-kb" title={t(locale, 'organization.invite.knowledgeBases')} count={numOf(joinPreview.share_count)} />
                      <FeatureBadge tone="stat-agent" title={t(locale, 'organization.invite.agents')} count={numOf(joinPreview.agent_share_count)} />
                    </div>
                  </div>
                  {previewIsAlreadyMember ? (
                    <div className="preview-member-status"><IconCheckCircle /><span>{t(locale, 'organization.invite.alreadyMember')}</span></div>
                  ) : (
                    <div className="org-preview-join-summary" style={{ borderTop: '1px solid var(--org-stroke)', paddingTop: '16px' }}>
                      <div className="org-preview-info-row">
                        <span className="org-preview-info-label">{t(locale, 'organization.invite.approvalLabel')}</span>
                        <span className={'org-tag ' + (previewJoinMode === 'request' ? 'warning' : 'success')}>{t(locale, previewJoinMode === 'request' ? 'organization.invite.needApproval' : 'organization.invite.noApproval')}</span>
                      </div>
                      {previewJoinMode === 'request' ? (
                        <>
                          <p className="org-preview-info-desc" style={{ color: 'var(--org-warning)' }}>{t(locale, 'organization.invite.requireApprovalTip')}</p>
                          <div className="org-preview-fields">
                            <div className="org-form-item" style={{ marginBottom: '0' }}>
                              <label className="org-form-label" htmlFor="join-request-role">{t(locale, 'organization.invite.requestRole')}</label>
                              <select id="join-request-role" className="org-select" aria-label={t(locale, 'organization.invite.requestRole')} value={requestRole} onChange={(event) => setRequestRole(event.target.value as 'admin' | 'editor' | 'viewer')}>
                                {roleOptions.map(([value, labelKey]) => <option key={value} value={value}>{t(locale, labelKey)}</option>)}
                              </select>
                            </div>
                            <div className="org-form-item" style={{ marginBottom: '0' }}>
                              <label className="org-form-label" htmlFor="join-request-note">{t(locale, 'organization.invite.applicationNote')}</label>
                              <textarea id="join-request-note" className="org-textarea" rows={2} maxLength={500} value={requestNote} onChange={(event) => setRequestNote(clampApplicationNote(event.target.value))} placeholder={t(locale, 'organization.invite.messagePlaceholder')} />
                            </div>
                          </div>
                        </>
                      ) : (
                        <p className="org-preview-info-desc">{t(locale, 'organization.invite.defaultRoleAfterJoin', { role: t(locale, 'organization.role.viewer') })}</p>
                      )}
                    </div>
                  )}
                </>
              ) : (
                <>
                  <div className="join-mode-pills">
                    <button type="button" className={'join-mode-pill' + (joinStep === 'invite' ? ' active' : '')} onClick={() => setJoinStep('invite')}>{t(locale, 'organization.join.byInviteCode')}</button>
                    <button type="button" className={'join-mode-pill' + (joinStep === 'search' ? ' active' : '')} onClick={openSearchTab}>{t(locale, 'organization.join.searchSpaces')}</button>
                  </div>
                  {joinStep === 'invite' ? (
                    <>
                      {joinPreviewError ? <div className="org-join-error"><IconInfoCircle size={20} /><span>{joinPreviewError}</span></div> : null}
                      <div className="org-form-item">
                        <label className="org-form-label" htmlFor="join-code">{t(locale, 'organization.inviteCode')}</label>
                        <p className="org-form-desc">{t(locale, 'organization.invite.inputDesc')}</p>
                        <input id="join-code" name="join-code" className="org-input" value={joinInputCode} maxLength={32} placeholder={t(locale, 'organization.inviteCodePlaceholder')} onChange={(event) => setJoinInputCode(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void doPreviewFromInput(); }} />
                        <p className="org-form-tip">{t(locale, 'organization.editor.inviteCodeTip')}</p>
                      </div>
                    </>
                  ) : (
                    <>
                      <div className="org-form-item">
                        <label className="org-form-label" htmlFor="join-search">{t(locale, 'organization.join.searchSpaces')}</label>
                        <p className="org-form-desc">{t(locale, 'organization.join.searchSpacesDesc')}</p>
                        <div style={{ position: 'relative' }}>
                          <input id="join-search" className="org-input" value={searchQuery} placeholder={t(locale, 'organization.join.searchSpacesPlaceholder')} onChange={(event) => onSearchQueryChange(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') runSearch(searchQuery.trim()); }} />
                          <span style={{ position: 'absolute', right: '10px', top: '8px', color: 'var(--org-text-placeholder)' }}><IconSearch /></span>
                        </div>
                      </div>
                      <div className="org-searchable-list">
                        {searchLoading ? (
                          <div className="org-empty-inline" style={{ textAlign: 'center' }}>{t(locale, 'common.loading')}</div>
                        ) : searchItems.length === 0 ? (
                          <div className="org-empty-inline" style={{ textAlign: 'center' }}>{t(locale, searchQuery ? 'organization.join.noSearchResult' : 'organization.join.noSearchableSpaces')}</div>
                        ) : searchItems.map((row) => (
                          <div key={strOf(row.id)} className={'org-searchable-row' + (searchRowFull(row) ? ' is-full' : '')} onClick={() => { if (!searchRowFull(row)) previewSearchableOrg(row); }}>
                            <div className="org-searchable-main">
                              <SpaceAvatar name={strOf(row.name)} avatar={row.avatar} size="small" />
                              <div className="org-searchable-info">
                                <span className="org-searchable-title" title={strOf(row.name)}>{strOf(row.name)}</span>
                                <span className="org-searchable-desc">{strOf(row.description) || t(locale, 'organization.noDescription')}</span>
                              </div>
                            </div>
                            <div className="org-searchable-meta">
                              <span className="org-searchable-meta-item" style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', fontSize: '12px', color: 'var(--org-text-secondary)' }}><IconUser size={12} />{numOf(row.member_limit) > 0 ? numOf(row.member_count) + '/' + numOf(row.member_limit) : numOf(row.member_count)}</span>
                              {row.require_approval === true ? <span className="org-tag warning">{t(locale, 'organization.invite.needApproval')}</span> : null}
                              {searchRowFull(row) ? <span className="org-tag">{t(locale, 'organization.join.memberLimitReached')}</span> : <button type="button" className="org-btn outline" style={{ minHeight: '26px', fontSize: '12px', padding: '0 10px' }} onClick={(event) => { event.stopPropagation(); previewSearchableOrg(row); }}>{t(locale, 'organization.invite.previewAction')}</button>}
                            </div>
                          </div>
                        ))}
                      </div>
                    </>
                  )}
                </>
              )}
            </div>
            <div className="org-join-footer">
              {joinPreview ? (
                <>
                  <button type="button" className="org-btn neutral" onClick={() => { setJoinPreview(null); if (!joinCode) setJoinStep('search'); }}>{!joinCode ? t(locale, 'organization.join.backToSearch') : t(locale, 'common.cancel')}</button>
                  {!previewIsAlreadyMember ? (
                    <button type="button" className="org-btn primary" disabled={joining} onClick={() => void confirmJoin()}>{previewJoinMode === 'request' ? t(locale, 'organization.invite.submitRequest') : t(locale, 'organization.invite.primaryJoin')}</button>
                  ) : null}
                </>
              ) : joinStep === 'invite' ? (
                <>
                  <button type="button" className="org-btn neutral" onClick={closeJoin}>{t(locale, 'common.cancel')}</button>
                  <button type="button" className="org-btn primary" disabled={joinPreviewLoading} onClick={() => void doPreviewFromInput()}>{t(locale, 'organization.invite.previewAction')}</button>
                </>
              ) : (
                <button type="button" className="org-btn neutral" onClick={closeJoin}>{t(locale, 'common.cancel')}</button>
              )}
            </div>
          </div>
        </div>
      ) : null}

      {/* Delete / leave confirm dialog. */}
      {confirmState ? (
        <div className="org-modal-overlay" onClick={() => setConfirmState(null)}>
          <div className="org-confirm-dialog" role="dialog" aria-label={t(locale, confirmState.kind === 'delete' ? 'organization.deleteConfirmTitle' : 'organization.leaveConfirmTitle')} onClick={(event) => event.stopPropagation()}>
            <div className="org-confirm-header"><IconInfoCircle /><span>{t(locale, confirmState.kind === 'delete' ? 'organization.deleteConfirmTitle' : 'organization.leaveConfirmTitle')}</span></div>
            <p className="org-confirm-message">{t(locale, confirmState.kind === 'delete' ? 'organization.deleteConfirmMessage' : 'organization.leaveConfirmMessage', { name: confirmState.org.name })}</p>
            <div className="org-confirm-actions">
              <button type="button" onClick={() => setConfirmState(null)}>{t(locale, 'common.cancel')}</button>
              <button type="button" className="confirm" onClick={() => void confirmLeaveOrDelete()}>{confirmState.kind === 'delete' ? t(locale, 'common.delete') : t(locale, 'organization.leave')}</button>
            </div>
          </div>
        </div>
      ) : null}

      {toast ? <div className={'org-toast ' + toast.tone} role="status">{toast.text}</div> : null}
    </main>
  );
}