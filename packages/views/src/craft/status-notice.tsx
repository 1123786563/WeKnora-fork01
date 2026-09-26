// CFT-S01-T012: the workbench status banner wrapper. Reconnect and expired
// tickets are CONNECTION/authorization facts — they must never paint the run
// itself as failed (the run status projection owns success/failure wording).
// Delegates the live region and kind styling to T004's CraftNotice.
import React, { type ReactNode } from 'react';
import { CraftNotice, type CraftNoticeKind } from './shell.tsx';

const STATUS_NOTICE_TEXT: Record<CraftNoticeKind, string> = {
  reconnect: '连接中断，正在重新同步…（任务仍在执行）',
  failed: '执行失败',
  canceled: '已停止',
  readonly: '当前会话为只读',
  expired: '预览票据已过期，可重新授权同一版本',
  unknown: '委派结果不明，等待核对',
};

/** The workbench banner for connection/authorization lifecycle states. */
export function CraftStatusNotice({ kind, children }: { kind: CraftNoticeKind; children?: ReactNode }) {
  return (
    <CraftNotice kind={kind}>
      {children ?? STATUS_NOTICE_TEXT[kind]}
    </CraftNotice>
  );
}

// T17 (#136): the stop-outcome notice vocabulary. The three durable stop
// outcomes of a Run (the frozen T00 CraftStopOutcome) each carry their own
// member-facing sentence — the accepted stop is NEVER announced as stopped,
// and an unobservable abort outcome is never announced as confirmed.
export type CraftStopOutcomeStatus = 'requested' | 'confirmed' | 'unknown';

const STOP_OUTCOME_NOTICE_TEXT: Record<CraftStopOutcomeStatus, string> = {
  requested: '已请求停止，正在等待执行器确认…',
  confirmed: '已停止',
  unknown: '停止结果不明，等待核对',
};

/** The workbench banner for one Run's durable stop outcome. */
export function CraftStopNotice(_props: { outcome: CraftStopOutcomeStatus }): ReactNode {
  return null; // T17 RED stub
}
