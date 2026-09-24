import { useEffect, useRef, useState } from 'react';
import { formatChatCopy, type ChatCopyTable } from './chat-copy.ts';
import {
  getAgentNotReadyReasonKeys,
  resolveAgentNotReadyHighlight,
  resolveAgentNotReadySection,
  type AgentNotReadyReasonKey,
} from './agent-readiness.ts';
import { SpriteIcon } from './message-face.tsx';

/**
 * Agent picker panel, ported from upstream AgentSelector.vue (2026-09-18
 * round 8): grouped options with a manage-agents entry, chat-readiness
 * gating with per-item missing-config labels, and a hover detail card with
 * capability badges. The browser-language sniffing and Vue store wiring are
 * replaced by props; all hooks stay at the top of each component.
 */

export interface AgentSelectorAgent {
  id: string;
  name: string;
  disabled?: boolean;
  description?: string;
  is_builtin?: boolean;
  /** Vue CustomAgent.avatar emoji fallback before the letter avatar. */
  avatar?: string;
  config?: Record<string, unknown>;
}

export interface AgentSelectorModel {
  id: string;
  type?: string;
}

export interface AgentSelectorProps {
  copy: ChatCopyTable;
  /** Empty id means the built-in quick-answer default. */
  currentAgentId: string;
  agents: readonly AgentSelectorAgent[];
  models: readonly AgentSelectorModel[];
  anchorRect: DOMRect;
  onSelect(agentId: string, agent: AgentSelectorAgent | null): void;
  onNotReady(agent: AgentSelectorAgent, labels: string[], keys: AgentNotReadyReasonKey[]): void;
  /** Opens the agents management page. */
  onManage(): void;
  /** Opens the agent editor at the section that fixes missing config. */
  onConfigureAgent(agent: AgentSelectorAgent, section: string, highlight?: AgentNotReadyReasonKey): void;
  onClose(): void;
}

const DETAIL_PANEL_WIDTH = 216;
const DETAIL_PANEL_GAP = 8;
const DETAIL_HIDE_DELAY_MS = 400;
const DROPDOWN_WIDTH = 220;

/** Vue api/agent.ts BUILTIN_QUICK_ANSWER_ID / BUILTIN_SMART_REASONING_ID. */
const BUILTIN_QUICK_ANSWER_ID = 'builtin-quick-answer';
const BUILTIN_SMART_REASONING_ID = 'builtin-smart-reasoning';

/** Vue AgentSelector.vue updateDropdownPosition (544-592) — ported verbatim:
 * opens below while 100px+6 fits under the anchor, otherwise anchors the
 * dropdown's bottom edge 6px above the trigger top (bottom CSS, not a
 * top-minus-maxHeight computation that leaves a floating gap). */
function vueDropdownStyle(anchor: DOMRect): React.CSSProperties {
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const dropdownWidth = DROPDOWN_WIDTH;
  const offsetY = 6;

  let left = Math.floor(anchor.left);
  const minLeft = 16;
  const maxLeft = Math.max(16, vw - dropdownWidth - 16);
  left = Math.max(minLeft, Math.min(maxLeft, left));

  const preferredDropdownHeight = 280;
  const minDropdownHeight = 100;
  const topMargin = 20;
  const spaceBelow = vh - anchor.bottom;
  const spaceAbove = anchor.top;

  if (spaceBelow >= minDropdownHeight + offsetY) {
    return {
      position: 'fixed',
      width: `${dropdownWidth}px`,
      left: `${left}px`,
      top: `${Math.floor(anchor.bottom + offsetY)}px`,
      maxHeight: `${Math.min(preferredDropdownHeight, spaceBelow - offsetY - 16)}px`,
      zIndex: 10001,
    };
  }
  const availableHeight = spaceAbove - offsetY - topMargin;
  const actualHeight = availableHeight >= preferredDropdownHeight
    ? preferredDropdownHeight
    : Math.max(minDropdownHeight, availableHeight);
  return {
    position: 'fixed',
    width: `${dropdownWidth}px`,
    left: `${left}px`,
    bottom: `${vh - anchor.top + offsetY}px`,
    maxHeight: `${actualHeight}px`,
    zIndex: 10001,
  };
}

