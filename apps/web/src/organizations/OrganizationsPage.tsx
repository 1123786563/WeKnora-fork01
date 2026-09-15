// Shared-space list page ported from the Vue baseline
// frontend/src/views/organization/OrganizationList.vue (+ ListSpaceSidebar.vue,
// SpaceAvatar.vue and the invite-preview modal). Layout, section chips, card
// anatomy, empty state and the create / join flows mirror the Vue page while
// all server wiring keeps using @weknora/api-client identity.organizations.
import { useEffect, useMemo, useRef, useState } from 'react';
import { readReactPlatformState } from '../platform/legacy-session.ts';
import type { Organization, OrganizationJoinRequest, OrganizationMember, WeKnoraClient } from '@weknora/api-client';
import { formatMessage, isLocale, supportedLocales } from '@weknora/i18n';
import { Input, Select, Textarea } from '@weknora/ui';
import { clampApplicationNote, inviteJoinMode, requestedRoleOf } from './join.ts';
import { buildInviteLink, copyText, sharedResourceRow } from './settings-actions.ts';
import { organizationRoleLabel } from './summary.ts';
import './organizations.css';
import emptyIllustration from './empty-organizations.svg';

/* organizations.css migrated to inline utilities (Tailwind v4). The css file
 * keeps only the two @keyframes; every static rule became utilities. Recipes
 * shared by many nodes keep the markup readable, and conditional state colors
 * are mutually exclusive branches so two utilities of the same property never
 * compete on one element (utilities have equal specificity). */
const ORG_CARD = 'group relative box-border flex h-[136px] min-h-[136px] cursor-pointer flex-col overflow-hidden rounded-[8px] border border-[#e7e7ea] bg-surface px-[14px] py-[12px] shadow-[0_1px_3px_rgba(0,0,0,0.04)] [transition:border-color_.25s_ease,box-shadow_.25s_ease,transform_.2s_ease] hover:border-[rgba(7,192,95,0.5)] hover:shadow-[0_6px_20px_rgba(7,192,95,0.12)] before:pointer-events-none before:absolute before:top-0 before:right-0 before:z-0 before:h-[80px] before:w-[120px] before:bg-[radial-gradient(ellipse_60%_50%_at_100%_0%,rgba(7,192,95,0.06)_0%,transparent_70%)] before:content-[""]';
const ORG_SKEL_BLOCK = 'animate-[orgSkelPulse_1.4s_ease-in-out_infinite] rounded-[6px] bg-[linear-gradient(90deg,#f2f2f3_25%,#e9e9ec_37%,#f2f2f3_63%)] [background-size:400%_100%]';
const ORG_CARD_WRAP = 'grid grid-cols-1 gap-[12px] animate-[orgContentFadeIn_0.32s_ease-out] min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4 min-[1900px]:grid-cols-5 min-[2200px]:grid-cols-6';
const ORG_BTN = 'box-border inline-flex min-h-[32px] cursor-pointer items-center justify-center gap-[6px] rounded-[6px] border px-[16px] font-[inherit] text-[14px] font-medium [transition:all_.2s_ease] disabled:cursor-not-allowed disabled:opacity-55';
const ORG_BTN_PRIMARY = ORG_BTN + ' border-transparent bg-accent text-white shadow-[0_2px_8px_rgba(7,192,95,0.25)] hover:shadow-[0_4px_14px_rgba(7,192,95,0.35)]';
const ORG_BTN_OUTLINE = ORG_BTN + ' border-[rgba(7,192,95,0.5)] bg-surface text-accent hover:border-accent hover:bg-accent-wash';
const ORG_BTN_NEUTRAL = ORG_BTN + ' border-[#e7e7ea] bg-surface text-[rgba(23,26,29,0.92)] hover:border-[#c9c9cf]';
const ORG_FIELD = 'box-border w-full rounded-[6px] border border-[#e7e7ea] bg-surface px-[10px] py-[6px] font-[inherit] text-[14px] text-[rgba(23,26,29,0.92)] focus:border-accent focus:outline-none';
const ORG_MEMBER_ROW = 'flex items-center justify-between gap-[12px] border-b border-[#e7e7ea] py-[10px] last:border-b-0';
const ORG_MEMBER_COPY = 'flex min-w-0 flex-col gap-[2px]';
const ORG_ROW_ACTIONS = 'flex shrink-0 items-center gap-[8px]';
const ORG_FORM_ITEM = 'mb-[16px]';
const ORG_FORM_LABEL = 'mb-[4px] block text-[14px] font-medium text-[rgba(23,26,29,0.92)]';
const ORG_FORM_DESC = 'm-0 mb-[10px] text-[13px] leading-[1.5] text-[rgba(23,26,29,0.6)]';
const ORG_SECTION_TITLE = 'm-0 mb-[4px] text-[15px] font-semibold text-[rgba(23,26,29,0.92)]';
const ORG_SECTION_DESC = 'm-0 mb-[16px] text-[12px] leading-[1.5] text-[rgba(23,26,29,0.6)]';
const ORG_EMPTY_INLINE = 'py-[18px] text-[13px] text-[rgba(23,26,29,0.4)]';
const ORG_AVATAR_EMOJIS = ['🚀', '📁', '👥', '🏢', '💡', '📚', '🌟', '🔧', '📌', '🎯', '📂', '🔒', '🌐', '⚡', '🎨', '📊', '🤝', '💼', '📧', '🏠', '🔑', '📈', '✨', '📋', '🌍', '💬', '🔔', '📦', '🎉', '🌈'];
const ORG_MODAL_OVERLAY = 'fixed inset-0 z-[2000] flex items-center justify-center bg-[rgba(0,0,0,0.5)] p-[20px] backdrop-blur-[4px]';
const ORG_CLOSE_BTN = 'absolute right-[16px] top-[16px] z-[10] flex h-[32px] w-[32px] cursor-pointer items-center justify-center rounded-[8px] border-0 bg-transparent text-[rgba(23,26,29,0.6)] hover:bg-[#f3f3f5] hover:text-[rgba(23,26,29,0.92)]';
const FEATURE_BADGE_BASE = 'box-border inline-flex h-[20px] cursor-default items-center justify-center gap-[3px] rounded-[5px] px-[5px] text-[11px] font-medium [transition:background_.2s_ease]';
const FEATURE_BADGE_TONES: Record<string, string> = {
  'stat-member': 'bg-[rgba(100,116,139,0.08)] text-[rgba(23,26,29,0.6)] hover:bg-[rgba(100,116,139,0.12)]',
  'stat-kb': 'bg-accent-wash text-accent hover:bg-accent-soft',
  'stat-agent': 'bg-[rgba(124,77,255,0.08)] text-[#7c4dff] hover:bg-[rgba(124,77,255,0.12)]',
};
const RELATION_ROLE_TAG = 'inline-flex h-[22px] items-center gap-[4px] rounded-[6px] px-[6px] text-[12px] font-medium';
const RELATION_ROLE_TAG_TONES: Record<string, string> = {
  owner: 'bg-[rgba(124,77,255,0.1)] text-accent',
  admin: 'bg-accent-soft text-accent',
  editor: 'bg-accent-wash text-accent',
};
const ORG_TAG = 'inline-flex rounded-[4px] px-[8px] text-[12px] leading-[20px]';
const ORG_TAG_TONES: Record<string, string> = {
  warning: 'bg-[rgba(250,173,20,0.12)] text-[#faad14]',
  success: 'bg-accent-wash text-accent',
};

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
  <svg width={size} height={size} viewBox="0 0 20 20" fill="currentColor" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="shrink-0 text-[#7c4dff]">
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
function SpaceAvatar(props: { name: string; avatar?: unknown; size?: 'small' | 'medium' | 'large'; className?: string }) {
  const size = props.size ?? 'medium';
  const dimension = size === 'small' ? 22 : size === 'large' ? 48 : 32;
  const avatarText = strOf(props.avatar).trim();
  const isEmoji = avatarText.startsWith('emoji:') && avatarText.length > 6;
  const name = props.name?.trim() ?? '';
  const firstChar = name ? name.charAt(0) : '?';
  const letter = /[a-zA-Z]/.test(firstChar) ? firstChar.toUpperCase() : firstChar;
  const gradient = AVATAR_GRADIENTS[avatarHash(name) % AVATAR_GRADIENTS.length];
  // .space-avatar / .space-avatar-large rules → utilities (margin hookable via className).
  const className = 'flex items-center justify-center overflow-hidden relative rounded-[8px] shadow-[0_1px_3px_rgba(0,0,0,0.16)]' + (props.className ? ' ' + props.className : '');
  const style: Record<string, string> = isEmoji
    ? { background: 'linear-gradient(135deg, #f1f5f9 0%, #e2e8f0 100%)', width: dimension + 'px', height: dimension + 'px' }
    : { background: 'linear-gradient(135deg, ' + gradient[0] + ' 0%, ' + gradient[1] + ' 100%)', width: dimension + 'px', height: dimension + 'px' };
  return (
    <div className={className} style={style}>
      {isEmoji ? (
        <span style={{ fontSize: Math.round(dimension * 0.5) + 'px' }}>{avatarText.slice(6).trim()}</span>
      ) : (
        <>
          <svg className="absolute top-0 right-0 h-auto w-full text-[rgba(255,255,255,0.5)]" viewBox="0 0 56 40" width={dimension} height={Math.round(dimension * 40 / 56)} preserveAspectRatio="xMaxYMax meet" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
            <circle cx="10" cy="12" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
            <circle cx="28" cy="8" r="5" stroke="currentColor" strokeWidth="1.8" fill="none" opacity="0.7" />
            <circle cx="46" cy="14" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
            <path d="M14 13 L24 10 M32 10 L42 13" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" opacity="0.4" />
            <circle cx="28" cy="28" r="6" stroke="currentColor" strokeWidth="1.2" fill="none" opacity="0.35" />
            <path d="M28 14 L28 22 M20 18 L26 24 M36 18 L30 24" stroke="currentColor" strokeWidth="1" strokeLinecap="round" opacity="0.3" />
          </svg>
          <span className={'relative text-white font-semibold leading-none ' + (size === 'large' ? 'text-[20px]' : 'text-[12px]')} style={{ textShadow: '0 1px 2px ' + gradient[1] + '80, 0 0 8px ' + gradient[0] + '30' }}>{letter}</span>
        </>
      )}
    </div>
  );
}

