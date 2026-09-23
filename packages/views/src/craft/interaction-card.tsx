// CFT-S01-T012 (P08): the decision card with an HONEST delivery state
// machine. Recording a decision is NOT execution: recorded →
// delivery_pending → delivered are distinct, observable layers, and unknown
// stays unknown (never repainted as success or failure to simplify). A
// question offers answer/reject only — answering never grants anything; a
// permission offers allow-ONCE, never a standing grant; an unknown kind is
// reject-only. A terminal card exposes no deciding controls at all — a late
// approval can never revive a canceled request.
import React from 'react';
import { Button } from './td.tsx';
import './craft.css';

export type CraftDecisionDelivery = 'recorded' | 'delivery_pending' | 'delivered' | 'unknown';

/** The delivery layers, each speaking for itself — none claims execution. */
export function decisionDeliveryLabel(delivery: string): string {
  switch (delivery) {
    case 'recorded':
      return '已记录（等待送达）';
    case 'delivery_pending':
      return '等待送达';
    case 'delivered':
      return '已确认送达';
    case 'unknown':
      return '送达不明，需核对';
    default:
      return '状态待核对';
  }
}

export type CraftDecisionAction = 'answer' | 'approve' | 'reject';

export interface CraftDecisionCardProps {
  id: string;
  kind: 'question' | 'permission' | 'unknown';
  prompt: string;
  scope: string;
  allowedActions: readonly CraftDecisionAction[];
  /** False while the request awaits a decision; true renders read-only. */
  resolved: boolean;
  decidedAction?: 'allow_once' | 'deny' | 'answer' | null;
  /** The honest delivery layer of the recorded decision. */
  delivery?: string;
  answer?: string;
  busy?: boolean;
  onAnswerChange?(text: string): void;
  onAction(action: CraftDecisionAction): void;
}

export function CraftDecisionCard(props: CraftDecisionCardProps) {
  const kindLabel = props.kind === 'question' ? '待回答' : props.kind === 'permission' ? '待批准' : '待核对';
  const decidedLabel =
    props.decidedAction === 'allow_once' ? '已批准（仅一次）'
    : props.decidedAction === 'deny' ? '已拒绝'
    : props.decidedAction === 'answer' ? '已提交回答'
    : null;

  return (
    <article
      className="wk-craft-interaction"
      data-testid="craft-decision-card"
      data-kind={props.kind}
      data-resolved={props.resolved}
      data-delivery={props.delivery ?? ''}
      aria-live="polite"
    >
      <h4>{props.id} · {kindLabel}</h4>
      <p>{props.prompt}</p>
      <p className="wk-craft-scope">{props.scope}</p>
      {props.resolved ? (
        <>
          {decidedLabel !== null ? <p role="status">{decidedLabel}</p> : null}
          {props.delivery !== undefined && props.delivery !== '' ? (
            <p role="status" className="wk-craft-hint" data-delivery-layer={props.delivery}>
              {decisionDeliveryLabel(props.delivery)}
            </p>
          ) : null}
        </>
      ) : (
        <div className="wk-craft-actions">
          {props.kind === 'question' ? (
            <>
              <textarea
                aria-label="回答"
                value={props.answer ?? ''}
                placeholder="输入回答…"
                onChange={(event) => props.onAnswerChange?.(event.target.value)}
              />
              <Button type="button" disabled={props.busy} onClick={() => props.onAction('answer')}>回答</Button>
            </>
          ) : null}
          {props.kind === 'permission' ? (
            <Button type="button" disabled={props.busy} onClick={() => props.onAction('approve')}>允许一次</Button>
          ) : null}
          <Button type="button" disabled={props.busy} onClick={() => props.onAction('reject')}>拒绝</Button>
        </div>
      )}
    </article>
  );
}
