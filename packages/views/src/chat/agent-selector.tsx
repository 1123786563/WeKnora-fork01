import { useEffect, useRef, useState } from 'react';
import { formatChatCopy, type ChatCopyTable } from './chat-copy.ts';
import {
  getAgentNotReadyReasonKeys,
  resolveAgentNotReadyHighlight,
  resolveAgentNotReadySection,
  type AgentNotReadyReasonKey,
} from './agent-readiness.ts';

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

function missingItemLabel(copy: ChatCopyTable, key: AgentNotReadyReasonKey): string {
  return key === 'rerank_model' ? copy.agentMissingRerankModel : copy.agentMissingChatModel;
}

/**
 * R484 D14 — Vue AgentSelector.vue:31-35 renders TDesign `error-circle` (a
 * circled exclamation mark) as the not-ready marker. The bare ⚠ text glyph
 * diverged visually from the Vue baseline; this svg mirrors the TDesign
 * error-circle geometry.
 */
function NotReadyMarker(props: { agentId: string; label: string }) {
  return (
    <span
      role="img"
      data-agent-not-ready={props.agentId}
      aria-label={props.label}
      title={props.label}
      className="wk-vc-agent-selector-1"
    >
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
        <circle cx="7" cy="7" r="5.6" stroke="currentColor" strokeWidth="1.3" />
        <line x1="7" y1="4" x2="7" y2="7.6" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
        <circle cx="7" cy="10" r="0.8" fill="currentColor" />
      </svg>
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

  function selectQuickAnswer(): void {
    props.onSelect('', null);
  }

  function configureAgent(agent: AgentSelectorAgent): void {
    const keys = notReadyKeysFor(agent);
    props.onConfigureAgent(agent, resolveAgentNotReadySection(keys), resolveAgentNotReadyHighlight(keys));
  }

  const builtinAgents = agents.filter((agent) => agent.is_builtin === true);
  const customAgents = agents.filter((agent) => agent.is_builtin !== true);
  const activeDetail = activeDetailId === null ? null
    : (agents.find((agent) => agent.id === activeDetailId) ?? null);
  const quickAnswerActive = currentAgentId === '';

  const anchor = props.anchorRect;
  const dropdownLeft = Math.max(16, Math.min(anchor.left, window.innerWidth - DROPDOWN_WIDTH - 16));
  const openDownward = window.innerHeight - anchor.bottom >= 120;
  const dropdownTop = openDownward ? anchor.bottom + 6 : Math.max(20, anchor.top - 6 - Math.min(280, anchor.top - 26));
  const dropdownMaxHeight = openDownward
    ? Math.min(280, window.innerHeight - anchor.bottom - 22)
    : Math.min(280, anchor.top - 26);

  const dropdown = (
    <div className="wk-agent-selector-overlay wk-vc-agent-selector-2" onClick={props.onClose}>
      <div
        role="dialog"
        aria-label={copy.selectAgent}
        className="wk-agent-selector-dropdown wk-vc-agent-selector-3"
        style={{ left: dropdownLeft, top: dropdownTop, width: DROPDOWN_WIDTH, maxHeight: dropdownMaxHeight }}
        onClick={(event) => event.stopPropagation()}
      >
        <div className="wk-vc-agent-selector-4">
          <span className="wk-vc-agent-selector-5">{copy.selectAgent}</span>
          <button
            type="button"
            className="wk-vc-agent-selector-6"
            onClick={() => { props.onClose(); props.onManage(); }}
          >
            <span aria-hidden="true">+</span>{copy.agentManageAgents}
          </button>
        </div>
        <div className="wk-vc-agent-selector-7">
          {agents.length === 0 ? (
            <div className="wk-vc-agent-selector-8">{copy.agentNoAgents}</div>
          ) : null}
          {builtinAgents.length > 0 ? (
            <div className="wk-agent-selector-group">
              <div className="wk-vc-agent-selector-9">{copy.agentBuiltinGroup}</div>
              {builtinAgents.map((agent) => (
                <button
                  type="button"
                  key={agent.id}
                  ref={(el) => { if (el) optionRefs.current.set(agent.id, el); }}
                  data-agent-id={agent.id}
                  className={`wk-vc-agent-selector-27 ${currentAgentId === agent.id ? 'wk-vc-agent-selector-28' : 'wk-vc-agent-selector-29'}`}
                  onMouseEnter={() => onOptionEnter(agent)}
                  onMouseLeave={onOptionLeave}
                  onFocus={() => onOptionEnter(agent)}
                  onClick={() => selectAgent(agent)}
                >
                  <span className="wk-agent-option-icon wk-vc-agent-selector-10" aria-hidden="true">{agent.config?.agent_mode === 'smart-reasoning' ? '✦' : '💬'}</span>
                  <span className="wk-vc-agent-selector-11">{agent.name}</span>
                  {notReadyKeysFor(agent).length > 0 ? (
                    <NotReadyMarker agentId={agent.id} label={formatChatCopy(copy, 'agentNotReadyHint', { items: agentNotReadyLabels(copy, notReadyKeysFor(agent)).join('、') })} />
                  ) : null}
                </button>
              ))}
            </div>
          ) : null}
          {customAgents.length > 0 ? (
            <div className="wk-agent-selector-group">
              <div className="wk-vc-agent-selector-9">{copy.agentCustomGroup}</div>
              {customAgents.map((agent) => (
                <button
                  type="button"
                  key={agent.id}
                  ref={(el) => { if (el) optionRefs.current.set(agent.id, el); }}
                  data-agent-id={agent.id}
                  className={`wk-vc-agent-selector-27 ${currentAgentId === agent.id ? 'wk-vc-agent-selector-28' : 'wk-vc-agent-selector-29'}`}
                  onMouseEnter={() => onOptionEnter(agent)}
                  onMouseLeave={onOptionLeave}
                  onFocus={() => onOptionEnter(agent)}
                  onClick={() => selectAgent(agent)}
                >
                  <span className="wk-vc-agent-selector-12" aria-hidden="true">{agent.name.slice(0, 1).toUpperCase()}</span>
                  <span className="wk-vc-agent-selector-11">{agent.name}</span>
                  {notReadyKeysFor(agent).length > 0 ? (
                    <NotReadyMarker agentId={agent.id} label={formatChatCopy(copy, 'agentNotReadyHint', { items: agentNotReadyLabels(copy, notReadyKeysFor(agent)).join('、') })} />
                  ) : null}
                </button>
              ))}
            </div>
          ) : null}
        </div>
      </div>
      {activeDetail && detailRect ? <AgentDetailCard
        copy={copy}
        agent={activeDetail}
        isCurrent={activeDetail.id === currentAgentId}
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
