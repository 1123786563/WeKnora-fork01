import { useCallback, useEffect, useMemo, useState } from 'react';
import type { AgentConfiguration, WeKnoraClient } from '@weknora/api-client';
import { Status } from '@weknora/ui';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';
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
  hydratePinnedCards,
  kbScope,
  mcpScope,
  FEATURE_BADGE_TITLE_KEYS,
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

function Icon({ name, size = 16 }: { name: string; size?: number }) {
  const paths: Record<string, React.ReactNode> = {
    layers: <><path d="M12 3 3 8l9 5 9-5-9-5Z" /><path d="m3 13 9 5 9-5" /></>,
    star: <path d="M12 3.5 14.7 9l6 .7-4.4 4.1 1.1 5.9L12 16.8 6.6 19.7l1.1-5.9L3.3 9.7l6-.7L12 3.5Z" />,
    history: <><path d="M3.5 12a8.5 8.5 0 1 0 2.5-6" /><path d="M3 4v4h4" /><path d="M12 8v4.5l3 1.8" /></>,
    workspace: <><rect x="4" y="4" width="7" height="7" rx="1" /><rect x="13" y="4" width="7" height="7" rx="1" /><rect x="4" y="13" width="7" height="7" rx="1" /><rect x="13" y="13" width="7" height="7" rx="1" /></>,
    app: <><rect x="4" y="4" width="16" height="16" rx="3" /><path d="M4 10h16M10 10v10" /></>,
    user: <><circle cx="12" cy="8" r="3.5" /><path d="M5 20a7 7 0 0 1 14 0" /></>,
    usergroup: <><circle cx="9" cy="9" r="3" /><path d="M3.5 19a5.5 5.5 0 0 1 11 0" /><path d="M16 6.5a3 3 0 0 1 0 5.5" /><path d="M17.5 14.5a5.5 5.5 0 0 1 3 4.5" /></>,
    'usergroup-add': <><circle cx="9" cy="9" r="3" /><path d="M3.5 19a5.5 5.5 0 0 1 11 0" /><path d="M18 6v6M15 9h6" /></>,
    share: <><circle cx="6" cy="12" r="2.5" /><circle cx="17" cy="6" r="2.5" /><circle cx="17" cy="18" r="2.5" /><path d="m8.3 10.8 6.4-3.6M8.3 13.2l6.4 3.6" /></>,
    'control-platform': <><circle cx="12" cy="12" r="3" /><path d="M12 4v2.5M12 17.5V20M4 12h2.5M17.5 12H20M6.3 6.3l1.8 1.8M15.9 15.9l1.8 1.8M17.7 6.3l-1.8 1.8M8.1 15.9l-1.8 1.8" /></>,
    chat: <path d="M4 6a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H9l-4.5 3.5V6Z" />,
    folder: <path d="M3.5 6.5A1.5 1.5 0 0 1 5 5h4l2 2.5h8A1.5 1.5 0 0 1 20.5 9v8A1.5 1.5 0 0 1 19 18.5H5A1.5 1.5 0 0 1 3.5 17v-10.5Z" />,
    extension: <path d="M10 4h4v2.5a1.5 1.5 0 0 0 3 0V4h3v4h-2.5a1.5 1.5 0 0 0 0 3H20v4h-3v-2.5a1.5 1.5 0 0 0-3 0V15h-4v-2.5" />,
    'chat-bubble': <><path d="M4 5h16v11H10l-4 3v-3H4V5Z" /><path d="M8 9.5h8M8 12.5h5" /></>,
    edit: <><path d="M4 20h4L20 8l-4-4L4 16v4Z" /><path d="m13.5 6.5 4 4" /></>,
    'file-copy': <><rect x="8" y="8" width="12" height="12" rx="2" /><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2" /></>,
    poweroff: <><path d="M12 3v8" /><path d="M6.3 6.5a8 8 0 1 0 11.4 0" /></>,
    delete: <><path d="M4 7h16" /><path d="M9 7V5a1.5 1.5 0 0 1 1.5-1.5h3A1.5 1.5 0 0 1 15 5v2" /><path d="M6.5 7 7.5 20h9L17.5 7" /></>,
    close: <path d="M6 6l12 12M18 6 6 18" />,
    'lock-on': <><rect x="5" y="10" width="14" height="10" rx="2" /><path d="M8 10V7a4 4 0 0 1 8 0v3" /></>,
    browse: <><path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z" /><circle cx="12" cy="12" r="3" /></>,
    'edit-1': <path d="M14.5 5.5 18.5 9.5 9 19H5v-4l9.5-9.5Z" />,
  };
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6"
      strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="wk-agent-icon">{paths[name] ?? null}</svg>
  );
}

