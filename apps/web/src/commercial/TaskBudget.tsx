import { useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';

export interface TaskBudgetSnapshot {
  /** 批准上限（credits） */
  limit: number;
  /** 已用（credits） */
  used: number;
  /** 占用（credits） */
  held: number;
}

interface TaskBudgetProps {
  client: WeKnoraClient;
  taskId: string;
  /** Budget facts come from the task-run data source; when absent the readout shows no fabricated numbers. */
  snapshot?: TaskBudgetSnapshot;
}

/**
 * TaskBudget shows the task budget (批准上限 / 已用 / 占用 / 可追加原因)
 * and offers a SEPARATE extend action. Extending the budget is a budget
 * decision only: it is never combined with approval for an external write
 * action, and every consent control here is unchecked by default — nothing
 * is ever auto-checked (no auto send authorization anywhere).
 */
export function TaskBudget({ client, taskId, snapshot }: TaskBudgetProps) {
  const [limit, setLimit] = useState<number | undefined>(snapshot?.limit);
  const [used] = useState<number | undefined>(snapshot?.used);
  const [held] = useState<number | undefined>(snapshot?.held);
  const [credits, setCredits] = useState('100');
  const [reason, setReason] = useState('');
  const [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState('');
  const [error, setError] = useState<string | null>(null);
  // One idempotency key per logical extension: a retried request reuses the
  // same key so a flaky connection can never double the budget.
  const idempotencyRef = useRef<string | null>(null);
  const lastFocusRef = useRef<HTMLElement | null>(null);

  const extend = async () => {
    const amount = Number(credits);
    if (!Number.isSafeInteger(amount) || amount <= 0) { setError('追加额度必须为正整数'); return; }
    if (reason.trim() === '') { setError('请填写可追加原因'); return; }
    if (!confirmed) { setError('请先单独确认追加任务预算'); return; }
    if (idempotencyRef.current === null) {
      idempotencyRef.current = typeof crypto !== 'undefined' && 'randomUUID' in crypto
        ? crypto.randomUUID()
        : 'idem-' + Date.now() + '-' + Math.random().toString(36).slice(2);
    }
    lastFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setBusy(true);
    setStatus('正在追加任务预算…');
    try {
      const result = await client.apps.extendTaskBudget(taskId, {
        additional_credits: amount,
        idempotency_key: idempotencyRef.current,
      });
      if (limit !== undefined) setLimit(limit + result.additional_credits);
      setStatus('已追加 ' + result.additional_credits + ' 额度。追加预算不等于外部操作批准；外部写操作需单独批准。');
      setError(null);
      setConfirmed(false);
      setReason('');
      idempotencyRef.current = null;
    } catch (e) {
      setStatus('');
      setError(e instanceof Error ? e.message : '追加任务预算失败');
    } finally {
      setBusy(false);
      lastFocusRef.current?.focus();
    }
  };

  return (
    <Card>
      <h3>任务预算</h3>
      <p className="wk-muted">预算追加与外部写操作审批相互独立：追加预算不会、也不能授权任何外部发送或删除操作。</p>
      <div aria-live="polite" role="status">
        {status !== '' ? <Status>{status}</Status> : null}
        {error !== null ? <Status tone="error">{error}</Status> : null}
      </div>
      <ul className="wk-list">
        <li><strong>批准上限</strong><span>{limit === undefined ? '暂无数据' : limit + ' 额度'}</span></li>
        <li><strong>已用</strong><span>{used === undefined ? '暂无数据' : used + ' 额度'}</span></li>
        <li><strong>占用</strong><span>{held === undefined ? '暂无数据' : held + ' 额度'}</span></li>
      </ul>
      <div>
        <label htmlFor="task-budget-credits">追加额度（正整数）</label>
        <input
          id="task-budget-credits"
          aria-label="追加额度（正整数）"
          inputMode="numeric"
          value={credits}
          onChange={(event) => setCredits(event.target.value)}
        />
        <label htmlFor="task-budget-reason">可追加原因</label>
        <textarea
          id="task-budget-reason"
          aria-label="可追加原因"
          rows={2}
          value={reason}
          onChange={(event) => setReason(event.target.value)}
        />
        <label htmlFor="task-budget-confirm">
          <input
            id="task-budget-confirm"
            type="checkbox"
            aria-label="确认追加任务预算（不包含外部发送授权）"
            checked={confirmed}
            onChange={(event) => setConfirmed(event.target.checked)}
          />
          我确认追加此任务预算（仅预算决定，不包含任何外部发送授权）
        </label>
        <Button type="button" aria-label="追加任务预算" disabled={busy} onClick={() => void extend()}>追加预算</Button>
      </div>
    </Card>
  );
}