/** Vue AgentAvatar.vue (size=small) — hashed gradient + first letter. The
 *  sparkles decoration is display:none on the small variant. */
function agentAvatarGradient(name: string): { from: string; to: string } {
  const gradients = [
    { from: '#667eea', to: '#764ba2' }, { from: '#4facfe', to: '#00f2fe' },
    { from: '#43e97b', to: '#38f9d7' }, { from: '#11998e', to: '#38ef7d' },
    { from: '#5ee7df', to: '#b490ca' }, { from: '#48c6ef', to: '#6f86d6' },
    { from: '#a8edea', to: '#fed6e3' }, { from: '#667db6', to: '#0082c8' },
    { from: '#36d1dc', to: '#5b86e5' }, { from: '#56ab2f', to: '#a8e063' },
    { from: '#614385', to: '#516395' }, { from: '#02aab0', to: '#00cdac' },
    { from: '#6a82fb', to: '#fc5c7d' }, { from: '#834d9b', to: '#d04ed6' },
    { from: '#4776e6', to: '#8e54e9' }, { from: '#00b09b', to: '#96c93d' },
  ];
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = ((hash << 5) - hash) + name.charCodeAt(i);
    hash = hash & hash;
  }
  return gradients[Math.abs(hash) % gradients.length]!;
}

function AgentAvatar({ name }: { name: string }) {
  const trimmed = name.trim();
  const firstChar = trimmed.charAt(0);
  const letter = !firstChar ? '?' : (/[a-zA-Z]/.test(firstChar) ? firstChar.toUpperCase() : firstChar);
  const g = agentAvatarGradient(trimmed);
  return (
    <div
      className="agent-avatar agent-avatar-small"
      style={{ background: `linear-gradient(135deg, ${g.from} 0%, ${g.to} 100%)` }}
      aria-hidden="true"
    >
      <span className="agent-avatar-letter" style={{ textShadow: `0 1px 2px ${g.to}80, 0 0 8px ${g.from}30` }}>{letter}</span>
    </div>
  );
}

/** Vue AgentSelector.vue builtin icon boxes (22×22 rounded, brand-tinted). */
function AgentOptionIcon({ agent }: { agent: AgentSelectorAgent }) {
  if (agent.id === BUILTIN_QUICK_ANSWER_ID || agent.id === BUILTIN_SMART_REASONING_ID) {
    const smart = agent.config?.agent_mode === 'smart-reasoning';
    return (
      <div className={'builtin-icon ' + (smart ? 'agent' : 'normal')}>
        <SpriteIcon name={smart ? 'control-platform' : 'chat'} size="13px" />
      </div>
    );
  }
  if (agent.avatar) {
    return <div className="builtin-avatar">{agent.avatar}</div>;
  }
  return (
    <div className="builtin-icon normal">
      <SpriteIcon name="app" size="13px" />
    </div>
  );
}

function missingItemLabel(copy: ChatCopyTable, key: AgentNotReadyReasonKey): string {
  return key === 'rerank_model' ? copy.agentMissingRerankModel : copy.agentMissingChatModel;
}

/**
 * R484 D14 — Vue AgentSelector.vue:31-35 renders TDesign `error-circle` (a
 * circled exclamation mark) as the not-ready marker inside the trailing
 * .agent-option-actions slot (22×22). The bare ⚠ text glyph diverged
 * visually from the Vue baseline; the sprite use mirrors the TDesign
 * error-circle geometry (台账 #10).
 */
function NotReadyMarker(props: { agentId: string; label: string }) {
  return (
    <span
      role="img"
      data-agent-not-ready={props.agentId}
      aria-label={props.label}
      title={props.label}
      className="agent-option-actions"
    >
      <SpriteIcon name="error-circle" className="not-ready-icon" size="14px" />
    </span>
  );
}

