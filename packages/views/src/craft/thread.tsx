// CFT-S01-T007: the craft conversation supplement — tool and interaction
// cards rendered from the SAME projection the assistant-ui thread consumes
// (projectAssistant). Cards key on stable server ids (tool call_id,
// interaction id), show a human-readable execution summary (never model
// internals), and a child run's finish only ever marks its own card.
import React from 'react';
import type { CraftInteractionCard, CraftToolFact } from './presentation.ts';

const TOOL_STATUS_LABELS: Record<string, string> = {
  running: '执行中',
  succeeded: '已完成',
  failed: '失败',
  unknown: '结果待核对',
};

export function toolStatusLabel(status: string): string {
  return TOOL_STATUS_LABELS[status] ?? status;
}

/** One tool execution card keyed by the stable server call id. */
export function CraftToolFactCard({ fact }: { fact: CraftToolFact }) {
  return (
    <li className="wk-craft-card" data-call-id={fact.callId} data-status={fact.status}>
      <summary>
        {fact.tool} · {toolStatusLabel(fact.status)}
      </summary>
    </li>
  );
}

/**
 * The tool-fact strip rendered under an assistant message. Replay-safe:
 * React keys are the server call ids, so a reconnect replay re-renders the
 * same cards instead of appending duplicates.
 */
export function CraftToolFactList({ facts }: { facts: readonly CraftToolFact[] }) {
  if (facts.length === 0) return null;
  return (
    <ul className="wk-craft-tool-facts" aria-label="工具执行摘要">
      {facts.map((fact) => (
        <CraftToolFactCard key={fact.callId || `seq-${fact.seq}`} fact={fact} />
      ))}
    </ul>
  );
}

const INTERACTION_KIND_LABELS: Record<string, string> = {
  question: '待回答',
  permission: '待批准',
  unknown: '待核对',
};

/**
 * Interaction summary line (the full decision UI stays interaction.tsx's
 * CraftInteractionPanel). Child-run finish never resolves a card here.
 */
export function CraftInteractionSummary({ card }: { card: CraftInteractionCard }) {
  const label = INTERACTION_KIND_LABELS[card.kind] ?? card.kind;
  return (
    <p className="wk-craft-notice" data-kind={card.resolved ? 'readonly' : 'unknown'} aria-live="polite">
      交互 {card.id} · {label}{card.resolved ? ' · 已处理' : ''}
    </p>
  );
}
