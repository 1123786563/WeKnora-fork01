import { useCallback, useEffect, useMemo, useState } from 'react';
import type { AgentConfiguration, WeKnoraClient } from '@weknora/api-client';
import { Status } from '@weknora/ui';
import { formatMessage, type Locale } from '@weknora/i18n';
import { usePreferredLocale } from '../locale.ts';
import { navigate } from '../platform/navigation.ts';
import {
  contextualGuideMessage,
  markContextualGuideDone,
  openContextualGuide,
} from '@weknora/views/guides/contextual-guides';
import './agents.css';
import { AgentEditorModal } from './AgentEditorModal.tsx';
import { makeEditorT } from './agent-editor.ts';
import {
  agentSectionLabelKey,
  avatarGradient,
  avatarLetter,
  buildAllViewRows,
  buildMineViewRows,
  buildPinIndex,
  buildSpaceViewRows,
  canManageAgent,
  cardActions,
  chatNavigationPath,
  cornerBadge,
  featureBadges,
  expertSourceId,
  hydratePinnedCards,
  kbScope,
  mcpScope,
  FEATURE_BADGE_TITLE_KEYS,
  opensEditorOnCardClick,
  readAgentRecents,
  readFavoriteIds,
  SECTION_ICON_KEYS,
  sectionize,
  sharedAgentsFromRecords,
  sharedCountByOrg,
  toggleFavoriteId,
  touchAgentRecent,
  writeAgentRecents,
  writeFavoriteIds,
  type AgentCardAction,
  type AgentCardModel,
  type AgentSectionView,
  type AgentViewer,
  type PinEntry,
  type SharedAgentSummary,
} from './list.ts';

type Translate = (key: string, values?: Record<string, string | number>) => string;

export interface AgentsPageData {
  ownAgents: AgentConfiguration[];
  disabledOwnIds: string[];
  organizations: Array<{ id: string; name: string; avatar?: string }>;
  sharedAgents: SharedAgentSummary[];
}

export interface AgentEditDeepLink {
  editId: string;
  section: string;
  highlight?: string;
  sourceTenantId?: string;
}

const AGENT_EDITOR_SECTIONS = new Set(['basic', 'prompts', 'model', 'conversation', 'knowledge', 'retrieval', 'websearch', 'tools', 'skills']);
const AGENT_HIGHLIGHT_SECTIONS: Record<string, string> = { summary_model: 'model', rerank_model: 'model', allowed_tools: 'tools' };

export function parseAgentEditDeepLink(search: string): AgentEditDeepLink | null {
  const params = new URLSearchParams(search);
  const editId = params.get('edit')?.trim();
  if (!editId) return null;
  const requestedSection = params.get('section')?.trim();
  const highlight = params.get('highlight')?.trim() || undefined;
  const section = requestedSection || (highlight ? AGENT_HIGHLIGHT_SECTIONS[highlight] : undefined) || 'basic';
  return {
    editId,
    section: section === 'sandbox' ? 'skills' : AGENT_EDITOR_SECTIONS.has(section) ? section : 'basic',
    highlight,
    sourceTenantId: params.get('sourceTenantId')?.trim() || undefined,
  };
}

export function resolveAgentEditTarget(ownAgents: AgentCardModel[], sharedAgents: AgentCardModel[], editId: string, sourceTenantId?: string): AgentCardModel | null {
  const own = ownAgents.find((agent) => agent.id === editId);
  if (own) return own;
  if (!sourceTenantId) return null;
  return sharedAgents.find((agent) => agent.id === editId && String(agent.sourceTenantId) === sourceTenantId) ?? null;
}

/** Parallel fetch behind the Vue chatResources.fetchAgentsForList + orgStore duo. */
export async function loadAgentsPageData(client: WeKnoraClient): Promise<AgentsPageData> {
  const [agents, organizations, sharedRecords] = await Promise.all([
    client.configuration.agents.listWithState({ creator: 'all' }),
    client.identity.organizations.list(),
    client.identity.organizations.agentShares.listShared(),
  ]);
  return {
    ownAgents: agents.items,
    disabledOwnIds: agents.disabledOwnAgentIds,
    organizations: organizations.items.map((org) => ({ id: String(org.id), name: String(org.name ?? '') })),
    sharedAgents: sharedAgentsFromRecords(sharedRecords),
  };
}

// --- icons (inline SVG; no TDesign in the React client) -----------------------

/* Icon path data lifted verbatim from the TDesign sprite the Vue client ships
 * (svg <symbol id="t-icon-*" viewBox="0 0 24 24">), so stroke geometry, 2px
 * stroke width and square line caps match the Vue baseline glyph-for-glyph.
 * The leading fill="transparent" hit-area paths are dropped; only the stroked
 * layer renders. 'star-filled' is TDesign's solid glyph (fill, no stroke). */