export function agentNotReadyLabels(copy: ChatCopyTable, keys: readonly AgentNotReadyReasonKey[]): string[] {
  return keys.map((key) => missingItemLabel(copy, key));
}

/** Upstream getKbCapability: empty when KB tools cannot run. */
function kbCapabilityLabel(copy: ChatCopyTable, agent: AgentSelectorAgent): string | null {
  const config = agent.config ?? {};
  if (config.kb_selection_mode === 'none') return null;
  const kbs = Array.isArray(config.knowledge_bases) ? config.knowledge_bases : [];
  if (kbs.length > 0) return formatChatCopy(copy, 'agentKbCount', { count: kbs.length });
  if (config.kb_selection_mode === 'all') return copy.agentKbAll;
  return null;
}

export function AgentSelectorPanel(props: AgentSelectorProps) {
  const { copy, currentAgentId, agents, models } = props;
  const [activeDetailId, setActiveDetailId] = useState<string | null>(null);
  const [detailRect, setDetailRect] = useState<DOMRect | null>(null);
  const hideTimer = useRef<number | null>(null);
  const optionRefs = useRef(new Map<string, HTMLElement>());

  useEffect(() => () => { if (hideTimer.current !== null) window.clearTimeout(hideTimer.current); }, []);

  function clearHideTimer(): void {
    if (hideTimer.current !== null) {
      window.clearTimeout(hideTimer.current);
      hideTimer.current = null;
    }
  }

  function notReadyKeysFor(agent: AgentSelectorAgent): AgentNotReadyReasonKey[] {
    const config = agent.config as Parameters<typeof getAgentNotReadyReasonKeys>[0] | undefined;
    return getAgentNotReadyReasonKeys(config, models, { isAgentMode: config?.agent_mode === 'smart-reasoning' });
  }

  function onOptionEnter(agent: AgentSelectorAgent): void {
    clearHideTimer();
    setActiveDetailId(agent.id);
    const el = optionRefs.current.get(agent.id);
    if (el) setDetailRect(el.getBoundingClientRect());
  }

  function onOptionLeave(): void {
    clearHideTimer();
    hideTimer.current = window.setTimeout(() => setActiveDetailId(null), DETAIL_HIDE_DELAY_MS);
  }

  function selectAgent(agent: AgentSelectorAgent): void {
    const keys = notReadyKeysFor(agent);
    if (keys.length > 0) {
      props.onNotReady(agent, agentNotReadyLabels(copy, keys), keys);
      return;
    }
    props.onSelect(agent.id, agent);
  }

  function configureAgent(agent: AgentSelectorAgent): void {
    const keys = notReadyKeysFor(agent);
    props.onConfigureAgent(agent, resolveAgentNotReadySection(keys), resolveAgentNotReadyHighlight(keys));
  }

  /* Vue AgentSelector.vue builtinAgents mapping — the two builtins always
   * display the localized mode name/description (input.normalMode /
   * input.agentMode), regardless of the backend row's stored name.
   * Vue selectedAgentId 恒为 agent id（快速问答=builtin-quick-answer）；React
   * host 以 '' 表示快速问答，此处对齐 Vue 口径再比较（selected 行高亮）。 */
  const effectiveCurrentAgentId = currentAgentId || BUILTIN_QUICK_ANSWER_ID;
  const builtinAgents = agents
    .filter((agent) => agent.is_builtin === true)
    .map((agent) => {
      if (agent.id === BUILTIN_QUICK_ANSWER_ID) return { ...agent, name: copy.quickAnswer, description: copy.inputNormalModeDesc };
      if (agent.id === BUILTIN_SMART_REASONING_ID) return { ...agent, name: copy.inputAgentMode, description: copy.inputAgentModeDesc };
      return agent;
    });
  const customAgents = agents.filter((agent) => agent.is_builtin !== true);
  const activeDetail = activeDetailId === null ? null
    : (agents.find((agent) => agent.id === activeDetailId) ?? null);

  const dropdownStyle = vueDropdownStyle(props.anchorRect);

  const dropdown = (
    <div className="agent-selector-overlay" onClick={props.onClose}>
      <div
        role="dialog"
        aria-label={copy.selectAgent}
        className="agent-selector-dropdown"
        style={dropdownStyle}
        onClick={(event) => event.stopPropagation()}
      >
        <div className="agent-selector-header">
          <span>{copy.selectAgent}</span>
          <button
            type="button"
            className="agent-selector-add"
            onClick={() => { props.onClose(); props.onManage(); }}
          >
            <span className="add-icon" aria-hidden="true">+</span><span className="add-text">{copy.agentManageAgents}</span>
          </button>
        </div>
        <div className="agent-selector-content">
          {builtinAgents.length > 0 ? (
            <div className="agent-group">
              <div className="agent-group-title">{copy.agentBuiltinGroup}</div>
              {builtinAgents.map((agent) => (
                <div
                  key={agent.id}
                  ref={(el) => { if (el) optionRefs.current.set(agent.id, el); }}
                  data-agent-id={agent.id}
                  className={'agent-option' + (effectiveCurrentAgentId === agent.id ? ' selected' : '')}
                  onMouseEnter={() => onOptionEnter(agent)}
                  onMouseLeave={onOptionLeave}
                  onFocus={() => onOptionEnter(agent)}
                  onClick={() => selectAgent(agent)}
                >
                  <AgentOptionIcon agent={agent} />
                  <span className="agent-option-name">{agent.name}</span>
                  {notReadyKeysFor(agent).length > 0 ? (
                    <NotReadyMarker agentId={agent.id} label={formatChatCopy(copy, 'agentNotReadyHint', { items: agentNotReadyLabels(copy, notReadyKeysFor(agent)).join('、') })} />
                  ) : null}
                </div>
              ))}
            </div>
          ) : null}
          {customAgents.length > 0 ? (
            <div className="agent-group">
              <div className="agent-group-title">{copy.agentCustomGroup}</div>
              {customAgents.map((agent) => (
                <div
                  key={agent.id}
                  ref={(el) => { if (el) optionRefs.current.set(agent.id, el); }}
                  data-agent-id={agent.id}
                  className={'agent-option' + (effectiveCurrentAgentId === agent.id ? ' selected' : '')}
                  onMouseEnter={() => onOptionEnter(agent)}
                  onMouseLeave={onOptionLeave}
                  onFocus={() => onOptionEnter(agent)}
                  onClick={() => selectAgent(agent)}
                >
                  <AgentAvatar name={agent.name} />
                  <span className="agent-option-name">{agent.name}</span>
                  {notReadyKeysFor(agent).length > 0 ? (
                    <NotReadyMarker agentId={agent.id} label={formatChatCopy(copy, 'agentNotReadyHint', { items: agentNotReadyLabels(copy, notReadyKeysFor(agent)).join('、') })} />
                  ) : null}
                </div>
              ))}
            </div>
          ) : null}
          {builtinAgents.length === 0 && customAgents.length === 0 ? (
            <div className="agent-option empty">{copy.agentNoAgents}</div>
          ) : null}
        </div>
      </div>
      {activeDetail && detailRect ? <AgentDetailCard
        copy={copy}
        agent={activeDetail}
        isCurrent={activeDetail.id === effectiveCurrentAgentId}
        notReadyKeys={notReadyKeysFor(activeDetail)}
        anchorRect={detailRect}
        onEnter={clearHideTimer}
        onLeave={onOptionLeave}
        onConfigure={() => { props.onClose(); configureAgent(activeDetail); }}
      /> : null}
    </div>
  );

  // Rendered in-tree instead of a portal (the package has no react-dom
  // dependency): the composer control bar sits outside the message scroll
  // area, so the fixed overlay is not clipped by any ancestor.
  return dropdown;
}

