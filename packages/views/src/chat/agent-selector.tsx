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
    <div className="wk-agent-selector-overlay fixed inset-0 z-[10000]" onClick={props.onClose}>
      <div
        role="dialog"
        aria-label={copy.selectAgent}
        className="wk-agent-selector-dropdown absolute flex flex-col overflow-hidden rounded-[10px] border-[0.5px] border-[#e7e7e7] bg-white shadow-[0_4px_16px_rgba(0,0,0,0.12)]"
        style={{ left: dropdownLeft, top: dropdownTop, width: DROPDOWN_WIDTH, maxHeight: dropdownMaxHeight }}
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-[#f0f0f0] px-[12px] py-[8px]">
          <span className="text-[13px] font-medium text-[rgba(0,0,0,0.9)]">{copy.selectAgent}</span>
          <button
            type="button"
            className="flex cursor-pointer items-center gap-[2px] border-0 bg-transparent p-0 text-[12px] text-[#245a9b] hover:underline"
            onClick={() => { props.onClose(); props.onManage(); }}
          >
            <span aria-hidden="true">+</span>{copy.agentManageAgents}
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-auto py-[4px]">
          {agents.length === 0 ? (
            <div className="px-[12px] py-[10px] text-[13px] text-[rgba(0,0,0,0.4)]">{copy.agentNoAgents}</div>
          ) : null}
          {builtinAgents.length > 0 ? (
            <div className="wk-agent-selector-group">
              <div className="px-[12px] pt-[6px] pb-[2px] text-[11px] text-[rgba(0,0,0,0.4)]">{copy.agentBuiltinGroup}</div>
              {builtinAgents.map((agent) => (
                <button
                  type="button"
                  key={agent.id}
                  ref={(el) => { if (el) optionRefs.current.set(agent.id, el); }}
                  data-agent-id={agent.id}
                  className={`flex w-full cursor-pointer items-center gap-[6px] border-0 bg-transparent px-[12px] py-[7px] text-left text-[13px] hover:bg-[#f5f5f5] ${currentAgentId === agent.id ? 'bg-[#eef4fb] font-medium text-[rgba(0,0,0,0.9)]' : 'text-[rgba(0,0,0,0.75)]'}`}
                  onMouseEnter={() => onOptionEnter(agent)}
                  onMouseLeave={onOptionLeave}
                  onFocus={() => onOptionEnter(agent)}
                  onClick={() => selectAgent(agent)}
                >
                  <span className="wk-agent-option-icon min-w-[16px] text-center" aria-hidden="true">{agent.config?.agent_mode === 'smart-reasoning' ? '✦' : '💬'}</span>
                  <span className="min-w-0 flex-1 truncate">{agent.name}</span>
                  {notReadyKeysFor(agent).length > 0 ? (
                    <span role="img" aria-label={formatChatCopy(copy, 'agentNotReadyHint', { items: agentNotReadyLabels(copy, notReadyKeysFor(agent)).join('、') })} title={formatChatCopy(copy, 'agentNotReadyHint', { items: agentNotReadyLabels(copy, notReadyKeysFor(agent)).join('、') })} className="shrink-0 text-[13px] text-[#e3730e]">⚠</span>
                  ) : null}
                </button>
              ))}
            </div>
          ) : null}
          {customAgents.length > 0 ? (
            <div className="wk-agent-selector-group">
              <div className="px-[12px] pt-[6px] pb-[2px] text-[11px] text-[rgba(0,0,0,0.4)]">{copy.agentCustomGroup}</div>
              {customAgents.map((agent) => (
                <button
                  type="button"
                  key={agent.id}
                  ref={(el) => { if (el) optionRefs.current.set(agent.id, el); }}
                  data-agent-id={agent.id}
                  className={`flex w-full cursor-pointer items-center gap-[6px] border-0 bg-transparent px-[12px] py-[7px] text-left text-[13px] hover:bg-[#f5f5f5] ${currentAgentId === agent.id ? 'bg-[#eef4fb] font-medium text-[rgba(0,0,0,0.9)]' : 'text-[rgba(0,0,0,0.75)]'}`}
                  onMouseEnter={() => onOptionEnter(agent)}
                  onMouseLeave={onOptionLeave}
                  onFocus={() => onOptionEnter(agent)}
                  onClick={() => selectAgent(agent)}
                >
                  <span className="min-w-[16px] text-center text-[12px] text-[rgba(0,0,0,0.4)]" aria-hidden="true">{agent.name.slice(0, 1).toUpperCase()}</span>
                  <span className="min-w-0 flex-1 truncate">{agent.name}</span>
                  {notReadyKeysFor(agent).length > 0 ? (
                    <span role="img" aria-label={formatChatCopy(copy, 'agentNotReadyHint', { items: agentNotReadyLabels(copy, notReadyKeysFor(agent)).join('、') })} title={formatChatCopy(copy, 'agentNotReadyHint', { items: agentNotReadyLabels(copy, notReadyKeysFor(agent)).join('、') })} className="shrink-0 text-[13px] text-[#e3730e]">⚠</span>
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
      className="wk-agent-detail-card fixed z-[10002] box-border rounded-[10px] border-[0.5px] border-[#e7e7e7] bg-white p-[12px] shadow-[0_4px_16px_rgba(0,0,0,0.12)]"
      style={{ left, top: Math.max(8, Math.min(props.anchorRect.top, window.innerHeight - 260)), width: DETAIL_PANEL_WIDTH }}
      onMouseEnter={props.onEnter}
      onMouseLeave={props.onLeave}
      data-agent-detail={agent.id}
    >
      <div className="flex items-start gap-[6px]">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-[6px]">
            <span className="truncate text-[13px] font-medium text-[rgba(0,0,0,0.9)]">{agent.name}</span>
            <button
              type="button"
              className={`shrink-0 cursor-pointer border-0 bg-transparent p-0 text-[12px] ${notReadyKeys.length > 0 ? 'text-[#e3730e]' : 'text-[rgba(0,0,0,0.45)]'}`}
              title={notReadyKeys.length > 0 ? copy.agentConfigureAction : copy.agentGoToSettings}
              aria-label={notReadyKeys.length > 0 ? copy.agentConfigureAction : copy.agentGoToSettings}
              onClick={props.onConfigure}
            >
              {notReadyKeys.length > 0 ? `↪ ${copy.agentConfigureAction}` : `⚙ ${copy.agentGoToSettings}`}
            </button>
          </div>
          {props.isCurrent ? (
            <span className="mt-[2px] inline-block rounded-[4px] bg-[#eef4fb] px-[4px] py-[1px] text-[11px] text-[#245a9b]">{copy.agentSelectorCurrent}</span>
          ) : null}
          {notReadyKeys.length > 0 ? (
            <div className="mt-[2px] flex flex-wrap items-center gap-[4px] text-[11px] text-[#e3730e]">
              <span>⚠ {copy.agentNotReadyStatus}</span>
              {agentNotReadyLabels(copy, notReadyKeys).map((item) => <span key={item} className="rounded-[3px] bg-[#fdf1e7] px-[3px]">{item}</span>)}
            </div>
          ) : null}
        </div>
      </div>
      <p className="m-[8px_0_6px] line-clamp-3 text-[12px] leading-[18px] text-[rgba(0,0,0,0.55)]">{agent.description || copy.agentNoDescription}</p>
      <div className="flex flex-wrap gap-[4px]">
        <span className="rounded-[4px] bg-[#f2f3f5] px-[6px] py-[2px] text-[11px] text-[rgba(0,0,0,0.65)]">{config.agent_mode === 'smart-reasoning' ? copy.agentModeTagReasoning : copy.agentModeTagQuick}</span>
        {kbLabel ? <span className="rounded-[4px] bg-[#f2f3f5] px-[6px] py-[2px] text-[11px] text-[rgba(0,0,0,0.65)]">{kbLabel}</span> : null}
        {multiTurn ? <span className="rounded-[4px] bg-[#f2f3f5] px-[6px] py-[2px] text-[11px] text-[rgba(0,0,0,0.65)]">{copy.agentMultiTurn}</span> : null}
      </div>
      <div className="mt-[8px] border-t border-[#f0f0f0] pt-[6px]">
        <div className="mb-[4px] text-[11px] text-[rgba(0,0,0,0.4)]">{copy.agentCapabilitiesSection}</div>
        <ul className="m-0 flex list-none flex-col gap-[3px] p-0">
          <li className={`flex items-center justify-between rounded-[4px] px-[6px] py-[2px] text-[11px] ${webSearchEnabled ? 'bg-[#eefaf2] text-[rgba(0,0,0,0.7)]' : 'bg-[#f7f7f7] text-[rgba(0,0,0,0.45)]'}`}>
            <span>{copy.agentCapWebSearch}</span>
            <span>{webSearchEnabled ? copy.agentCapOn : copy.agentCapOff}</span>
          </li>
          <li className={`flex items-center justify-between rounded-[4px] px-[6px] py-[2px] text-[11px] ${imageUpload ? 'bg-[#eefaf2] text-[rgba(0,0,0,0.7)]' : 'bg-[#f7f7f7] text-[rgba(0,0,0,0.45)]'}`}>
            <span>{copy.agentCapImageUpload}</span>
            <span>{imageUpload ? copy.agentCapSupported : copy.agentCapUnsupported}</span>
          </li>
        </ul>
      </div>
    </aside>
  );
}