const ICON_PATHS: Record<string, React.ReactNode> = {
  layers: <path d="M4.5 6.125 12 3l7.5 3.125L12 9.25 4.5 6.125ZM3 11.5l9 3.877 9-3.877m0 6-9 3.877L3 17.5" stroke="currentColor" strokeWidth="2" />,
  star: <path d="m12 3.676 2.187 6.29 6.658.136-5.307 4.024 1.928 6.374L12 16.696 6.534 20.5l1.928-6.374-5.307-4.024 6.659-.136L12 3.676Z" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />,
  'star-filled': <path d="m12.001.63 2.903 8.35 8.839.181-7.045 5.341 2.56 8.462L12 17.914l-7.256 5.05 2.56-8.462L.26 9.161l8.839-.18L12 .63Z" fill="currentColor" />,
  history: <path d="M2.552 13c.5 4.777 4.539 8.5 9.448 8.5a9.5 9.5 0 0 0 0-19c-1.628 0-3.16.41-4.5 1.131A9.54 9.54 0 0 0 3.38 8M12 7v5l2.5 2.5m-12-11v5h5" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />,
  // TDesign system-sum: the Vue workspace rail glyph (four-petal sum shape).
  workspace: <g stroke="currentColor" strokeWidth="2"><path d="M15.244 15.404c-4.836 4.836-10.28 7.231-12.161 5.35-1.88-1.88.515-7.324 5.35-12.16 4.836-4.836 10.281-7.232 12.162-5.351 1.88 1.88-.515 7.325-5.351 12.161Z" /><path d="M8.434 15.404c4.836 4.836 10.28 7.231 12.161 5.35 1.88-1.88-.515-7.324-5.35-12.16C10.408 3.758 4.963 1.362 3.082 3.243c-1.88 1.88.515 7.325 5.351 12.161Z" /><path d="M12 12h.004v.004H12V12Z" /></g>,
  app: <g stroke="currentColor" strokeWidth="2"><path d="M3 3h7v7H3V3ZM14 14h7v7h-7v-7ZM3 14h7v7H3v-7Z" /><path d="M21.5 6.5a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z" /></g>,
  user: <g stroke="currentColor" strokeWidth="2" strokeLinecap="square"><path d="M16.5 7.5a4.5 4.5 0 1 1-9 0 4.5 4.5 0 0 1 9 0ZM20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2h16Z" /></g>,
  usergroup: <g stroke="currentColor" strokeWidth="2" strokeLinecap="square"><path d="M16 8a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM5 19a4 4 0 0 1 4-4h6a4 4 0 0 1 4 4v2H5v-2Z" /><path d="M7 4a4 4 0 1 0 0 8 6 6 0 0 0-6 6v3m22 0v-3a6 6 0 0 0-6-6 4 4 0 0 0 0-8" /></g>,
  'usergroup-add': <path d="M9 4a4 4 0 1 0 0 8 6 6 0 0 0-6 6v3m11-6h-2a4 4 0 0 0-4 4v2h6m5-13a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM20 15v3m0 0v3m0-3h-3m3 0h3" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />,
  share: <><circle cx="6" cy="12" r="2.5" /><circle cx="17" cy="6" r="2.5" /><circle cx="17" cy="18" r="2.5" /><path d="m8.3 10.8 6.4-3.6M8.3 13.2l6.4 3.6" /></>,
  // TDesign control-platform.svg: the Vue smart-reasoning avatar is a
  // faceted cube, not the generic radial control glyph.
  'control-platform': <path d="m12 12 8.5-4.5M12 12v9.5m0-9.5L3.5 7.5M12 2l9 5v10l-9 5-9-5V7l9-5Z" stroke="currentColor" strokeWidth="2" />,
  chat: <path d="M2.5 4h19v15h-15l-4 3V4Z" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />,
  folder: <path d="M2 3.5h7L11 6h11v14H2V3.5Z" stroke="currentColor" strokeWidth="2" />,
  extension: <path d="M9 4a2 2 0 1 1 4 0v1h6v6h1a2 2 0 1 1 0 4h-1v6h-4.535a3.501 3.501 0 0 0-6.93 0H3v-4.535a3.5 3.5 0 0 0 0-6.93V5h6V4Z" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />,
  // TDesign chat-bubble: the multi-turn badge is a ROUND speech bubble.
  'chat-bubble': <path d="M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12a9.966 9.966 0 0 0 2.737 6.874L3 22h9Z" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />,
  edit: <path d="m14.105 6.004-9.318 9.318L3.998 20l4.679-.79 9.317-9.317m-3.89-3.889 3.89 3.89m-3.89-3.89 3.058-3.057 3.889 3.89-3.057 3.056M17.994 9.893l3.057-3.057-3.89-3.889-3.057 3.057z" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />,
  'file-copy': <><path d="M14 2v6h6m-6-6h1l5 5v1m-6-6H7v16h13V8" stroke="currentColor" strokeWidth="2" /><path d="M3 6v16h11" stroke="currentColor" strokeWidth="2" strokeLinecap="square" /></>,
  poweroff: <g stroke="currentColor" strokeWidth="2" strokeLinecap="square"><path d="M7 5.125a8.5 8.5 0 1 0 10 0M12 3v8" /></g>,
  delete: <path d="M21 5H3m2 0h14l-.5 17h-13L5 5Zm3.5-3h7v3h-7V2ZM12 9v9" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />,
  close: <path d="M16.95 7.05 12 12m0 0-4.95 4.95M12 12l4.95 4.95M12 12 7.05 7.05" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />,
  'chevron-right': <path d="M9.5 17.5 15 12 9.5 6.5" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />,
  'chevron-down': <path d="M17.5 9.5 12 15 6.5 9.5" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />,
  'lock-on': <g stroke="currentColor" strokeWidth="2" strokeLinecap="square"><path d="M4.5 11h15v10h-15V11ZM7 7a5 5 0 0 1 10 0v4H7V7Z" /><path d="M10 16h4" /></g>,
  browse: <g stroke="currentColor" strokeWidth="2" strokeLinecap="square"><path d="M12 4C6.869 4 2.523 7.36 1.042 12c1.48 4.64 5.827 8 10.958 8 5.13 0 9.477-3.36 10.957-8C21.477 7.36 17.131 4 12 4Z" /><path d="M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z" /></g>,
  'edit-1': <path d="m13 6.5-10 10V21h4.5l10-10M13 6.5l4.5 4.5M13 6.5l4-4L21.5 7l-4 4" stroke="currentColor" strokeWidth="2" />,
};

function Icon({ name, size = 16 }: { name: string; size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none"
      aria-hidden="true" className="wk-agent-icon">{ICON_PATHS[name] ?? null}</svg>
  );
}