function AgentDetailCard(props: {
  copy: ChatCopyTable;
  agent: AgentSelectorAgent;
  isCurrent: boolean;
  notReadyKeys: readonly AgentNotReadyReasonKey[];
  anchorRect: DOMRect;
  onEnter(): void;
  onLeave(): void;
  onConfigure(): void;
}) {
  const { copy, agent, notReadyKeys } = props;
  const config = agent.config ?? {};
  const kbLabel = kbCapabilityLabel(copy, agent);
  const multiTurn = config.multi_turn_enabled === true;
  const webSearchEnabled = config.web_search_enabled === true;
  const imageUpload = config.image_upload_enabled === true;

  let left = props.anchorRect.right + DETAIL_PANEL_GAP;
  if (left + DETAIL_PANEL_WIDTH > window.innerWidth - 8) {
    left = Math.max(8, window.innerWidth - DETAIL_PANEL_WIDTH - 8);
  }
  return (
    <aside
      className="wk-agent-detail-card wk-vc-agent-selector-13"
      style={{ left, top: Math.max(8, Math.min(props.anchorRect.top, window.innerHeight - 260)), width: DETAIL_PANEL_WIDTH }}
      onMouseEnter={props.onEnter}
      onMouseLeave={props.onLeave}
      data-agent-detail={agent.id}
    >
      <div className="wk-vc-agent-selector-14">
        <div className="wk-vc-agent-selector-15">
          <div className="wk-vc-agent-selector-16">
            <span className="wk-vc-agent-selector-17">{agent.name}</span>
            <button
              type="button"
              className={`wk-vc-agent-selector-30 ${notReadyKeys.length > 0 ? 'wk-vc-agent-selector-31' : 'wk-vc-agent-selector-32'}`}
              title={notReadyKeys.length > 0 ? copy.agentConfigureAction : copy.agentGoToSettings}
              aria-label={notReadyKeys.length > 0 ? copy.agentConfigureAction : copy.agentGoToSettings}
              onClick={props.onConfigure}
            >
              {notReadyKeys.length > 0 ? `↪ ${copy.agentConfigureAction}` : `⚙ ${copy.agentGoToSettings}`}
            </button>
          </div>
          {props.isCurrent ? (
            <span className="wk-vc-agent-selector-18">{copy.agentSelectorCurrent}</span>
          ) : null}
          {notReadyKeys.length > 0 ? (
            <div className="wk-vc-agent-selector-19">
              <span>⚠ {copy.agentNotReadyStatus}</span>
              {agentNotReadyLabels(copy, notReadyKeys).map((item) => <span key={item} className="wk-vc-agent-selector-20">{item}</span>)}
            </div>
          ) : null}
        </div>
      </div>
      <p className="wk-vc-agent-selector-21">{agent.description || copy.agentNoDescription}</p>
      <div className="wk-vc-agent-selector-22">
        <span className="wk-vc-agent-selector-23">{config.agent_mode === 'smart-reasoning' ? copy.agentModeTagReasoning : copy.agentModeTagQuick}</span>
        {kbLabel ? <span className="wk-vc-agent-selector-23">{kbLabel}</span> : null}
        {multiTurn ? <span className="wk-vc-agent-selector-23">{copy.agentMultiTurn}</span> : null}
      </div>
      <div className="wk-vc-agent-selector-24">
        <div className="wk-vc-agent-selector-25">{copy.agentCapabilitiesSection}</div>
        <ul className="wk-vc-agent-selector-26">
          <li className={`wk-vc-agent-selector-33 ${webSearchEnabled ? 'wk-vc-agent-selector-34' : 'wk-vc-agent-selector-35'}`}>
            <span>{copy.agentCapWebSearch}</span>
            <span>{webSearchEnabled ? copy.agentCapOn : copy.agentCapOff}</span>
          </li>
          <li className={`wk-vc-agent-selector-33 ${imageUpload ? 'wk-vc-agent-selector-34' : 'wk-vc-agent-selector-35'}`}>
            <span>{copy.agentCapImageUpload}</span>
            <span>{imageUpload ? copy.agentCapSupported : copy.agentCapUnsupported}</span>
          </li>
        </ul>
      </div>
    </aside>
  );
}