/* Constellation card decoration, ported verbatim from OrganizationList.vue. */
function CardDecoration() {
  return (
    <div className="pointer-events-none absolute right-[14px] top-[8px] z-0 flex items-start justify-end text-[rgba(7,192,95,0.35)] [transition:color_.3s_ease] group-hover:text-[rgba(7,192,95,0.55)]">
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
    <div className={FEATURE_BADGE_BASE + ' ' + FEATURE_BADGE_TONES[props.tone]} title={props.title}>
      {props.tone === 'stat-member' ? <IconUser /> : props.tone === 'stat-kb' ? <IconFolder /> : <IconAgent />}
      <span>{props.count}</span>
    </div>
  );
}

function skeletonCard(key: string) {
  return (
    <div key={key} className={ORG_CARD + ' cursor-default'}>
      <div className="relative z-[2] mb-[6px] flex items-center justify-between">
        <div className="flex min-w-0 flex-1 items-center gap-[8px]">
          <div className={ORG_SKEL_BLOCK} style={{ width: '36px', height: '36px', borderRadius: '8px' }} />
          <div className={ORG_SKEL_BLOCK} style={{ width: '50%', height: '20px' }} />
        </div>
      </div>
      <div style={{ flex: 1, marginTop: '12px' }}>
        <div className={ORG_SKEL_BLOCK} style={{ width: '100%', height: '14px', marginBottom: '8px' }} />
        <div className={ORG_SKEL_BLOCK} style={{ width: '70%', height: '14px' }} />
      </div>
      <div className="relative z-[1] mt-auto flex items-center justify-between border-t-[0.5px] border-[#e7e7ea] pt-[6px]">
        <div className={ORG_SKEL_BLOCK} style={{ width: '60px', height: '22px' }} />
        <div className={ORG_SKEL_BLOCK} style={{ width: '60px', height: '22px' }} />
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
  const [formAvatar, setFormAvatar] = useState('');
  const [avatarPickerOpen, setAvatarPickerOpen] = useState(false);
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
      .catch((reason) => { if (active) setJoinPreviewError(errorText(reason, t(locale, 'organization.invite.previewFailed'))); })
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
    setFormAvatar(''); setAvatarPickerOpen(false);
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
    // Vue OrganizationList.vue:879 — messageless preview failures fall back to
    // previewFailed, not invalidCode (which is only the server's own copy).
    catch (reason) { setJoinPreviewError(errorText(reason, t(locale, 'organization.invite.previewFailed'))); }
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
      await organizationsApi.create({ name: formName.trim(), description: formDescription, ...(formAvatar ? { avatar: formAvatar } : {}) });
      setSettingsOpen(false); setFormName(''); setFormDescription('');
      setFormAvatar(''); setAvatarPickerOpen(false);
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
      <div key={'header-' + key} className="sticky top-0 z-[5] col-span-full flex cursor-pointer select-none items-center gap-[6px] rounded-[6px] bg-surface py-[6px] pr-[4px] pl-0 text-[13px] font-semibold leading-[20px] text-[rgba(23,26,29,0.6)] shadow-[0_-8px_0_0_#fff,0_4px_0_0_#fff] outline-none hover:text-[rgba(23,26,29,0.92)]" role="button" tabIndex={0}
        onClick={() => toggleSection(key)}
        onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); toggleSection(key); } }}>
        {key === 'created' ? <IconUsergroupAdd /> : <IconUsergroup />}
        <span>{t(locale, key === 'created' ? 'organization.createdByMe' : 'organization.joinedByMe')}</span>
        <span className="ml-[2px] rounded-[8px] bg-[#f3f3f5] px-[6px] text-[11px] font-medium leading-[16px] text-[rgba(23,26,29,0.6)]">{key === 'created' ? createdCount : joinedCount}</span>
        <span className="ml-[4px] opacity-70"><IconChevron direction={collapsed ? 'right' : 'down'} /></span>
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
      <div key={org.id || index} className={ORG_CARD} role="button" tabIndex={0}
        onClick={() => openSettingsModal(org)}
        onKeyDown={(event) => { if (event.key === 'Enter') openSettingsModal(org); }}>
        <CardDecoration />
        <div className="relative z-[2] mb-[6px] flex items-center justify-between">
          <div className="flex min-w-0 flex-1 items-center gap-[8px]">
            <div className="flex shrink-0"><SpaceAvatar name={org.name} avatar={org.avatar} size="small" /></div>
            <div className="flex min-w-0 flex-1 flex-col gap-[2px]"><span className="truncate text-[15px] font-semibold leading-[22px] tracking-[0.01em] text-[rgba(23,26,29,0.92)]" title={org.name}>{org.name}</span></div>
          </div>
          <div className={'relative flex h-[28px] w-[28px] shrink-0 cursor-pointer items-center justify-center rounded-[8px] opacity-0 [transition:all_.2s_ease] group-hover:opacity-60 hover:bg-[#f3f3f5] hover:opacity-100!' + (moreMenuOrgId === org.id ? ' bg-[#f3f3f5] opacity-100!' : '')}
            role="button" tabIndex={0} aria-label={t(locale, 'common.edit')}
            onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(moreMenuOrgId === org.id ? null : org.id); }}
            onKeyDown={(event) => { if (event.key === 'Enter') { event.stopPropagation(); setMoreMenuOrgId(moreMenuOrgId === org.id ? null : org.id); } }}>
            <IconMore />
            {moreMenuOrgId === org.id ? (
              <div className="absolute right-0 top-[32px] z-[30]">
                <div className="min-w-[148px] rounded-[10px] border border-[#e7e7ea] bg-surface p-[6px] shadow-[0_8px_24px_rgba(0,0,0,0.1)]" onClick={(event) => event.stopPropagation()}>
                  <div className="flex cursor-pointer items-center gap-[8px] whitespace-nowrap rounded-[6px] px-[10px] py-[8px] text-[13px] text-[rgba(23,26,29,0.92)] hover:bg-[#f3f3f5]" onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(null); openSettingsModal(org); }}>
                    <IconSetting /><span>{t(locale, 'organization.settings.editTitle')}</span>
                  </div>
                  {!owner ? (
                    <div className="flex cursor-pointer items-center gap-[8px] whitespace-nowrap rounded-[6px] px-[10px] py-[8px] text-[13px] text-[#d54941] hover:bg-[#f3f3f5]" onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(null); setConfirmState({ kind: 'leave', org }); }}>
                      <IconLogout /><span>{t(locale, 'organization.leave')}</span>
                    </div>
                  ) : canManageOrg ? (
                    // Vue: v-if="org.is_owner && canManageOrg" — deleting an
                    // owned space also requires the tenant admin+ role.
                    <div className="flex cursor-pointer items-center gap-[8px] whitespace-nowrap rounded-[6px] px-[10px] py-[8px] text-[13px] text-[#d54941] hover:bg-[#f3f3f5]" onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(null); setConfirmState({ kind: 'delete', org }); }}>
                      <IconDelete /><span>{t(locale, 'common.delete')}</span>
                    </div>
                  ) : null}
                </div>
              </div>
            ) : null}
          </div>
        </div>
        <div className="relative z-[1] mb-[6px] flex min-h-0 flex-1 flex-col gap-[6px] overflow-hidden">
          <div className="line-clamp-2 text-[12px] font-normal leading-[18px] text-[rgba(23,26,29,0.6)]">{description || t(locale, 'organization.noDescription')}</div>
        </div>
        <div className="relative z-[1] mt-auto flex items-center justify-between border-t-[0.5px] border-[#e7e7ea] pt-[6px]">
          <div className="flex min-w-0 flex-1 items-center gap-[6px]">
            <div className="flex items-center gap-[4px]">
              <FeatureBadge tone="stat-member" title={t(locale, 'organization.memberCount')} count={memberCount} />
              <FeatureBadge tone="stat-kb" title={t(locale, 'organization.invite.knowledgeBases')} count={shareCount} />
              <FeatureBadge tone="stat-agent" title={t(locale, 'organization.invite.agents')} count={agentShareCount} />
            </div>
            {pendingCount > 0 ? (
              <span className="inline-flex h-[22px] items-center whitespace-nowrap rounded-[6px] bg-[rgba(250,173,20,0.12)] px-[6px] text-[12px] font-medium text-[#faad14]" title={t(locale, 'organization.settings.pendingJoinRequestsBadge')}>{pendingCount} {t(locale, 'organization.settings.pendingReview')}</span>
            ) : null}
          </div>
          {showRelation ? (
            <div className="flex shrink-0 items-center">
              <div className={RELATION_ROLE_TAG + ' ' + (RELATION_ROLE_TAG_TONES[relationClass] ?? 'bg-[rgba(107,114,128,0.08)] text-[rgba(23,26,29,0.6)]')}>
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

  // .wk-page .wk-org-page → utilities: the org page cancels the shared
  // page gutter (max-width/padding !important) and fills the shell height.
  return (
    <main className="box-border h-full max-w-none! overflow-hidden p-0!">
      <div className="relative flex h-full min-h-0 w-full m-0">
        <aside className="box-border flex w-[56px] shrink-0 flex-col items-center gap-1 border-r border-[#e7e7ea] px-0 pt-3 pb-1.5" aria-label={t(locale, 'organization.title')}>
          {/* Vue ListSpaceSidebar collapsed-strip tooltips carry the live
              counts (tooltipText(name, count) → "name (count)").
              "is-active" stays as a state hook (styles are utilities). */}
          <button type="button" className={'flex w-[46px] shrink-0 cursor-pointer flex-col items-center justify-center gap-0.5 rounded-[8px] border-0 px-0 pt-[5px] pb-0.5 font-[inherit] text-[11px] ' + (selection === 'all' ? 'is-active text-accent' : 'text-[rgba(23,26,29,0.6)] hover:text-[rgba(23,26,29,0.92)]')} title={t(locale, 'common.all') + ' (' + organizations.length + ')'} onClick={() => setSelection('all')}>
            <span className={'flex h-[30px] w-[30px] box-border items-center justify-center rounded-[8px] border text-inherit ' + (selection === 'all' ? 'border-[rgba(7,192,95,0.35)] bg-accent-wash' : 'border-transparent')}><IconLayers /></span>
            <span>{t(locale, 'common.all')}</span>
          </button>
          <button type="button" className={'flex w-[46px] shrink-0 cursor-pointer flex-col items-center justify-center gap-0.5 rounded-[8px] border-0 px-0 pt-[5px] pb-0.5 font-[inherit] text-[11px] ' + (selection === 'created' ? 'is-active text-accent' : 'text-[rgba(23,26,29,0.6)] hover:text-[rgba(23,26,29,0.92)]')} title={t(locale, 'organization.createdByMe') + ' (' + createdCount + ')'} onClick={() => setSelection('created')}>
            <span className={'flex h-[30px] w-[30px] box-border items-center justify-center rounded-[8px] border text-inherit ' + (selection === 'created' ? 'border-[rgba(7,192,95,0.35)] bg-accent-wash' : 'border-transparent')}><IconUsergroupAdd /></span>
            <span>{t(locale, 'organization.createdByMe')}</span>
          </button>
          <button type="button" className={'flex w-[46px] shrink-0 cursor-pointer flex-col items-center justify-center gap-0.5 rounded-[8px] border-0 px-0 pt-[5px] pb-0.5 font-[inherit] text-[11px] ' + (selection === 'joined' ? 'is-active text-accent' : 'text-[rgba(23,26,29,0.6)] hover:text-[rgba(23,26,29,0.92)]')} title={t(locale, 'organization.joinedByMe') + ' (' + joinedCount + ')'} onClick={() => setSelection('joined')}>
            <span className={'flex h-[30px] w-[30px] box-border items-center justify-center rounded-[8px] border text-inherit ' + (selection === 'joined' ? 'border-[rgba(7,192,95,0.35)] bg-accent-wash' : 'border-transparent')}><IconUsergroup /></span>
            <span>{t(locale, 'organization.joinedByMe')}</span>
          </button>
        </aside>
        <div className="box-border flex flex-1 flex-col min-w-0 px-[28px] pt-[20px] pb-0">
          <header className="mb-[16px] flex shrink-0 items-center justify-between">
            <div className="flex flex-col gap-[4px]">
              <div className="flex items-center gap-[8px]">
                <h2 className="m-0 text-[24px] font-semibold leading-[32px] text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.title')}</h2>
                <div className="flex shrink-0 items-center gap-[8px]">
                  {/* Vue header-actions: disabled={!canManageOrg} with the
                      joinOrg/createOrg tooltip swapped for the rbac tip. */}
                  <button type="button" className="flex h-[28px] w-[28px] min-w-[28px] box-border cursor-pointer items-center justify-center rounded-[6px] border border-[#e7e7ea] bg-[#f3f3f5] p-0 text-[rgba(23,26,29,0.6)] shadow-[inset_0_1px_0_rgba(255,255,255,0.72)] [transition:background_.2s,border-color_.2s,color_.2s] hover:text-[rgba(23,26,29,0.92)] [&_svg]:text-accent" aria-label={t(locale, 'organization.joinOrg')} title={canManageOrg ? t(locale, 'organization.joinOrg') : writeGuardTitle} disabled={!canManageOrg} onClick={openJoinModal}>
                    <IconEnter />
                  </button>
                  <button type="button" className="flex h-[28px] w-[28px] min-w-[28px] box-border cursor-pointer items-center justify-center rounded-[6px] border border-[#e7e7ea] bg-[#f3f3f5] p-0 text-[rgba(23,26,29,0.6)] shadow-[inset_0_1px_0_rgba(255,255,255,0.72)] [transition:background_.2s,border-color_.2s,color_.2s] hover:text-[rgba(23,26,29,0.92)] [&_svg]:text-accent" aria-label={t(locale, 'organization.createOrg')} title={canManageOrg ? t(locale, 'organization.createOrg') : writeGuardTitle} disabled={!canManageOrg} onClick={openCreateModal}>
                    <IconOrgCreate />
                  </button>
                </div>
              </div>
              <p className="m-0 text-[14px] font-normal leading-[20px] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.subtitle')}</p>
            </div>
          </header>
          <div className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto py-[8px]">
            {loading && organizations.length === 0 ? (
              <div className={ORG_CARD_WRAP}>{[1, 2, 3, 4].map((n) => skeletonCard('skel-' + n))}</div>
            ) : ordered.length === 0 ? (
              <div className="flex flex-1 flex-col items-center justify-center px-[20px] py-[60px]">
                <img className="h-[162px] w-[162px] mb-[20px]" src={emptyIllustration} alt="" />
                <span className="text-[16px] font-semibold leading-[26px] text-[rgba(23,26,29,0.4)] mb-[8px]">{emptyTitle}</span>
                <span className="m-0 text-[14px] font-normal leading-[22px] text-[rgba(23,26,29,0.4)]">{emptyDesc}</span>
                <div className="flex items-center gap-[12px] mt-[20px]">
                  <button type="button" className={ORG_BTN_OUTLINE} title={canManageOrg ? undefined : writeGuardTitle} disabled={!canManageOrg} onClick={openJoinModal}>
                    <IconEnter />{t(locale, 'organization.joinOrg')}
                  </button>
                  <button type="button" className={ORG_BTN_PRIMARY} title={canManageOrg ? undefined : writeGuardTitle} disabled={!canManageOrg} onClick={openCreateModal}>
                    <IconOrgCreate />{t(locale, 'organization.createOrg')}
                  </button>
                </div>
              </div>
            ) : (
              <div className={ORG_CARD_WRAP}>{cardRows}</div>
            )}
          </div>
        </div>
      </div>

      {/* Create / edit settings modal. */}
      {settingsOpen ? (
        <div className={ORG_MODAL_OVERLAY} onClick={closeSettings}>
          <div className="relative box-border flex h-[85vh] w-[90vw] max-w-[1100px] flex-col overflow-hidden rounded-[12px] bg-surface shadow-[0_8px_32px_rgba(0,0,0,0.12)]" role="dialog" aria-label={t(locale, settingsMode === 'create' ? 'organization.createOrg' : 'organization.settings.editTitle')} onClick={(event) => event.stopPropagation()}>
            <button type="button" className={ORG_CLOSE_BTN} aria-label={t(locale, 'common.close')} onClick={closeSettings}><IconClose /></button>
            <div className="flex min-h-0 flex-1">
              {settingsMode === 'create' || (settingsMode === 'edit' && settingsOrg) ? (
                <nav className="box-border w-[208px] shrink-0 overflow-y-auto border-r border-[#e7e7ea] bg-[#f9f9f9] px-2 py-2">
                  <h2 className="m-0 mb-[12px] ml-[6px] text-[16px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, settingsMode === 'create' ? 'organization.createOrg' : 'organization.settings.editTitle')}</h2>
                  {(settingsMode === 'create' ? [
                    ['basic', 'organization.editor.navBasic'],
                    ['permissions', 'organization.editor.navPermissions'],
                  ] : [
                    ['basic', 'organization.editor.navBasic'],
                    ['members', 'organization.members.listTitle'],
                    ['requests', 'organization.joinRequests.listTitle'],
                    ['shares', 'organization.sharedResources.kbListTitle'],
                    ['invite', 'organization.settings.inviteLink'],
                  ] as Array<[string, string]>).map(([key, labelKey]) => (
                    <button key={key} type="button" className={'flex w-full cursor-pointer items-center gap-[8px] rounded-[8px] border-0 px-[10px] py-[9px] text-left font-[inherit] text-[13px] ' + (settingsSection === key ? 'bg-accent-wash font-medium text-accent' : 'bg-transparent text-[rgba(23,26,29,0.6)] hover:bg-[#f3f3f5]')} onClick={() => setSettingsSection(key)}>{t(locale, labelKey)}</button>
                  ))}
                </nav>
              ) : null}
              <div className="flex min-w-0 flex-1 flex-col">
                <div className={'min-h-0 flex-1 overflow-y-auto ' + (settingsMode === 'create' ? 'px-[40px] py-[28px]' : 'px-[24px] py-[20px]')}>
                  {settingsMode === 'create' && settingsSection === 'basic' ? (
                    <form onSubmit={submitCreate}>
                      <h2 className="m-0 mb-2 text-[16px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.editor.basicTitle')}</h2>
                      <p className="m-0 mb-6 text-[14px] leading-[22px] text-[rgba(23,26,29,0.4)]">{t(locale, 'organization.editor.basicDesc')}</p>
                      <div className="mb-6 grid grid-cols-[minmax(0,1fr)_minmax(280px,446px)] items-start gap-6">
                        <div><label className={ORG_FORM_LABEL} htmlFor="organization-name">{t(locale, 'organization.name')} *</label><p className={ORG_FORM_DESC}>{t(locale, 'organization.editor.nameTip')}</p></div>
                        <div className="flex min-w-0 items-center gap-3"><div className="relative flex shrink-0 flex-col items-center gap-1"><button type="button" className="cursor-pointer rounded-lg border-0 bg-transparent p-0" aria-label={t(locale, 'organization.avatarPickerHint')} onClick={() => setAvatarPickerOpen((open) => !open)}><SpaceAvatar name={formName || '?'} avatar={formAvatar} size="medium" /></button><span className="text-[12px] text-[rgba(23,26,29,0.4)]">{t(locale, 'organization.avatar')}</span>{avatarPickerOpen ? <div className="absolute left-0 top-[64px] z-20 grid w-[220px] grid-cols-6 gap-1 rounded-lg border border-[#e7e7ea] bg-surface p-2 shadow-[0_8px_24px_rgba(0,0,0,0.12)]">{ORG_AVATAR_EMOJIS.map((emoji) => <button type="button" key={emoji} className="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-base hover:bg-[#f3f3f5]" aria-label={emoji} onClick={() => { setFormAvatar('emoji:' + emoji); setAvatarPickerOpen(false); }}>{emoji}</button>)}{formAvatar ? <button type="button" className="col-span-6 border-0 bg-transparent py-1 text-xs text-muted hover:bg-[#f3f3f5]" onClick={() => { setFormAvatar(''); setAvatarPickerOpen(false); }}>{t(locale, 'organization.avatarClear')}</button> : null}</div> : null}</div><Input id="organization-name" name="organization-name" className={ORG_FIELD + ' min-h-[34px]'} value={formName} onChange={(event) => setFormName(event.target.value)} required placeholder={t(locale, 'organization.namePlaceholder')} /></div>
                      </div>
                      <div className="mb-6 grid grid-cols-[minmax(0,1fr)_minmax(280px,446px)] items-start gap-6">
                        <div><label className={ORG_FORM_LABEL} htmlFor="organization-description">{t(locale, 'organization.description')}</label><p className={ORG_FORM_DESC}>{t(locale, 'organization.editor.descriptionTip')}</p></div>
                        <Textarea id="organization-description" name="organization-description" className={ORG_FIELD + ' min-h-[72px] resize-y'} rows={3} value={formDescription} onChange={(event) => setFormDescription(event.target.value)} placeholder={t(locale, 'organization.descriptionPlaceholder')} />
                      </div>
                      <button type="submit" className={ORG_BTN_PRIMARY + ' mt-4'} disabled={saving}>{t(locale, 'organization.createOrg')}</button>
                    </form>
                  ) : settingsMode === 'create' ? (
                    <section>
                      <h2 className="m-0 mb-2 text-[16px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.editor.permissionsTitle')}</h2>
                      <p className="m-0 mb-6 text-[14px] leading-[22px] text-[rgba(23,26,29,0.4)]">{t(locale, 'organization.editor.permissionsDesc')}</p>
                      <div className="grid gap-4">
                        {(['admin', 'editor', 'viewer'] as const).map((roleKey) => <div key={roleKey} className="rounded-lg border border-[#e7e7ea] bg-[#f9f9f9] p-4"><strong className="text-[15px]">{t(locale, 'organization.role.' + roleKey)}</strong><p className="m-0 mt-2 text-[13px] text-[rgba(23,26,29,0.6)]">{t(locale, roleKey === 'admin' ? 'organization.editor.fullAccess' : roleKey === 'editor' ? 'organization.editor.editAccess' : 'organization.editor.viewAccess')}</p></div>)}
                      </div>
                    </section>
                  ) : settingsSection === 'basic' ? (
                    <>
                      <form onSubmit={submitBasic}>
                        <h2 className={ORG_SECTION_TITLE}>{t(locale, 'organization.editor.basicTitle')}</h2>
                        <p className={ORG_SECTION_DESC}>{t(locale, 'organization.editor.basicDesc')}</p>
                        <div className={ORG_FORM_ITEM}>
                          <label className={ORG_FORM_LABEL} htmlFor="organization-name">{t(locale, 'organization.name')} *</label>
                          <Input id="organization-name" name="organization-name" className={ORG_FIELD + ' min-h-[34px]'} value={formName} onChange={(event) => setFormName(event.target.value)} required />
                        </div>
                        <div className={ORG_FORM_ITEM}>
                          <label className={ORG_FORM_LABEL} htmlFor="organization-description">{t(locale, 'organization.description')}</label>
                          <Textarea id="organization-description" name="organization-description" className={ORG_FIELD + ' min-h-[72px] resize-y'} rows={3} value={formDescription} onChange={(event) => setFormDescription(event.target.value)} />
                        </div>
                        <button type="submit" className={ORG_BTN_PRIMARY} disabled={saving}>{t(locale, 'common.save')}</button>
                      </form>
                      <form onSubmit={submitUpgradeRequest} style={{ marginTop: '24px', borderTop: '1px dashed #e7e7ea', paddingTop: '16px' }}>
                        <h3 className={ORG_SECTION_TITLE}>{t(locale, 'organization.upgrade.requestUpgrade')}</h3>
                        <div className={ORG_FORM_ITEM}>
                          <label className={ORG_FORM_LABEL} htmlFor="upgrade-role">{t(locale, 'organization.upgrade.selectRole')}</label>
                          <Select id="upgrade-role" className={ORG_FIELD + ' min-h-[34px]'} value={upgradeRole} onChange={(event) => setUpgradeRole(event.target.value as 'admin' | 'editor' | 'viewer')}>
                            {roleOptions.map(([value, labelKey]) => <option key={value} value={value}>{t(locale, labelKey)}</option>)}
                          </Select>
                        </div>
                        <div className={ORG_FORM_ITEM}>
                          <label className={ORG_FORM_LABEL} htmlFor="upgrade-note">{t(locale, 'organization.upgrade.reason')}</label>
                          <Textarea id="upgrade-note" className={ORG_FIELD + ' min-h-[72px] resize-y'} rows={2} maxLength={500} value={upgradeNote} onChange={(event) => setUpgradeNote(clampApplicationNote(event.target.value))} placeholder={t(locale, 'organization.upgrade.reasonPlaceholder')} />
                        </div>
                        <button type="submit" className={ORG_BTN_OUTLINE}>{t(locale, 'organization.upgrade.submitBtn')}</button>
                      </form>
                    </>
                  ) : settingsSection === 'members' ? (
                    <>
                      <h2 className={ORG_SECTION_TITLE}>{t(locale, 'organization.members.listTitle')}</h2>
                      <p className={ORG_SECTION_DESC}>{t(locale, 'organization.settings.membersDesc')}</p>
                      {members.length === 0 ? <p className={ORG_EMPTY_INLINE}>{t(locale, 'organization.noMembers')}</p> : members.map((member) => (
                        <div key={member.id} className={ORG_MEMBER_ROW}>
                          <div className={ORG_MEMBER_COPY}>
                            <strong className="text-[14px] font-semibold text-[rgba(23,26,29,0.92)]">{member.tenant_name ?? member.username}</strong>
                            <span className="text-[12px] text-[rgba(23,26,29,0.6)]">{member.email} · {t(locale, 'organization.role.' + member.role)}</span>
                          </div>
                          <div className={ORG_ROW_ACTIONS}>
                            <Select className={ORG_FIELD + ' min-h-[30px] w-[116px]!'} aria-label={t(locale, 'organization.members.columns.role')} value={member.role} onChange={(event) => void updateMemberRole(member, event.target.value as 'admin' | 'editor' | 'viewer')}>
                              {roleOptions.map(([value, labelKey]) => <option key={value} value={value}>{t(locale, labelKey)}</option>)}
                            </Select>
                            <button type="button" className={ORG_BTN_NEUTRAL} onClick={() => void removeMember(member)}>{t(locale, 'common.remove')}</button>
                          </div>
                        </div>
                      ))}
                    </>
                  ) : settingsSection === 'requests' ? (
                    <>
                      <h2 className={ORG_SECTION_TITLE}>{t(locale, 'organization.joinRequests.listTitle')}</h2>
                      {requests.filter((request) => request.status === 'pending').length === 0 ? <p className={ORG_EMPTY_INLINE}>{t(locale, 'organization.settings.noPendingRequests')}</p> : requests.filter((request) => request.status === 'pending').map((request) => (
                        <div key={request.id} className={ORG_MEMBER_ROW}>
                          <div className={ORG_MEMBER_COPY}>
                            <strong className="text-[14px] font-semibold text-[rgba(23,26,29,0.92)]">{request.username}</strong>
                            <span className="text-[12px] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.joinRequests.columns.requestedRole')}: {t(locale, 'organization.role.' + request.requested_role)}</span>
                          </div>
                          <div className={ORG_ROW_ACTIONS}>
                            <button type="button" className={ORG_BTN_OUTLINE} onClick={() => void reviewRequest(request, true)}>{t(locale, 'organization.settings.approve')}</button>
                            <button type="button" className={ORG_BTN_NEUTRAL} onClick={() => void reviewRequest(request, false)}>{t(locale, 'organization.settings.reject')}</button>
                          </div>
                        </div>
                      ))}
                    </>
                  ) : settingsSection === 'shares' ? (
                    <>
                      <h2 className={ORG_SECTION_TITLE}>{t(locale, 'organization.sharedResources.kbListTitle')}</h2>
                      {sharedResources.length === 0 ? <p className={ORG_EMPTY_INLINE}>{t(locale, 'organization.settings.noSharedKB')}</p> : sharedResources.map((resource, index) => {
                        const row = sharedResourceRow(resource);
                        return (
                          <div key={row.shareId || index} className={ORG_MEMBER_ROW}>
                            <div className={ORG_MEMBER_COPY}>
                              <strong className="text-[14px] font-semibold text-[rgba(23,26,29,0.92)]">{row.name}</strong>
                              <span className="text-[12px] text-[rgba(23,26,29,0.6)]">{row.permission || t(locale, 'organization.sharedResources.columns.permission')}</span>
                            </div>
                            {row.canUnshare ? (
                              <div className={ORG_ROW_ACTIONS}>
                                <button type="button" className={ORG_BTN_NEUTRAL} onClick={() => void unshareKnowledgeBase(row)}>{t(locale, 'organization.share.unshareAction')}</button>
                              </div>
                            ) : null}
                          </div>
                        );
                      })}
                    </>
                  ) : (
                    <>
                      <h2 className={ORG_SECTION_TITLE}>{t(locale, 'organization.settings.inviteLink')}</h2>
                      <p className={ORG_SECTION_DESC}>{t(locale, 'organization.settings.inviteMembersDesc')}</p>
                      <div className={ORG_ROW_ACTIONS} style={{ marginBottom: '12px' }}>
                        <button type="button" className={ORG_BTN_PRIMARY} onClick={() => void generateInviteLink()}>{t(locale, 'organization.settings.inviteMembers')}</button>
                      </div>
                      {inviteLink ? (
                        <div className="flex flex-col items-start gap-[8px] rounded-[8px] border border-[#e7e7ea] bg-[#f3f3f5] p-[12px]">
                          <code className="text-[13px] text-[rgba(23,26,29,0.92)] [overflow-wrap:anywhere]">{inviteLink}</code>
                          <button type="button" className={ORG_BTN_OUTLINE} onClick={() => { void copyText(inviteLink).then((copied) => { if (copied) showToast('success', t(locale, 'common.copied')); }); }}>{t(locale, 'common.copy')}</button>
                        </div>
                      ) : null}
                    </>
                  )}
                </div>
                <div className="flex justify-end gap-[12px] border-t border-[#e7e7ea] px-[24px] pt-[12px] pb-[16px]">
                  <button type="button" className={ORG_BTN_NEUTRAL} onClick={closeSettings}>{t(locale, 'common.cancel')}</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {/* Join modal (invite code / search / preview). */}
      {joinOpen ? (
        <div className={ORG_MODAL_OVERLAY} onClick={closeJoin}>
          <div className={'relative box-border flex w-full flex-col overflow-hidden rounded-[12px] bg-surface shadow-[0_8px_32px_rgba(0,0,0,0.12)] max-h-[90vh] ' + (!joinPreview && !joinPreviewLoading && joinStep === 'search' ? 'max-w-[560px]' : 'max-w-[480px]')} role="dialog" aria-label={t(locale, joinPreview ? 'organization.invite.previewTitle' : 'organization.joinOrg')} onClick={(event) => event.stopPropagation()}>
            <button type="button" className={ORG_CLOSE_BTN} aria-label={t(locale, 'common.close')} onClick={closeJoin}><IconClose /></button>
            <div className="flex shrink-0 items-center justify-between gap-[12px] border-b border-[#e7e7ea] py-[16px] pl-[20px] pr-[48px]">
              {joinPreview && !joinCode ? (
                <button type="button" className="flex h-[32px] w-[32px] shrink-0 cursor-pointer items-center justify-center rounded-[8px] border-0 bg-transparent text-[rgba(23,26,29,0.6)] hover:bg-[#f3f3f5] hover:text-accent" aria-label={t(locale, 'organization.join.backToSearch')} onClick={() => { setJoinPreview(null); setJoinStep('search'); }}><IconBack /></button>
              ) : null}
              <h2 className="m-0 min-w-0 flex-1 text-[16px] font-semibold text-[rgba(23,26,29,0.92)]">{joinPreview ? t(locale, 'organization.invite.previewTitle') : t(locale, 'organization.joinOrg')}</h2>
            </div>
            <div className="max-h-[calc(90vh-120px)] min-h-0 overflow-x-hidden overflow-y-auto px-[24px] pt-[20px]">
              {joinPreviewLoading ? (
                <div className={ORG_EMPTY_INLINE} style={{ textAlign: 'center', padding: '48px 0' }}>{t(locale, 'organization.invite.loading')}</div>
              ) : joinPreview ? (
                <>
                  <div className="flex flex-col items-center pt-[8px] pb-[20px] text-center">
                    <SpaceAvatar name={previewName} avatar={joinPreview.avatar} size="large" className="mb-[12px]" />
                    <h3 className="mx-0 mt-0 mb-[6px] max-w-full truncate text-[18px] font-semibold leading-[1.35] text-[rgba(23,26,29,0.92)]">{previewName}</h3>
                    <p className="mx-0 mt-0 mb-[14px] line-clamp-2 max-w-[360px] text-[13px] leading-[1.5] text-[rgba(23,26,29,0.6)]">{previewDescription || t(locale, 'organization.noDescription')}</p>
                    <div className="flex items-center justify-center gap-[4px] mb-[12px]">
                      <FeatureBadge tone="stat-member" title={t(locale, 'organization.memberCount')} count={numOf(joinPreview.member_count)} />
                      <FeatureBadge tone="stat-kb" title={t(locale, 'organization.invite.knowledgeBases')} count={numOf(joinPreview.share_count)} />
                      <FeatureBadge tone="stat-agent" title={t(locale, 'organization.invite.agents')} count={numOf(joinPreview.agent_share_count)} />
                    </div>
                  </div>
                  {previewIsAlreadyMember ? (
                    <div className="flex items-center justify-center gap-[8px] pt-[12px] pb-[4px] text-[14px] font-medium text-accent"><IconCheckCircle /><span>{t(locale, 'organization.invite.alreadyMember')}</span></div>
                  ) : (
                    <div style={{ borderTop: '1px solid #e7e7ea', paddingTop: '16px' }}>
                      <div className="flex min-h-[28px] items-center justify-between gap-[12px]">
                        <span className="text-[14px] font-medium text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.invite.approvalLabel')}</span>
                        <span className={ORG_TAG + ' ' + ORG_TAG_TONES[previewJoinMode === 'request' ? 'warning' : 'success']}>{t(locale, previewJoinMode === 'request' ? 'organization.invite.needApproval' : 'organization.invite.noApproval')}</span>
                      </div>
                      {previewJoinMode === 'request' ? (
                        <>
                          <p className="mx-0 mb-0 mt-[8px] text-[13px] leading-[1.5] text-[#faad14]">{t(locale, 'organization.invite.requireApprovalTip')}</p>
                          <div className="mt-[14px] flex flex-col gap-[12px] border-t border-dashed border-[#e7e7ea] pt-[14px]">
                            <div className={ORG_FORM_ITEM} style={{ marginBottom: '0' }}>
                              <label className={ORG_FORM_LABEL} htmlFor="join-request-role">{t(locale, 'organization.invite.requestRole')}</label>
                              <Select id="join-request-role" className={ORG_FIELD + ' min-h-[34px]'} aria-label={t(locale, 'organization.invite.requestRole')} value={requestRole} onChange={(event) => setRequestRole(event.target.value as 'admin' | 'editor' | 'viewer')}>
                                {roleOptions.map(([value, labelKey]) => <option key={value} value={value}>{t(locale, labelKey)}</option>)}
                              </Select>
                            </div>
                            <div className={ORG_FORM_ITEM} style={{ marginBottom: '0' }}>
                              <label className={ORG_FORM_LABEL} htmlFor="join-request-note">{t(locale, 'organization.invite.applicationNote')}</label>
                              <Textarea id="join-request-note" className={ORG_FIELD + ' min-h-[72px] resize-y'} rows={2} maxLength={500} value={requestNote} onChange={(event) => setRequestNote(clampApplicationNote(event.target.value))} placeholder={t(locale, 'organization.invite.messagePlaceholder')} />
                            </div>
                          </div>
                        </>
                      ) : (
                        <p className="mx-0 mb-0 mt-[8px] text-[13px] leading-[1.5] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.invite.defaultRoleAfterJoin', { role: t(locale, 'organization.role.viewer') })}</p>
                      )}
                    </div>
                  )}
                </>
              ) : (
                <>
                  <div className="mb-[20px] flex flex-wrap gap-[8px]">
                    <button type="button" className={'cursor-pointer rounded-[6px] border-0 px-[14px] py-[6px] font-[inherit] text-[13px] leading-[1.4] ' + (joinStep === 'invite' ? 'bg-accent-soft font-medium text-accent' : 'bg-[#f3f3f5] text-[rgba(23,26,29,0.6)] hover:text-accent')} onClick={() => setJoinStep('invite')}>{t(locale, 'organization.join.byInviteCode')}</button>
                    <button type="button" className={'cursor-pointer rounded-[6px] border-0 px-[14px] py-[6px] font-[inherit] text-[13px] leading-[1.4] ' + (joinStep === 'search' ? 'bg-accent-soft font-medium text-accent' : 'bg-[#f3f3f5] text-[rgba(23,26,29,0.6)] hover:text-accent')} onClick={openSearchTab}>{t(locale, 'organization.join.searchSpaces')}</button>
                  </div>
                  {joinStep === 'invite' ? (
                    <>
                      {joinPreviewError ? <div className="mb-[12px] flex items-center gap-[8px] rounded-[8px] bg-[rgba(213,73,65,0.08)] px-[12px] py-[10px] text-[13px] text-[#d54941]"><IconInfoCircle size={20} /><span>{joinPreviewError}</span></div> : null}
                      <div className={ORG_FORM_ITEM}>
                        <label className={ORG_FORM_LABEL} htmlFor="join-code">{t(locale, 'organization.inviteCode')}</label>
                        <p className={ORG_FORM_DESC}>{t(locale, 'organization.invite.inputDesc')}</p>
                        <Input id="join-code" name="join-code" className={ORG_FIELD + ' min-h-[34px]'} value={joinInputCode} maxLength={32} placeholder={t(locale, 'organization.inviteCodePlaceholder')} onChange={(event) => setJoinInputCode(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void doPreviewFromInput(); }} />
                        <p className="m-0 mt-[8px] text-[12px] leading-[1.45] text-[rgba(23,26,29,0.4)]">{t(locale, 'organization.editor.inviteCodeTip')}</p>
                      </div>
                    </>
                  ) : (
                    <>
                      <div className={ORG_FORM_ITEM}>
                        <label className={ORG_FORM_LABEL} htmlFor="join-search">{t(locale, 'organization.join.searchSpaces')}</label>
                        <p className={ORG_FORM_DESC}>{t(locale, 'organization.join.searchSpacesDesc')}</p>
                        <div style={{ position: 'relative' }}>
                          <Input id="join-search" className={ORG_FIELD + ' min-h-[34px]'} value={searchQuery} placeholder={t(locale, 'organization.join.searchSpacesPlaceholder')} onChange={(event) => onSearchQueryChange(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') runSearch(searchQuery.trim()); }} />
                          <span style={{ position: 'absolute', right: '10px', top: '8px', color: 'rgba(23, 26, 29, 0.4)' }}><IconSearch /></span>
                        </div>
                      </div>
                      <div className="mb-[16px] flex max-h-[320px] min-h-[120px] flex-col overflow-y-auto rounded-[10px] border border-[#e7e7ea] bg-surface">
                        {searchLoading ? (
                          <div className={ORG_EMPTY_INLINE} style={{ textAlign: 'center' }}>{t(locale, 'common.loading')}</div>
                        ) : searchItems.length === 0 ? (
                          <div className={ORG_EMPTY_INLINE} style={{ textAlign: 'center' }}>{t(locale, searchQuery ? 'organization.join.noSearchResult' : 'organization.join.noSearchableSpaces')}</div>
                        ) : searchItems.map((row) => (
                          <div key={strOf(row.id)} className="flex cursor-pointer items-center justify-between gap-[12px] border-b border-[#e7e7ea] px-[14px] py-[12px] last:border-b-0 hover:bg-[#f3f3f5]" onClick={() => { if (!searchRowFull(row)) previewSearchableOrg(row); }}>
                            <div className="flex min-w-0 flex-1 items-center gap-[10px]">
                              <SpaceAvatar name={strOf(row.name)} avatar={row.avatar} size="small" />
                              <div className="flex min-w-0 flex-col gap-[2px]">
                                <span className="truncate text-[14px] font-medium text-[rgba(23,26,29,0.92)]" title={strOf(row.name)}>{strOf(row.name)}</span>
                                <span className="truncate text-[12px] text-[rgba(23,26,29,0.6)]">{strOf(row.description) || t(locale, 'organization.noDescription')}</span>
                              </div>
                            </div>
                            <div className="flex shrink-0 items-center gap-[8px]">
                              <span style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', fontSize: '12px', color: 'rgba(23, 26, 29, 0.6)' }}><IconUser size={12} />{numOf(row.member_limit) > 0 ? numOf(row.member_count) + '/' + numOf(row.member_limit) : numOf(row.member_count)}</span>
                              {row.require_approval === true ? <span className={ORG_TAG + ' ' + ORG_TAG_TONES.warning}>{t(locale, 'organization.invite.needApproval')}</span> : null}
                              {searchRowFull(row) ? <span className={ORG_TAG}>{t(locale, 'organization.join.memberLimitReached')}</span> : <button type="button" className={ORG_BTN_OUTLINE} style={{ minHeight: '26px', fontSize: '12px', padding: '0 10px' }} onClick={(event) => { event.stopPropagation(); previewSearchableOrg(row); }}>{t(locale, 'organization.invite.previewAction')}</button>}
                            </div>
                          </div>
                        ))}
                      </div>
                    </>
                  )}
                </>
              )}
            </div>
            <div className="mt-[16px] flex shrink-0 justify-end gap-[12px] border-t border-[#e7e7ea] px-[24px] pt-[16px] pb-[20px]">
              {joinPreview ? (
                <>
                  <button type="button" className={ORG_BTN_NEUTRAL} onClick={() => { setJoinPreview(null); if (!joinCode) setJoinStep('search'); }}>{!joinCode ? t(locale, 'organization.join.backToSearch') : t(locale, 'common.cancel')}</button>
                  {!previewIsAlreadyMember ? (
                    <button type="button" className={ORG_BTN_PRIMARY} disabled={joining} onClick={() => void confirmJoin()}>{previewJoinMode === 'request' ? t(locale, 'organization.invite.submitRequest') : t(locale, 'organization.invite.primaryJoin')}</button>
                  ) : null}
                </>
              ) : joinStep === 'invite' ? (
                <>
                  <button type="button" className={ORG_BTN_NEUTRAL} onClick={closeJoin}>{t(locale, 'common.cancel')}</button>
                  <button type="button" className={ORG_BTN_PRIMARY} disabled={joinPreviewLoading} onClick={() => void doPreviewFromInput()}>{t(locale, 'organization.invite.previewAction')}</button>
                </>
              ) : (
                <button type="button" className={ORG_BTN_NEUTRAL} onClick={closeJoin}>{t(locale, 'common.cancel')}</button>
              )}
            </div>
          </div>
        </div>
      ) : null}

      {/* Delete / leave confirm dialog. */}
      {confirmState ? (
        <div className={ORG_MODAL_OVERLAY} onClick={() => setConfirmState(null)}>
          <div className="box-border w-full max-w-[400px] rounded-[8px] bg-surface p-[16px] shadow-[0_8px_32px_rgba(0,0,0,0.16)]" role="dialog" aria-label={t(locale, confirmState.kind === 'delete' ? 'organization.deleteConfirmTitle' : 'organization.leaveConfirmTitle')} onClick={(event) => event.stopPropagation()}>
            <div className="mb-[8px] flex items-center gap-[8px] text-[16px] font-semibold text-[rgba(23,26,29,0.92)] [&_svg]:text-accent"><IconInfoCircle /><span>{t(locale, confirmState.kind === 'delete' ? 'organization.deleteConfirmTitle' : 'organization.leaveConfirmTitle')}</span></div>
            <p className="mt-0 mr-0 mb-[20px] ml-[28px] inline-block text-[14px] leading-[22px] text-[rgba(23,26,29,0.6)]">{t(locale, confirmState.kind === 'delete' ? 'organization.deleteConfirmMessage' : 'organization.leaveConfirmMessage', { name: confirmState.org.name })}</p>
            <div className="flex justify-end gap-[40px] text-[14px]">
              <button type="button" className="cursor-pointer border-0 bg-transparent font-[inherit] text-[14px] text-[rgba(23,26,29,0.92)]" onClick={() => setConfirmState(null)}>{t(locale, 'common.cancel')}</button>
              <button type="button" className="cursor-pointer border-0 bg-transparent font-[inherit] text-[14px] text-[#d54941]" onClick={() => void confirmLeaveOrDelete()}>{confirmState.kind === 'delete' ? t(locale, 'common.delete') : t(locale, 'organization.leave')}</button>
            </div>
          </div>
        </div>
      ) : null}

      {toast ? <div className={'fixed left-1/2 top-[24px] z-[3000] flex items-center rounded-[8px] px-[18px] py-[10px] shadow-[0_6px_20px_rgba(0,0,0,0.18)] box-border max-w-[420px] bg-[rgba(23,26,29,0.86)] -translate-x-1/2 ' + (toast.tone === 'success' ? 'text-[#7bf2b6]' : 'text-[#ffb4ae]')} role="status">{toast.text}</div> : null}
    </main>
  );
}