/** Three-star sparkles decoration, ported from the Vue header create button. */
function SparklesIcon({ size = 19 }: { size?: number }) {
  return (
    <svg className="animate-[wk-agent-twinkle_2s_ease-in-out_infinite]" width={size} height={size} viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M10 3L10.8 6.2C10.9 6.7 11.3 7.1 11.8 7.2L15 8L11.8 8.8C11.3 8.9 10.9 9.3 10.8 9.8L10 13L9.2 9.8C9.1 9.3 8.7 8.9 8.2 8.8L5 8L8.2 7.2C8.7 7.1 9.1 6.7 9.2 6.2L10 3Z" fill="currentColor" stroke="currentColor" strokeWidth="0.8" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M15.5 4L15.8 5.2C15.85 5.45 16.05 5.65 16.3 5.7L17.5 6L16.3 6.3C16.05 6.35 15.85 6.55 15.8 6.8L15.5 8L15.2 6.8C15.15 6.55 14.95 6.35 14.7 6.3L13.5 6L14.7 5.7C14.95 5.65 15.15 5.45 15.2 5.2L15.5 4Z" fill="currentColor" stroke="currentColor" strokeWidth="0.6" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M4.5 13L4.8 14.2C4.85 14.45 5.05 14.65 5.3 14.7L6.5 15L5.3 15.3C5.05 15.35 4.85 15.55 4.8 15.8L4.5 17L4.2 15.8C4.15 15.55 3.95 15.35 3.7 15.3L2.5 15L3.7 14.7C3.95 14.65 4.15 14.45 4.2 14.2L4.5 13Z" fill="currentColor" stroke="currentColor" strokeWidth="0.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

/** AgentAvatar.vue port: gradient tile + initial (emoji avatars render as-is). */
function AgentAvatar({ agent }: { agent: AgentCardModel }) {
  if (agent.is_builtin) {
    const smart = agent.config?.agent_mode === 'smart-reasoning';
    // Vue .builtin-avatar: gradient tint backgrounds, icon color = brand
    // (green) for the agent facet / brand-active for quick-answer.
    return (
      <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${smart ? 'bg-[linear-gradient(135deg,rgba(124,77,255,0.15)_0%,rgba(124,77,255,0.08)_100%)] text-[#07c05f]' : 'bg-[linear-gradient(135deg,rgba(7,192,95,0.15)_0%,rgba(7,192,95,0.08)_100%)] text-[#06b04d]'}`}>
        <Icon name={smart ? 'control-platform' : 'chat'} size={18} />
      </span>
    );
  }
  if (agent.avatar) return <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[#f3f3f3] text-[18px] leading-none">{agent.avatar}</span>;
  const gradient = avatarGradient(agent.name || '');
  return (
    <span
      className="flex h-[22px] w-[22px] shrink-0 items-center justify-center overflow-hidden rounded-[5px] text-[11px] font-semibold text-white"
      style={{ background: `linear-gradient(135deg, ${gradient.from} 0%, ${gradient.to} 100%)` }}
    >{avatarLetter(agent.name || '')}</span>
  );
}

const FEATURE_BADGE_ICONS: Record<string, string> = {
  modeNormal: 'chat',
  modePlain: 'chat',
  modeAgent: 'control-platform',
  knowledge: 'folder',
  mcp: 'extension',
  multiTurn: 'chat-bubble',
};

// agents.css .wk-agent-feature-badge.badge-* tones as utilities (static map).
// Colors read off the live Vue page (AgentList.vue): mode/knowledge/multiTurn
// icons ride the brand greens, webSearch the warning orange, mcp the error red.
const FEATURE_BADGE_TONES: Record<string, string> = {
  modeNormal: 'bg-[rgba(7,192,95,0.08)] text-[#06b04d]',
  knowledge: 'bg-[rgba(7,192,95,0.08)] text-[#06b04d]',
  modeAgent: 'bg-[rgba(124,77,255,0.08)] text-[#07c05f]',
  webSearch: 'bg-[rgba(255,152,0,0.08)] text-[#ed7b2f]',
  mcp: 'bg-[rgba(236,72,153,0.08)] text-[#e34d59]',
  multiTurn: 'bg-[rgba(59,130,246,0.08)] text-[#07c05f]',
};

function FeatureBadgeSvg({ badge }: { badge: string }) {
  if (badge === 'webSearch') {
    return (
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
        <circle cx="8" cy="8" r="6" stroke="currentColor" strokeWidth="1.2" fill="none" />
        <ellipse cx="8" cy="8" rx="2.5" ry="6" stroke="currentColor" strokeWidth="1.2" fill="none" />
        <line x1="2" y1="6" x2="14" y2="6" stroke="currentColor" strokeWidth="1.2" />
        <line x1="2" y1="10" x2="14" y2="10" stroke="currentColor" strokeWidth="1.2" />
      </svg>
    );
  }
  const modeBadge = badge === 'modeNormal' || badge === 'modeAgent' || badge === 'modePlain';
  return <Icon name={FEATURE_BADGE_ICONS[badge] ?? 'chat'} size={modeBadge ? 14 : 16} />;
}

// --- rail (ListSpaceSidebar.vue resource-mode strip + expanded panel) ----------

export interface AgentRailItem {
  key: string;
  label: string;
  icon: 'layers' | 'star' | 'history' | 'workspace' | 'space';
  avatarName?: string;
  count: number | undefined;
  active: boolean;
}

export function AgentRail({ t, items, onSelect }: { t: Translate; items: AgentRailItem[]; onSelect: (key: string) => void }) {
  // Vue .icon-strip: 56px column, 4px gaps, no x-padding; .icon-item-labeled
  // is a fixed 46px column (5px auto-inset) — active tile is the neutral
  // #f3f3f3 with brand-green content, hover is the same grey.
  return (
    <nav className="flex w-14 min-h-0 shrink-0 flex-col items-center gap-1 overflow-y-auto box-border shadow-[inset_-1px_0_0_#e7e7e7] pt-3 pb-[6px]" aria-label={t('agent.title')}>
      {items.map((item) => (
        <button
          key={item.key}
          type="button"
          data-space-key={item.key}
          className={`flex w-[46px] shrink-0 cursor-pointer flex-col items-center justify-center gap-[2px] rounded-lg border-none px-0 pt-[5px] pb-[2px] transition-[background] duration-150 ease-[ease] hover:bg-[#f3f3f3]${item.active ? ' bg-[#f3f3f3] text-[#07c05f]' : ' bg-transparent text-[rgba(0,0,0,0.6)] hover:text-[rgba(0,0,0,0.9)]'}`}
          aria-current={item.active ? 'true' : undefined}
          title={`${item.label}${item.count === undefined ? '' : ` (${item.count})`}`}
          onClick={() => onSelect(item.key)}
        >
          <span className="inline-flex items-center justify-center">
            {item.icon === 'space'
              ? <span className="inline-flex h-5 w-5 items-center justify-center overflow-hidden rounded-[5px] bg-[linear-gradient(135deg,#667eea_0%,#764ba2_100%)] text-[10px] font-semibold text-white">{avatarLetter(item.avatarName ?? item.label)}</span>
              : <Icon name={item.icon === 'workspace' ? 'workspace' : item.icon} size={16} />}
          </span>
          <span className="max-w-[52px] truncate text-[11px] leading-[1.25]">{item.label}</span>
        </button>
      ))}
    </nav>
  );
}

// --- section header -------------------------------------------------------------

const SECTION_SUBICONS: Partial<Record<string, string>> = { sharedEditable: 'edit-1', sharedReadonly: 'browse' };

function AgentSectionHeader({ section, t, viewer, collapsed, onToggle }: {
  section: AgentSectionView; t: Translate; viewer: AgentViewer; collapsed: boolean; onToggle: (key: string) => void;
}) {
  const subIcon = SECTION_SUBICONS[section.key];
  return (
    <button
      type="button"
      className="wk-agent-section-header"
      data-agent-section={section.key}
      aria-expanded={!collapsed}
      onClick={() => onToggle(section.key)}
    >
      <Icon name={SECTION_ICON_KEYS[section.key]} size={14} />
      {subIcon ? <span className="-ml-1 inline-flex opacity-75"><Icon name={subIcon} size={12} /></span> : null}
      <span>{t(agentSectionLabelKey(section.key, viewer))}</span>
      <span className="wk-agent-section-count">{section.count}</span>
      <span className="ml-1 inline-flex opacity-70" data-agent-section-toggle={section.key} aria-hidden="true">
        <Icon name={collapsed ? 'chevron-right' : 'chevron-down'} size={14} />
      </span>
    </button>
  );
}

// --- card -------------------------------------------------------------------------

const ACTION_META: Record<AgentCardAction, { icon: string; labelKey: string }> = {
  edit: { icon: 'edit', labelKey: 'common.edit' },
  copy: { icon: 'file-copy', labelKey: 'common.copy' },
  toggle: { icon: 'poweroff', labelKey: 'agent.disable' },
  delete: { icon: 'delete', labelKey: 'common.delete' },
};

export function AgentCard({ agent, t, viewer, favorited, menuOpen, onOpen, onToggleFavorite, onToggleMenu, onMenuAction }: {
  agent: AgentCardModel;
  t: Translate;
  viewer: AgentViewer;
  favorited: boolean;
  menuOpen: boolean;
  onOpen: (agent: AgentCardModel) => void;
  onToggleFavorite: (id: string) => void;
  onToggleMenu: (id: string | null) => void;
  onMenuAction: (action: AgentCardAction, agent: AgentCardModel) => void;
}) {
  const mode = agent.config?.agent_mode === 'smart-reasoning' ? 'agent' : 'normal';
  const badges = featureBadges(agent);
  const expertBadgeId = expertSourceId(agent.config);
  const actions = cardActions(agent, viewer);
  const badge = cornerBadge(agent, viewer.userId);
  const modeTitleKey = agent.config?.agent_mode === 'smart-reasoning' ? 'agent.mode.agent' : 'agent.mode.normal';
  const modeTone = mode === 'agent'
    ? 'bg-[linear-gradient(135deg,var(--wk-bg,#fff)_0%,rgba(124,77,255,0.04)_100%)] hover:bg-[linear-gradient(135deg,var(--wk-bg,#fff)_0%,rgba(124,77,255,0.08)_100%)] hover:shadow-[0_4px_12px_rgba(124,77,255,0.12)]'
    : 'bg-[linear-gradient(135deg,var(--wk-bg,#fff)_0%,rgba(7,192,95,0.04)_100%)] hover:bg-[linear-gradient(135deg,var(--wk-bg,#fff)_0%,rgba(7,192,95,0.08)_100%)] hover:shadow-[0_4px_12px_rgba(7,192,95,0.12)]';
  return (
    <article
      className={`wk-agent-card agent-mode-${mode}${agent.isMine ? '' : ' wk-agent-card-shared'} group/card relative flex h-[136px] min-h-[136px] cursor-pointer flex-col overflow-hidden box-border rounded-lg border border-[#e7e7e7] px-3.5 py-3 shadow-[0_1px_3px_rgba(0,0,0,0.04)] transition-all duration-[250ms] ease-[ease] hover:border-[#07c05f] ${modeTone}`}
      data-agent-id={agent.id}
      onClick={() => onOpen(agent)}
    >
      <div className={`pointer-events-none absolute top-3 right-11 z-0 flex items-start gap-1 transition-[color] duration-[250ms] ease-[ease] ${mode === 'agent' ? 'text-[rgba(124,77,255,0.35)]' : 'text-[rgba(7,192,95,0.35)]'}`} aria-hidden="true">
        <svg className="opacity-90" width="24" height="24" viewBox="0 0 20 20" fill="none"><path d="M10 3L10.8 6.2C10.9 6.7 11.3 7.1 11.8 7.2L15 8L11.8 8.8C11.3 8.9 10.9 9.3 10.8 9.8L10 13L9.2 9.8C9.1 9.3 8.7 8.9 8.2 8.8L5 8L8.2 7.2C8.7 7.1 9.1 6.7 9.2 6.2L10 3Z" stroke="currentColor" strokeWidth="0.8" strokeLinecap="round" strokeLinejoin="round" fill="currentColor" fillOpacity="0.15" /></svg>
        <svg className="mt-[10px] opacity-70" width="14" height="14" viewBox="0 0 20 20" fill="none"><path d="M10 3L10.8 6.2C10.9 6.7 11.3 7.1 11.8 7.2L15 8L11.8 8.8C11.3 8.9 10.9 9.3 10.8 9.8L10 13L9.2 9.8C9.1 9.3 8.7 8.9 8.2 8.8L5 8L8.2 7.2C8.7 7.1 9.1 6.7 9.2 6.2L10 3Z" stroke="currentColor" strokeWidth="0.8" strokeLinecap="round" strokeLinejoin="round" fill="currentColor" fillOpacity="0.15" /></svg>
      </div>
      <button
        type="button"
        className={`absolute top-0 right-0 z-[3] flex h-6 w-6 cursor-pointer items-center justify-center rounded-md border-none bg-transparent text-[rgba(0,0,0,0.6)] transition-[opacity,background,color] duration-150 ease-[ease] hover:bg-[#f3f3f3] hover:text-[#e37318] ${favorited ? 'opacity-100 text-[#e37318]' : 'opacity-0 group-hover/card:opacity-100'}`}
        aria-pressed={favorited ? 'true' : 'false'}
        aria-label={t('common.favorite')}
        onClick={(event) => { event.stopPropagation(); onToggleFavorite(agent.id); }}
      >
        <Icon name={favorited ? 'star-filled' : 'star'} size={14} />
      </button>
      <div className="relative z-[1] mb-1.5 flex items-center justify-between gap-1">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <AgentAvatar agent={agent} />
          <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-[15px] font-semibold leading-[22px] tracking-[0.01em] text-[rgba(0,0,0,0.9)]" title={agent.name}>{agent.name}</span>
        </div>
        {actions.length > 0 ? (
          <div className="wk-agent-card-more-wrap">
            <button
              type="button"
              className={`flex h-7 w-7 shrink-0 cursor-pointer items-center justify-center gap-0.5 rounded-lg border-none bg-transparent transition-all duration-200 ease-[ease] group-hover/card:opacity-60 hover:bg-[#f3f3f3] hover:opacity-100 ${menuOpen ? 'bg-[#f3f3f3] opacity-100' : 'opacity-0'}`}
              aria-label={t('agent.manageAgents')}
              aria-haspopup="menu"
              aria-expanded={menuOpen ? 'true' : 'false'}
              onClick={(event) => { event.stopPropagation(); onToggleMenu(menuOpen ? null : agent.id); }}
            >
              <span className="h-[3px] w-[3px] rounded-full bg-current" /><span className="h-[3px] w-[3px] rounded-full bg-current" /><span className="h-[3px] w-[3px] rounded-full bg-current" />
            </button>
            {menuOpen ? (
              <div className="absolute top-[30px] right-0 z-20 flex min-w-32 flex-col rounded-md bg-[var(--wk-bg,#fff)] p-1 shadow-[0_4px_16px_rgba(0,0,0,0.14)]" role="menu" onClick={(event) => event.stopPropagation()}>
                {actions.map((action) => (
                  <button
                    key={action}
                    type="button"
                    role="menuitem"
                    data-action={action}
                    className={`flex cursor-pointer items-center gap-2 rounded border-none px-2.5 py-[7px] text-left text-[13px] text-inherit hover:bg-[rgba(127,127,127,0.1)]${action === 'delete' ? ' text-[#d54941]' : ''}`}
                    onClick={() => onMenuAction(action, agent)}
                  >
                    <Icon name={ACTION_META[action]!.icon} size={14} />
                    <span>{t(action === 'toggle' && agent.disabledByMe ? 'agent.enable' : ACTION_META[action]!.labelKey)}</span>
                  </button>
                ))}
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
      <div className="relative z-[1] mb-1.5 flex min-h-0 flex-1 flex-col gap-1.5 overflow-hidden">
        <div className="line-clamp-2 overflow-hidden text-[12px] font-normal leading-[17px] text-[rgba(0,0,0,0.6)]">{agent.description || t('agent.noDescription')}</div>
      </div>
      <div className="relative z-[1] mt-auto flex items-center justify-between border-t-[0.5px] border-t-[#e7e7e7] pt-1.5">
        <div className="flex items-center gap-1">
          {agent.disabledByMe ? <span className="inline-flex items-center rounded bg-[rgba(127,127,127,0.12)] px-1.5 py-[2px] text-[11px]">{t('agent.disabled')}</span> : null}
          {expertBadgeId ? (
            // Octop M2 provenance: read-only label naming the expert template
            // the agent was instantiated from (config.expert_source).
            <span className="inline-flex items-center rounded bg-[rgba(124,77,255,0.08)] px-1.5 py-[2px] text-[11px] font-medium text-[#7c4dff]" data-agent-expert-badge>{t('experts.badge', { expertId: expertBadgeId })}</span>
          ) : null}
          {badges.map((key) => (
            <span key={key} className={`flex h-[22px] w-[22px] items-center justify-center rounded-[5px] transition-[background] duration-200 ease-[ease] ${FEATURE_BADGE_TONES[key] ?? ''}`} title={key === 'modeNormal' || key === 'modeAgent' || key === 'modePlain' ? t(modeTitleKey) : t(FEATURE_BADGE_TITLE_KEYS[key]!)}>
              <FeatureBadgeSvg badge={key} />
            </span>
          ))}
        </div>
        {!agent.isMine && !agent.sharedByMe ? (
          // Org source pill only on "all"-view shared cards; the Vue space-tab
          // card (AgentList.vue:546-635) ends with the feature badges alone.
          <span className="wk-agent-card-source inline-flex shrink-0 items-center gap-1 overflow-hidden text-ellipsis whitespace-nowrap rounded-[10px] bg-[rgba(127,127,127,0.1)] px-2 py-[2px] text-[11px] font-medium"><Icon name="usergroup" size={12} />{agent.orgName}</span>
        ) : badge ? (
          <span className="inline-flex shrink-0 items-center gap-[3px] rounded-[10px] bg-[rgba(127,127,127,0.1)] px-2 py-[2px] text-[11px] font-medium">
            {badge.kind === 'builtin' ? <Icon name="lock-on" size={12} /> : null}
            <span>{badge.kind === 'builtin' ? t('agent.builtin') : badge.name}</span>
          </span>
        ) : null}
      </div>
    </article>
  );
}

// --- detail drawer (AgentList.vue shared-detail-drawer + editable basics) --------

function scopeText(scope: ReturnType<typeof kbScope>, t: Translate, allKey: string, selectedKey: string, noneKey: string): string {
  if (scope.kind === 'all') return t(allKey);
  if (scope.kind === 'selected') return t(selectedKey, { count: scope.count });
  return t(noneKey);
}

export function AgentDetailDrawer({ kind, agent, t, onClose, onUseInChat }: {
  kind: 'shared';
  agent: AgentCardModel;
  t: Translate;
  onClose: () => void;
  onUseInChat: (agent: AgentCardModel) => void;
}) {
  const config = agent.config;
  return (
    <div className="fixed inset-0 z-[1000] flex justify-end bg-[rgba(0,0,0,0.4)]" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <aside className="flex h-full w-[360px] max-w-[90vw] flex-col bg-[var(--wk-bg,#fff)] shadow-[-4px_0_24px_rgba(0,0,0,0.12)]" role="dialog" aria-label={t('agent.detail.title')}>
        <div className="flex shrink-0 items-center justify-between border-b border-[rgba(127,127,127,0.2)] px-6 py-5">
          <h3 className="m-0 text-[18px] font-semibold">{t('agent.detail.title')}</h3>
          <button type="button" className="flex h-8 w-8 cursor-pointer items-center justify-center rounded-md border-none bg-[rgba(127,127,127,0.1)]" aria-label={t('common.cancel')} onClick={onClose}><Icon name="close" size={16} /></button>
        </div>
        <div className="flex flex-1 flex-col gap-5 overflow-y-auto p-6">
          <div className="flex flex-col gap-1.5"><span className="text-[12px] leading-[1.4] opacity-65">{t('agent.title')}</span><span className="break-words text-[14px] leading-[1.5]">{agent.name}</span></div>
          <div className="flex flex-col gap-1.5"><span className="text-[12px] leading-[1.4] opacity-65">{t('agent.noDescription')}</span><span className="break-words text-[14px] leading-[1.5]">{agent.description || t('agent.noDescription')}</span></div>
          <div className="flex flex-col gap-1.5"><span className="text-[12px] leading-[1.4] opacity-65">{t('agent.shareScope.title')}</span><span className="flex flex-col gap-2 break-words text-[14px] leading-[1.5]">
            <span>{t('agent.shareScope.knowledgeBase')}: {scopeText(kbScope(config), t, 'agent.shareScope.kbAll', 'agent.shareScope.kbSelected', 'agent.shareScope.kbNone')}</span>
            <span>{t('agent.shareScope.chatModel')}: {config?.model_id ? t('agent.shareScope.modelConfigured') : t('agent.shareScope.modelNotSet')}</span>
            <span>{t('agent.shareScope.webSearch')}: {config?.web_search_enabled ? t('agent.shareScope.enabled') : t('agent.shareScope.disabled')}</span>
            <span>{t('agent.shareScope.mcp')}: {scopeText(mcpScope(config), t, 'agent.shareScope.mcpAll', 'agent.shareScope.mcpSelected', 'agent.shareScope.mcpNone')}</span>
          </span></div>
        </div>
        <div className="flex shrink-0 justify-end gap-2.5 border-t border-[rgba(127,127,127,0.2)] bg-[var(--wk-bg,#fff)] px-6 py-4">
          <button type="button" className="inline-flex w-full cursor-pointer items-center justify-center rounded-md border-none bg-[#07c05f] px-[18px] py-2 text-[14px] text-white transition-[background] duration-200 ease-[ease] hover:bg-[#06b04d] disabled:cursor-default disabled:opacity-60" onClick={() => onUseInChat(agent)}>{t('agent.detail.useInChat')}</button>
        </div>
      </aside>
    </div>
  );
}

// --- delete confirm dialog (AgentList.vue del-agent-dialog) -----------------------

export function AgentDeleteDialog({ agent, t, busy, onConfirm, onCancel }: {
  agent: AgentCardModel | null; t: Translate; busy: boolean; onConfirm: () => void; onCancel: () => void;
}) {
  if (!agent) return null;
  return (
    <div className="fixed inset-0 z-[1100] flex items-start justify-center bg-[rgba(0,0,0,0.4)] pt-[40vh]" onClick={(event) => { if (event.target === event.currentTarget) onCancel(); }}>
      <div className="w-[400px] max-w-[90vw] rounded-md bg-[var(--wk-bg,#fff)] p-4 shadow-[0_8px_32px_rgba(0,0,0,0.18)]" role="alertdialog" aria-label={t('agent.delete.confirmTitle')}>
        <div className="mb-2 flex items-center">
          <span className="mr-2 inline-flex text-[#d54941]" aria-hidden="true"><Icon name="delete" size={18} /></span>
          <span className="text-[16px] font-semibold leading-6">{t('agent.delete.confirmTitle')}</span>
        </div>
        <p className="m-0 mb-[21px] ml-[29px] text-[14px] leading-[22px] opacity-70">{t('agent.delete.confirmMessage', { name: agent.name })}</p>
        <div className="flex justify-end gap-10">
          <button type="button" className="cursor-pointer border-none bg-transparent p-0 text-[14px] text-inherit" onClick={onCancel}>{t('common.cancel')}</button>
          <button type="button" className="cursor-pointer border-none bg-transparent p-0 text-[14px] text-[#d54941]" disabled={busy} onClick={onConfirm}>{busy ? t('common.loading') : t('agent.delete.confirmButton')}</button>
        </div>
      </div>
    </div>
  );
}

// --- page view ------------------------------------------------------------------------

export interface AgentsPageViewProps {
  t: Translate;
  editorT: Translate;
  client: WeKnoraClient;
  viewer: AgentViewer;
  loading: boolean;
  space: string;
  /** Space tab fetch in flight (Vue AgentList.vue spaceAgentsLoading). */
  spaceLoading?: boolean;
  rail: AgentRailItem[];
  sections: AgentSectionView[];
  flatCards: AgentCardModel[];
  isSectioned: boolean;
  favorites: ReadonlySet<string>;
  openMenuId: string | null;
  error: string | null;
  notice: string | null;
  drawer: { kind: 'shared'; agent: AgentCardModel } | null;
  editor: { mode: 'create' | 'edit'; agent: AgentCardModel | null; initialSection?: string; initialHighlight?: string; readOnly?: boolean } | null;
  deleteTarget: AgentCardModel | null;
  deleting: boolean;
  collapsedSections: ReadonlySet<string>;
  canCreate: boolean;
  onSpaceChange: (key: string) => void;
  onOpenCard: (agent: AgentCardModel) => void;
  onToggleFavorite: (id: string) => void;
  onToggleMenu: (id: string | null) => void;
  onMenuAction: (action: AgentCardAction, agent: AgentCardModel) => void;
  onCreate: () => void;
  onCloseDrawer: () => void;
  onUseInChat: (agent: AgentCardModel) => void;
  onCloseEditor: () => void;
  onEditorSaved: (agent: Record<string, unknown>, mode: 'create' | 'edit') => void;
  onDeleteConfirm: () => void;
  onDeleteCancel: () => void;
  onToggleSection: (key: string) => void;
}

function EmptyState({ t, space, canCreate, onCreate }: { t: Translate; space: string; canCreate: boolean; onCreate: () => void }) {
  if (space === 'favorites') {
    return (
      <div className="col-span-full flex flex-1 flex-col items-center justify-center px-5 py-[60px]">
        <span className="mb-5 opacity-40"><Icon name="star" size={48} /></span>
        <span className="mb-2 text-[16px] font-semibold leading-[26px] opacity-60">{t('agent.empty.favoritesTitle')}</span>
        <span className="text-[14px] leading-[22px] opacity-45">{t('agent.empty.favoritesDescription')}</span>
      </div>
    );
  }
  if (space === 'recents') {
    return (
      <div className="col-span-full flex flex-1 flex-col items-center justify-center px-5 py-[60px]">
        <span className="mb-5 opacity-40"><Icon name="history" size={48} /></span>
        <span className="mb-2 text-[16px] font-semibold leading-[26px] opacity-60">{t('agent.empty.recentsTitle')}</span>
        <span className="text-[14px] leading-[22px] opacity-45">{t('agent.empty.recentsDescription')}</span>
      </div>
    );
  }
  if (space === 'all' || space === 'mine') {
    return (
      <div className="col-span-full flex flex-1 flex-col items-center justify-center px-5 py-[60px]">
        <span className="mb-2 text-[16px] font-semibold leading-[26px] opacity-60">{t('agent.empty.title')}</span>
        <span className="text-[14px] leading-[22px] opacity-45">{t('agent.empty.description')}</span>
        {canCreate ? (
          <button type="button" className="wk-agent-empty-btn relative mt-5 inline-flex cursor-pointer items-center gap-1.5 overflow-hidden rounded-md border-none bg-[#07c05f] px-4 py-[7px] text-[14px] text-white transition-[background] duration-200 ease-[ease] hover:bg-[#06b04d]" data-guide="agent-list-create" onClick={onCreate}><SparklesIcon size={18} /><span>{t('agent.createAgent')}</span></button>
        ) : null}
      </div>
    );
  }
  return (
    <div className="col-span-full flex flex-1 flex-col items-center justify-center px-5 py-[60px]">
      <span className="mb-2 text-[16px] font-semibold leading-[26px] opacity-60">{t('agent.empty.sharedTitle')}</span>
      <span className="text-[14px] leading-[22px] opacity-45">{t('agent.empty.sharedDescription')}</span>
    </div>
  );
}

function AgentSection({ section, t, viewer, collapsed, onToggle, cardProps }: {
  section: AgentSectionView; t: Translate; viewer: AgentViewer; collapsed: boolean;
  onToggle: (key: string) => void;
  cardProps: (agent: AgentCardModel) => Parameters<typeof AgentCard>[0];
}) {
  return (
    <>
      <AgentSectionHeader section={section} t={t} viewer={viewer} collapsed={collapsed} onToggle={onToggle} />
      {collapsed ? null : section.cards.map((agent) => <AgentCard key={agent.id} {...cardProps(agent)} />)}
    </>
  );
}

export function AgentsPageView(props: AgentsPageViewProps) {
  const { t, editorT, viewer, loading, space, spaceLoading, rail, sections, flatCards, isSectioned, favorites, openMenuId, error, notice, drawer, editor, deleteTarget, deleting, collapsedSections, canCreate } = props;
  const cardProps = (agent: AgentCardModel) => ({
    agent, t, viewer,
    favorited: favorites.has(agent.id),
    menuOpen: openMenuId === agent.id,
    onOpen: props.onOpenCard,
    onToggleFavorite: props.onToggleFavorite,
    onToggleMenu: props.onToggleMenu,
    onMenuAction: props.onMenuAction,
  });
  const hasCards = isSectioned ? sections.length > 0 : flatCards.length > 0;
  return (
    <main className="relative m-0 flex h-full min-h-0 flex-1 box-border [-webkit-font-smoothing:antialiased] [-moz-osx-font-smoothing:grayscale]">
      <AgentRail t={t} items={rail} onSelect={props.onSpaceChange} />
      <div className="flex min-w-0 flex-1 flex-col pt-5 pl-7">
        <header className="mb-4 flex items-center justify-between pr-7 [&_h2]:m-0 [&_h2]:text-[24px] [&_h2]:font-semibold [&_h2]:leading-8">
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-2">
              <h2>{t('agent.title')}</h2>
              {canCreate ? (
                <button type="button" className="inline-flex h-7 w-7 min-w-7 cursor-pointer items-center justify-center rounded-md border border-[#e7e7e7] bg-[#f3f3f3] p-0 text-[#07c05f] transition-[background,border-color,color] duration-200 ease-[ease] hover:text-[#06b04d]" data-guide="agent-list-create" title={t('agent.createAgent')} aria-label={t('agent.createAgent')} onClick={props.onCreate}><SparklesIcon /></button>
              ) : null}
            </div>
            <p className="m-0 text-[14px] font-normal leading-5 opacity-65">{t('agent.subtitle')}</p>
          </div>
        </header>
        {notice ? <Status tone="success">{notice}</Status> : null}
        {loading ? <div className="grid min-w-0 flex-1 content-start gap-3 overflow-y-auto pr-7 pb-2 grid-cols-1 min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4 min-[1900px]:grid-cols-5 min-[2200px]:grid-cols-6" aria-busy="true">{Array.from({ length: 6 }, (_, index) => <div className="h-[136px] animate-[wk-agent-shimmer_1.2s_ease_infinite] rounded-lg border border-[rgba(127,127,127,0.18)] bg-[linear-gradient(90deg,rgba(127,127,127,0.06)_25%,rgba(127,127,127,0.12)_37%,rgba(127,127,127,0.06)_63%)] bg-[length:400%_100%]" key={index} />)}</div> : null}
        {spaceLoading ? (
          <div className="flex min-w-0 flex-1 items-center justify-center" role="status" aria-label={t('common.loading')} data-agent-space-loading="true">
            <span className="h-6 w-6 animate-spin rounded-full border-2 border-[rgba(127,127,127,0.25)] border-t-[#07c05f]" aria-hidden="true" />
          </div>
        ) : null}
        {!loading && !spaceLoading && hasCards ? (
          <div className="grid min-w-0 flex-1 content-start gap-3 overflow-y-auto pr-7 pb-2 grid-cols-1 min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4 min-[1900px]:grid-cols-5 min-[2200px]:grid-cols-6">
            {isSectioned
              ? sections.map((section) => (
                  <AgentSection key={section.key} section={section} t={t} viewer={viewer} collapsed={collapsedSections.has(section.key)} onToggle={props.onToggleSection} cardProps={cardProps} />
              ))
              : flatCards.map((agent) => <AgentCard key={agent.id} {...cardProps(agent)} />)}
          </div>
        ) : null}
        {!loading && !spaceLoading && !hasCards ? <EmptyState t={t} space={space} canCreate={canCreate} onCreate={props.onCreate} /> : null}
      </div>
      {drawer ? <AgentDetailDrawer kind={drawer.kind} agent={drawer.agent} t={t} onClose={props.onCloseDrawer} onUseInChat={props.onUseInChat} /> : null}
      {editor ? <AgentEditorModal open mode={editor.mode} agent={editor.agent} initialSection={editor.initialSection} initialHighlightField={editor.initialHighlight} readOnly={editor.readOnly} client={props.client} t={editorT} onClose={props.onCloseEditor} onSaved={props.onEditorSaved} /> : null}
      <AgentDeleteDialog agent={deleteTarget} t={t} busy={deleting} onConfirm={props.onDeleteConfirm} onCancel={props.onDeleteCancel} />
    </main>
  );
}

// --- space URL state (Vue useListUrlState ?scope=) ------------------------------------

const RESERVED_SCOPES = new Set(['all', 'mine', 'favorites', 'recents']);

function readSpaceFromUrl(): string | null {
  const value = new URLSearchParams(window.location.search).get('scope');
  return value && value.trim() !== '' ? value : null;
}

function writeSpaceToUrl(space: string): void {
  const url = new URL(window.location.href);
  if (space === 'all' || !space) url.searchParams.delete('scope');
  else url.searchParams.set('scope', space);
  window.history.replaceState({}, '', url);
}

function isSectionedView(space: string): boolean {
  return space === 'all' || space === 'mine' || (space !== '' && space !== 'favorites' && space !== 'recents');
}

// The Vue rail labels live under listSpaceSidebar.*, which packages/i18n does
// not carry yet (reported gap). formatMessage echoes unknown keys, so try the
// real key first and fall back to the Vue literals until the keys land.
const RAIL_LABEL_FALLBACK: Record<'recents' | 'workspace', Partial<Record<Locale, string>>> = {
  recents: { 'zh-CN': '最近', 'en-US': 'Recent', 'ja-JP': '最近', 'ko-KR': '최근', 'ru-RU': 'Недавние' },
  workspace: { 'zh-CN': '本空间', 'en-US': 'Workspace', 'ja-JP': 'ワークスペース', 'ko-KR': '워크스페이스', 'ru-RU': 'Пространство' },
};

function railLabel(t: Translate, key: string, kind: 'recents' | 'workspace', locale: Locale): string {
  const resolved = t(key);
  return resolved === key ? RAIL_LABEL_FALLBACK[kind][locale] ?? resolved : resolved;
}

function membershipRoleOf(memberships: unknown, tenantId: string | null): string {
  if (!Array.isArray(memberships)) return 'viewer';
  for (const item of memberships) {
    if (!item || typeof item !== 'object') continue;
    const row = item as Record<string, unknown>;
    const id = row.tenant_id ?? row.tenantId;
    const role = row.role;
    if (tenantId !== null && String(id) === tenantId && typeof role === 'string') return role;
  }
  return 'viewer';
}

interface AgentsPageProps { client: WeKnoraClient; tenantId?: string | number | null }

// Vue's useTenantModelReadiness counts KnowledgeQA models as chat models.
export function hasAgentChatModel(models: ReadonlyArray<{ type?: unknown }>): boolean {
  return models.some((model) => model.type === 'KnowledgeQA');
}

export function AgentsPage({ client, tenantId }: AgentsPageProps) {
  const locale = usePreferredLocale();
  const t = useCallback<Translate>((key, values) => formatMessage(locale, key, values), [locale]);
  // agentEditor.* copy is not in packages/i18n yet; the editor falls back to
  // ported literals for those keys while agent.* resolves normally.
  const editorT = useMemo(() => makeEditorT(locale), [locale]);
  const [viewer, setViewer] = useState<AgentViewer>({ userId: '', isAdmin: false, isContributor: false });
  const [viewerReady, setViewerReady] = useState(false);
  // Vue useTenantModelReadiness (frontend/src/composables/useTenantModelReadiness.ts):
  // the agent list tours and the create gate read tenant model readiness —
  // a configured chat (KnowledgeQA / type llm) model.
  const [modelsReady, setModelsReady] = useState<boolean | null>(null);
  const [data, setData] = useState<AgentsPageData | null>(null);
  const [spaceItems, setSpaceItems] = useState<Array<Record<string, unknown>>>([]);
  const [spaceLoading, setSpaceLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [reloadToken, setReloadToken] = useState(0);
  const [space, setSpaceState] = useState<string>(() => (typeof window === 'undefined' ? 'all' : readSpaceFromUrl() ?? ''));
  const [favorites, setFavorites] = useState<Set<string>>(() => new Set());
  const [recents, setRecents] = useState<PinEntry[]>([]);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const [collapsedSections, setCollapsedSections] = useState<ReadonlySet<string>>(new Set());
  const [drawer, setDrawer] = useState<AgentsPageViewProps['drawer']>(null);
  const [editor, setEditor] = useState<AgentsPageViewProps['editor']>(null);
  const [deleteTarget, setDeleteTarget] = useState<AgentCardModel | null>(null);
  const [deleting, setDeleting] = useState(false);

  const tenantKey = tenantId === undefined || tenantId === null ? null : String(tenantId);

  // Viewer + per-(user, tenant) pins hydrate (App.tsx membershipRole pattern).
  useEffect(() => {
    let active = true;
    setViewerReady(false);
    void client.auth.me().then((me) => {
      if (!active) return;
      const role = membershipRoleOf(me.memberships, tenantKey);
      const userId = typeof me.user?.id === 'string' ? me.user.id : '';
      setViewer({
        userId,
        isAdmin: role === 'owner' || role === 'admin' || me.user?.is_system_admin === true,
        isContributor: role === 'owner' || role === 'admin' || role === 'contributor' || me.user?.is_system_admin === true,
      });
      setViewerReady(true);
      setFavorites(new Set(readFavoriteIds(window.localStorage, userId, tenantKey)));
      setRecents(readAgentRecents(window.localStorage, userId, tenantKey));
    }).catch(() => {
      // Deep-link consumption waits for an authoritative permission result;
      // a failed identity lookup must not accidentally grant an edit surface.
      setViewerReady(true);
    });
    return () => { active = false; };
  }, [client, tenantKey]);

  useEffect(() => {
    let active = true;
    setLoadError(null);
    void loadAgentsPageData(client).then((next) => {
      if (active) setData(next);
    }).catch((loadFailure) => {
      if (active) setLoadError(loadFailure instanceof Error ? loadFailure.message : t('common.error'));
    });
    return () => { active = false; };
  }, [client, reloadToken, t]);

  useEffect(() => {
    let active = true;
    void client.configuration.models.list().then((models) => {
      if (active) setModelsReady(hasAgentChatModel(models));
    }).catch(() => { if (active) setModelsReady(false); });
    return () => { active = false; };
  }, [client]);

  // Default scope follows the Vue role rule: contributor → mine, else all
  // (AgentList.vue defaultScope). Resolved once here so rows, sections, and
  // the rail all agree even before the user picks a scope.
  const effectiveSpace = space || (viewer.isContributor ? 'mine' : 'all');

  // Space view fetch (GET /organizations/:id/shared-agents — routes_agent.go:171).
  // While in flight the space tab shows the Vue spaceAgentsLoading spinner
  // instead of the "no shared agents" empty state (AgentList.vue:498-500,710).
  useEffect(() => {
    if (!effectiveSpace || RESERVED_SCOPES.has(effectiveSpace)) { setSpaceItems([]); setSpaceLoading(false); return; }
    let active = true;
    setSpaceLoading(true);
    void client.identity.organizations.agentShares.listInOrganization(effectiveSpace).then((rows) => {
      if (!active) return;
      setSpaceItems(rows);
      setSpaceLoading(false);
    }).catch(() => { if (active) { setSpaceItems([]); setSpaceLoading(false); } });
    return () => { active = false; };
  }, [client, effectiveSpace]);

  const setSpace = useCallback((next: string) => {
    setSpaceState(next);
    setOpenMenuId(null);
    writeSpaceToUrl(next);
  }, []);

  const reload = useCallback(() => setReloadToken((value) => value + 1), []);

  const toggleFavorite = useCallback((id: string) => {
    setFavorites((current) => {
      const next = new Set(toggleFavoriteId([...current], id));
      writeFavoriteIds(window.localStorage, viewer.userId, tenantKey, [...next]);
      return next;
    });
  }, [tenantKey, viewer.userId]);

  const openCard = useCallback((agent: AgentCardModel) => {
    setOpenMenuId(null);
    setRecents((current) => {
      const next = touchAgentRecent(current, agent.id, Date.now());
      writeAgentRecents(window.localStorage, viewer.userId, tenantKey, next);
      return next;
    });
    // AgentList.vue handleCardClick / handleSpaceAgentCardClick: shared →
    // detail drawer, own (and space-view "shared by me") → editor.
    if (opensEditorOnCardClick(agent)) { setEditor({ mode: 'edit', agent }); return; }
    setDrawer({ kind: 'shared', agent });
  }, [tenantKey, viewer.userId]);

  const onMenuAction = useCallback((action: AgentCardAction, agent: AgentCardModel) => {
    setOpenMenuId(null);
    if (action === 'edit') { setEditor({ mode: 'edit', agent }); return; }
    if (action === 'delete') { setDeleteTarget(agent); return; }
    if (action === 'copy') {
      void client.configuration.agents.copy(agent.id).then(() => { setNotice(t('agent.messages.copied')); reload(); }).catch(() => setNotice(t('agent.messages.copyFailed')));
      return;
    }
    if (action === 'toggle') {
      void client.identity.organizations.agentShares.setDisabledByMe(agent.id, !agent.disabledByMe).then(() => { setNotice(t(agent.disabledByMe ? 'agent.messages.enabled' : 'agent.messages.disabled')); reload(); }).catch(() => setNotice(t('agent.messages.saveFailed')));
    }
  }, [client, reload, t]);

  const onCreate = useCallback(() => {
    // Vue AgentList.vue:1597-1602 — without a chat model the create click
    // warns and opens the models settings section instead; the tenantModels
    // (agent) tour re-arms there via the queued intent.
    if (modelsReady === false) {
      setNotice(contextualGuideMessage(locale, 'contextualGuide.tenantModels.needChatModelFirst'));
      openContextualGuide('tenantModels', { variant: 'agent' });
    navigate('/platform/settings?section=models');
      return;
    }
    // Vue AgentList.vue:1603 + AgentCreateContextualGuide :when="create" —
    // opening the editor retires the agentList tour and arms agentCreate.
    markContextualGuideDone(window.localStorage, 'agentList');
    openContextualGuide('agentCreate', { isAgentMode: false });
    setEditor({ mode: 'create', agent: null });
  }, [locale, modelsReady]);

  const onEditorSaved = useCallback((_agent: Record<string, unknown>, mode: 'create' | 'edit') => {
    // Vue AgentEditorModal.vue:4825 — a successful create retires agentCreate.
    if (mode === 'create') markContextualGuideDone(window.localStorage, 'agentCreate');
    // create stays open inside the modal (post-create session); refresh the list either way
    setNotice(t(mode === 'create' ? 'agent.messages.created' : 'agent.messages.updated'));
    reload();
  }, [reload, t]);

  const onDeleteConfirm = useCallback(() => {
    if (!deleteTarget || deleting) return;
    setDeleting(true);
    void client.configuration.agents.remove(deleteTarget.id).then(() => { setDeleting(false); setDeleteTarget(null); setNotice(t('agent.messages.deleted')); reload(); })
      .catch(() => { setDeleting(false); setNotice(t('agent.messages.deleteFailed')); });
  }, [client, deleteTarget, deleting, reload, t]);

  const onUseInChat = useCallback((agent: AgentCardModel) => {
    setDrawer(null);
    navigate(chatNavigationPath(agent));
  }, []);

  const rows = useMemo<AgentCardModel[]>(() => {
    if (!data) return [];
    const options = { userId: viewer.userId, disabledOwnIds: data.disabledOwnIds };
    if (effectiveSpace === 'favorites' || effectiveSpace === 'recents') {
      const index = buildPinIndex(data.ownAgents, data.sharedAgents, options);
      const ids = effectiveSpace === 'favorites' ? [...favorites] : recents.map((entry) => entry.id);
      return hydratePinnedCards(index, ids);
    }
    if (effectiveSpace === 'mine') return buildMineViewRows(data.ownAgents, options);
    if (!RESERVED_SCOPES.has(effectiveSpace)) return buildSpaceViewRows(spaceItems);
    return buildAllViewRows(data.ownAgents, data.sharedAgents, options);
  }, [data, effectiveSpace, favorites, recents, spaceItems, viewer.userId]);

  useEffect(() => {
    if (!data || !viewerReady) return;
    const deepLink = parseAgentEditDeepLink(window.location.search);
    if (!deepLink) return;
    const allRows = buildAllViewRows(data.ownAgents, data.sharedAgents, { userId: viewer.userId, disabledOwnIds: data.disabledOwnIds });
    const target = resolveAgentEditTarget(
      allRows.filter((agent) => agent.isMine),
      allRows.filter((agent) => !agent.isMine),
      deepLink.editId,
      deepLink.sourceTenantId,
    );
    if (!target) return;
    setDrawer(null);
    setEditor({
      mode: 'edit',
      agent: target,
      initialSection: deepLink.section,
      initialHighlight: deepLink.highlight,
      readOnly: !canManageAgent(target, viewer),
    });
    const url = new URL(window.location.href);
    url.searchParams.delete('edit');
    url.searchParams.delete('section');
    url.searchParams.delete('highlight');
    url.searchParams.delete('sourceTenantId');
    window.history.replaceState({}, '', url);
  }, [data, viewer, viewerReady]);

  const sectioned = isSectionedView(effectiveSpace);
  const sections = useMemo<AgentSectionView[]>(() => (sectioned ? sectionize(rows, viewer.userId) : []), [rows, sectioned, viewer.userId]);
  const flatCards = useMemo(() => (sectioned ? [] : rows), [rows, sectioned]);

  const rail = useMemo<AgentRailItem[]>(() => {
    const ownCount = data?.ownAgents.length ?? 0;
    const sharedCount = data?.sharedAgents.length ?? 0;
    const counts = data ? sharedCountByOrg(data.sharedAgents) : {};
    return [
      { key: 'all', label: t('common.all'), icon: 'layers' as const, count: ownCount + sharedCount, active: effectiveSpace === 'all' },
      { key: 'favorites', label: t('common.favorite'), icon: 'star' as const, count: favorites.size, active: effectiveSpace === 'favorites' },
      { key: 'recents', label: railLabel(t, 'listSpaceSidebar.recents', 'recents', locale), icon: 'history' as const, count: recents.length, active: effectiveSpace === 'recents' },
      { key: 'mine', label: railLabel(t, 'listSpaceSidebar.workspace', 'workspace', locale), icon: 'workspace' as const, count: ownCount, active: effectiveSpace === 'mine' },
      ...(data?.organizations ?? []).filter((org) => (counts[org.id] ?? 0) > 0).map((org) => ({
        key: org.id, label: org.name, icon: 'space' as const, avatarName: org.name, count: counts[org.id], active: effectiveSpace === org.id,
      })),
    ];
  }, [data, effectiveSpace, favorites.size, locale, recents.length, t]);

  // Vue AgentList.vue:1088-1102 — the empty contributor list (all/mine scope,
  // editor closed) selects between the tenantModels tour (agent variant, when
  // no chat model is configured) and the agentList tour (when it is). The
  // shell-level guide host applies the dismissal + welcome-tour gates.
  const agentListGuideBase = data !== null
    && viewer.isContributor
    && editor === null
    && (effectiveSpace === 'all' || effectiveSpace === 'mine')
    && rows.length === 0;
  const tenantModelsGuideWhen = agentListGuideBase && modelsReady === false;
  const agentListGuideWhen = agentListGuideBase && modelsReady === true;
  useEffect(() => {
    if (!tenantModelsGuideWhen) return;
    openContextualGuide('tenantModels', { variant: 'agent' });
  }, [tenantModelsGuideWhen]);
  useEffect(() => {
    if (!agentListGuideWhen) return;
    openContextualGuide('agentList');
  }, [agentListGuideWhen]);

  return (
    <AgentsPageView
      t={t}
      editorT={editorT}
      client={client}
      viewer={viewer}
      loading={!data && !loadError}
      space={effectiveSpace}
      spaceLoading={spaceLoading}
      rail={rail}
      sections={sections}
      flatCards={flatCards}
      isSectioned={isSectionedView(effectiveSpace)}
      favorites={favorites}
      openMenuId={openMenuId}
      error={loadError}
      notice={notice}
      drawer={drawer}
      editor={editor}
      deleteTarget={deleteTarget}
      deleting={deleting}
      collapsedSections={collapsedSections}
      canCreate={viewer.isContributor}
      onSpaceChange={setSpace}
      onOpenCard={openCard}
      onToggleFavorite={toggleFavorite}
      onToggleMenu={setOpenMenuId}
      onMenuAction={onMenuAction}
      onCreate={onCreate}
      onCloseDrawer={() => setDrawer(null)}
      onUseInChat={onUseInChat}
      onCloseEditor={() => setEditor(null)}
      onEditorSaved={onEditorSaved}
      onDeleteConfirm={onDeleteConfirm}
      onDeleteCancel={() => setDeleteTarget(null)}
      onToggleSection={(key) => setCollapsedSections((current) => {
        const next = new Set(current);
        if (next.has(key)) next.delete(key); else next.add(key);
        return next;
      })}
    />
  );
}