/** Three-star sparkles decoration, ported from the Vue header create button. */
function SparklesIcon({ size = 19 }: { size?: number }) {
  return (
    <svg className="wk-agent-sparkles" width={size} height={size} viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
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
    return (
      <span className={`wk-agent-builtin-avatar ${smart ? 'agent' : 'normal'}`}>
        <Icon name={smart ? 'control-platform' : 'chat'} size={18} />
      </span>
    );
  }
  if (agent.avatar) return <span className="wk-agent-builtin-avatar agent-emoji">{agent.avatar}</span>;
  const gradient = avatarGradient(agent.name || '');
  return (
    <span
      className="wk-agent-avatar"
      style={{ background: `linear-gradient(135deg, ${gradient.from} 0%, ${gradient.to} 100%)` }}
    >{avatarLetter(agent.name || '')}</span>
  );
}

const FEATURE_BADGE_ICONS: Record<string, string> = {
  modeNormal: 'chat',
  modeAgent: 'control-platform',
  knowledge: 'folder',
  mcp: 'extension',
  multiTurn: 'chat-bubble',
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
  return <Icon name={FEATURE_BADGE_ICONS[badge] ?? 'chat'} size={16} />;
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
  return (
    <nav className="wk-agent-rail" aria-label={t('agent.title')}>
      {items.map((item) => (
        <button
          key={item.key}
          type="button"
          data-space-key={item.key}
          className={`wk-agent-rail-item${item.active ? ' wk-agent-rail-item-active' : ''}`}
          aria-current={item.active ? 'true' : undefined}
          title={`${item.label}${item.count === undefined ? '' : ` (${item.count})`}`}
          onClick={() => onSelect(item.key)}
        >
          <span className="wk-agent-rail-icon">
            {item.icon === 'space'
              ? <span className="wk-agent-rail-space-avatar">{avatarLetter(item.avatarName ?? item.label)}</span>
              : <Icon name={item.icon === 'workspace' ? 'workspace' : item.icon} size={16} />}
          </span>
          <span className="wk-agent-rail-label">{item.label}</span>
          {item.count !== undefined ? <span className="wk-agent-rail-count">{item.count}</span> : null}
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
      {subIcon ? <span className="wk-agent-section-subicon"><Icon name={subIcon} size={12} /></span> : null}
      <span>{t(agentSectionLabelKey(section.key, viewer))}</span>
      <span className="wk-agent-section-count">{section.count}</span>
      <span className="wk-agent-section-toggle" aria-hidden="true">{collapsed ? '›' : '⌄'}</span>
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
  const actions = cardActions(agent, viewer);
  const badge = cornerBadge(agent, viewer.userId);
  const modeTitleKey = agent.config?.agent_mode === 'smart-reasoning' ? 'agent.mode.agent' : 'agent.mode.normal';
  return (
    <article
      className={`wk-agent-card agent-mode-${mode}${agent.isMine ? '' : ' wk-agent-card-shared'}`}
      data-agent-id={agent.id}
      onClick={() => onOpen(agent)}
    >
      <div className="wk-agent-card-decoration" aria-hidden="true">
        <svg className="star-icon" width="24" height="24" viewBox="0 0 20 20" fill="none"><path d="M10 3L10.8 6.2C10.9 6.7 11.3 7.1 11.8 7.2L15 8L11.8 8.8C11.3 8.9 10.9 9.3 10.8 9.8L10 13L9.2 9.8C9.1 9.3 8.7 8.9 8.2 8.8L5 8L8.2 7.2C8.7 7.1 9.1 6.7 9.2 6.2L10 3Z" stroke="currentColor" strokeWidth="0.8" strokeLinecap="round" strokeLinejoin="round" fill="currentColor" fillOpacity="0.15" /></svg>
        <svg className="star-icon small" width="14" height="14" viewBox="0 0 20 20" fill="none"><path d="M10 3L10.8 6.2C10.9 6.7 11.3 7.1 11.8 7.2L15 8L11.8 8.8C11.3 8.9 10.9 9.3 10.8 9.8L10 13L9.2 9.8C9.1 9.3 8.7 8.9 8.2 8.8L5 8L8.2 7.2C8.7 7.1 9.1 6.7 9.2 6.2L10 3Z" stroke="currentColor" strokeWidth="0.8" strokeLinecap="round" strokeLinejoin="round" fill="currentColor" fillOpacity="0.15" /></svg>
      </div>
      <button
        type="button"
        className={`wk-agent-favorite-star${favorited ? ' is-favorited' : ''}`}
        aria-pressed={favorited ? 'true' : 'false'}
        aria-label={t('common.favorite')}
        onClick={(event) => { event.stopPropagation(); onToggleFavorite(agent.id); }}
      >
        <Icon name="star" size={14} />
      </button>
      <div className="wk-agent-card-header">
        <div className="wk-agent-card-header-left">
          <AgentAvatar agent={agent} />
          <span className="wk-agent-card-title" title={agent.name}>{agent.name}</span>
        </div>
        {actions.length > 0 ? (
          <div className="wk-agent-card-more-wrap">
            <button
              type="button"
              className={`wk-agent-more${menuOpen ? ' active-more' : ''}`}
              aria-label={t('agent.manageAgents')}
              aria-haspopup="menu"
              aria-expanded={menuOpen ? 'true' : 'false'}
              onClick={(event) => { event.stopPropagation(); onToggleMenu(menuOpen ? null : agent.id); }}
            >
              <span className="wk-agent-more-dot" /><span className="wk-agent-more-dot" /><span className="wk-agent-more-dot" />
            </button>
            {menuOpen ? (
              <div className="wk-agent-card-menu" role="menu" onClick={(event) => event.stopPropagation()}>
                {actions.map((action) => (
                  <button
                    key={action}
                    type="button"
                    role="menuitem"
                    data-action={action}
                    className={`wk-agent-card-menu-item${action === 'delete' ? ' is-danger' : ''}`}
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
      <div className="wk-agent-card-content">
        <div className="wk-agent-card-description">{agent.description || t('agent.noDescription')}</div>
      </div>
      <div className="wk-agent-card-bottom">
        <div className="wk-agent-card-badges">
          {agent.disabledByMe ? <span className="wk-agent-disabled-badge">{t('agent.disabled')}</span> : null}
          {badges.map((key) => (
            <span key={key} className={`wk-agent-feature-badge badge-${key}`} title={key === 'modeNormal' || key === 'modeAgent' ? t(modeTitleKey) : t(FEATURE_BADGE_TITLE_KEYS[key]!)}>
              <FeatureBadgeSvg badge={key} />
            </span>
          ))}
        </div>
        {!agent.isMine ? (
          <span className="wk-agent-card-source"><Icon name="usergroup" size={12} />{agent.orgName}</span>
        ) : badge ? (
          <span className={`wk-agent-corner-badge is-${badge.kind}`}>
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
    <div className="wk-agent-drawer-overlay" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <aside className="wk-agent-drawer" role="dialog" aria-label={t('agent.detail.title')}>
        <div className="wk-agent-drawer-header">
          <h3 className="wk-agent-drawer-title">{t('agent.detail.title')}</h3>
          <button type="button" className="wk-agent-drawer-close" aria-label={t('common.cancel')} onClick={onClose}><Icon name="close" size={16} /></button>
        </div>
        <div className="wk-agent-drawer-body">
          <div className="wk-agent-drawer-row"><span className="wk-agent-drawer-label">{t('agent.title')}</span><span className="wk-agent-drawer-value">{agent.name}</span></div>
          <div className="wk-agent-drawer-row"><span className="wk-agent-drawer-label">{t('agent.noDescription')}</span><span className="wk-agent-drawer-value">{agent.description || t('agent.noDescription')}</span></div>
          <div className="wk-agent-drawer-row"><span className="wk-agent-drawer-label">{t('agent.shareScope.title')}</span><span className="wk-agent-drawer-value wk-agent-drawer-scope">
            <span>{t('agent.shareScope.knowledgeBase')}: {scopeText(kbScope(config), t, 'agent.shareScope.kbAll', 'agent.shareScope.kbSelected', 'agent.shareScope.kbNone')}</span>
            <span>{t('agent.shareScope.chatModel')}: {config?.model_id ? t('agent.shareScope.modelConfigured') : t('agent.shareScope.modelNotSet')}</span>
            <span>{t('agent.shareScope.webSearch')}: {config?.web_search_enabled ? t('agent.shareScope.enabled') : t('agent.shareScope.disabled')}</span>
            <span>{t('agent.shareScope.mcp')}: {scopeText(mcpScope(config), t, 'agent.shareScope.mcpAll', 'agent.shareScope.mcpSelected', 'agent.shareScope.mcpNone')}</span>
          </span></div>
        </div>
        <div className="wk-agent-drawer-footer">
          <button type="button" className="wk-agent-btn wk-agent-btn-primary wk-agent-btn-block" onClick={() => onUseInChat(agent)}>{t('agent.detail.useInChat')}</button>
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
    <div className="wk-agent-dialog-overlay" onClick={(event) => { if (event.target === event.currentTarget) onCancel(); }}>
      <div className="wk-agent-dialog" role="alertdialog" aria-label={t('agent.delete.confirmTitle')}>
        <div className="wk-agent-dialog-header">
          <span className="wk-agent-dialog-icon" aria-hidden="true"><Icon name="delete" size={18} /></span>
          <span className="wk-agent-dialog-title">{t('agent.delete.confirmTitle')}</span>
        </div>
        <p className="wk-agent-dialog-text">{t('agent.delete.confirmMessage', { name: agent.name })}</p>
        <div className="wk-agent-dialog-actions">
          <button type="button" className="wk-agent-dialog-btn" onClick={onCancel}>{t('common.cancel')}</button>
          <button type="button" className="wk-agent-dialog-btn is-confirm" disabled={busy} onClick={onConfirm}>{busy ? t('common.loading') : t('agent.delete.confirmButton')}</button>
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
  rail: AgentRailItem[];
  sections: AgentSectionView[];
  flatCards: AgentCardModel[];
  isSectioned: boolean;
  favorites: ReadonlySet<string>;
  openMenuId: string | null;
  error: string | null;
  notice: string | null;
  drawer: { kind: 'shared'; agent: AgentCardModel } | null;
  editor: { mode: 'create' | 'edit'; agent: AgentCardModel | null } | null;
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
      <div className="wk-agent-empty">
        <span className="wk-agent-empty-icon"><Icon name="star" size={48} /></span>
        <span className="wk-agent-empty-title">{t('agent.empty.favoritesTitle')}</span>
        <span className="wk-agent-empty-desc">{t('agent.empty.favoritesDescription')}</span>
      </div>
    );
  }
  if (space === 'recents') {
    return (
      <div className="wk-agent-empty">
        <span className="wk-agent-empty-icon"><Icon name="history" size={48} /></span>
        <span className="wk-agent-empty-title">{t('agent.empty.recentsTitle')}</span>
        <span className="wk-agent-empty-desc">{t('agent.empty.recentsDescription')}</span>
      </div>
    );
  }
  if (space === 'all' || space === 'mine') {
    return (
      <div className="wk-agent-empty">
        <span className="wk-agent-empty-title">{t('agent.empty.title')}</span>
        <span className="wk-agent-empty-desc">{t('agent.empty.description')}</span>
        {canCreate ? (
          <button type="button" className="wk-agent-create-btn wk-agent-empty-btn" data-guide="agent-list-create" onClick={onCreate}><SparklesIcon size={18} /><span>{t('agent.createAgent')}</span></button>
        ) : null}
      </div>
    );
  }
  return (
    <div className="wk-agent-empty">
      <span className="wk-agent-empty-title">{t('agent.empty.sharedTitle')}</span>
      <span className="wk-agent-empty-desc">{t('agent.empty.sharedDescription')}</span>
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
  const { t, editorT, viewer, loading, space, rail, sections, flatCards, isSectioned, favorites, openMenuId, error, notice, drawer, editor, deleteTarget, deleting, collapsedSections, canCreate } = props;
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
    <main className="wk-agent-page">
      <AgentRail t={t} items={rail} onSelect={props.onSpaceChange} />
      <div className="wk-agent-content">
        <header className="wk-agent-header">
          <div className="wk-agent-header-title">
            <div className="wk-agent-title-row">
              <h2>{t('agent.title')}</h2>
              {canCreate ? (
                <button type="button" className="wk-agent-header-create" data-guide="agent-list-create" title={t('agent.createAgent')} aria-label={t('agent.createAgent')} onClick={props.onCreate}><SparklesIcon /></button>
              ) : null}
            </div>
            <p className="wk-agent-header-subtitle">{t('agent.subtitle')}</p>
          </div>
          {canCreate ? (
            <button type="button" className="wk-agent-create-btn" data-guide="agent-list-create" onClick={props.onCreate}><SparklesIcon size={18} /><span>{t('agent.createAgent')}</span></button>
          ) : null}
        </header>
        {error ? <Status tone="error">{error}</Status> : null}
        {notice ? <Status tone="success">{notice}</Status> : null}
        {loading ? <div className="wk-agent-grid" aria-busy="true">{Array.from({ length: 6 }, (_, index) => <div className="wk-agent-skeleton" key={index} />)}</div> : null}
        {!loading && hasCards ? (
          <div className="wk-agent-grid">
            {isSectioned
              ? sections.map((section) => (
                  <AgentSection key={section.key} section={section} t={t} viewer={viewer} collapsed={collapsedSections.has(section.key)} onToggle={props.onToggleSection} cardProps={cardProps} />
              ))
              : flatCards.map((agent) => <AgentCard key={agent.id} {...cardProps(agent)} />)}
          </div>
        ) : null}
        {!loading && !hasCards ? <EmptyState t={t} space={space} canCreate={canCreate} onCreate={props.onCreate} /> : null}
      </div>
      {drawer ? <AgentDetailDrawer kind={drawer.kind} agent={drawer.agent} t={t} onClose={props.onCloseDrawer} onUseInChat={props.onUseInChat} /> : null}
      {editor ? <AgentEditorModal open mode={editor.mode} agent={editor.agent} client={props.client} t={editorT} onClose={props.onCloseEditor} onSaved={props.onEditorSaved} /> : null}
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

function resolveLocale(): Locale {
  const language = typeof navigator !== 'undefined' ? navigator.language : 'en-US';
  return isLocale(language) ? language : isLocale(language.split('-')[0] ?? '') ? (language.split('-')[0] as Locale) : 'en-US';
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

export function AgentsPage({ client, tenantId }: AgentsPageProps) {
  const locale = useMemo(resolveLocale, []);
  const t = useCallback<Translate>((key, values) => formatMessage(locale, key, values), [locale]);
  // agentEditor.* copy is not in packages/i18n yet; the editor falls back to
  // ported literals for those keys while agent.* resolves normally.
  const editorT = useMemo(() => makeEditorT(locale), [locale]);
  const [viewer, setViewer] = useState<AgentViewer>({ userId: '', isAdmin: false, isContributor: false });
  const [data, setData] = useState<AgentsPageData | null>(null);
  const [spaceItems, setSpaceItems] = useState<Array<Record<string, unknown>>>([]);
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
    void client.auth.me().then((me) => {
      if (!active) return;
      const role = membershipRoleOf(me.memberships, tenantKey);
      const userId = typeof me.user?.id === 'string' ? me.user.id : '';
      setViewer({
        userId,
        isAdmin: role === 'owner' || role === 'admin' || me.user?.is_system_admin === true,
        isContributor: role === 'owner' || role === 'admin' || role === 'contributor' || me.user?.is_system_admin === true,
      });
      setFavorites(new Set(readFavoriteIds(window.localStorage, userId, tenantKey)));
      setRecents(readAgentRecents(window.localStorage, userId, tenantKey));
    }).catch(() => { /* pins fall back to the unscoped '' user bucket */ });
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

  // Default scope follows the Vue role rule: contributor → mine, else all
  // (AgentList.vue defaultScope). Resolved once here so rows, sections, and
  // the rail all agree even before the user picks a scope.
  const effectiveSpace = space || (viewer.isContributor ? 'mine' : 'all');

  // Space view fetch (GET /organizations/:id/shared-agents — routes_agent.go:171).
  useEffect(() => {
    if (!effectiveSpace || RESERVED_SCOPES.has(effectiveSpace)) { setSpaceItems([]); return; }
    let active = true;
    void client.identity.organizations.agentShares.listInOrganization(effectiveSpace).then((rows) => {
      if (active) setSpaceItems(rows);
    }).catch(() => { if (active) setSpaceItems([]); });
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
    // AgentList.vue handleCardClick: shared → detail drawer, own → editor.
    // The full AgentEditorModal now owns own-agent editing (Vue parity).
    if (agent.isMine) { setEditor({ mode: 'edit', agent }); return; }
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

  const onCreate = useCallback(() => { setEditor({ mode: 'create', agent: null }); }, []);

  const onEditorSaved = useCallback((_agent: Record<string, unknown>, mode: 'create' | 'edit') => {
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
    window.location.assign(chatNavigationPath(agent));
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

  return (
    <AgentsPageView
      t={t}
      editorT={editorT}
      client={client}
      viewer={viewer}
      loading={!data && !loadError}
      space={effectiveSpace}
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
