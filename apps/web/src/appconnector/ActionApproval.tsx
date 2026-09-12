import { useEffect, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { ActionDetail } from '@weknora/contracts';
import { Button, Card, Status } from '@weknora/ui';
import { actionMessage } from './action-state.ts';

/** Connection context needed to re-prepare edited content as a NEW action. */
export interface ActionPrepareContext {
  connection_id: string;
  risk: 'read' | 'write' | 'send' | 'delete';
  app_version?: string;
}

interface ActionApprovalProps {
  client: WeKnoraClient;
  actionId: string;
  /**
   * Without a prepare context the component still renders, approves,
   * executes and queries — but content edits cannot mint a new digest
   * (that requires the connection/risk the original action was born with).
   */
  prepare?: ActionPrepareContext;
}

/**
 * ActionApproval renders the SERVER snapshot of one external write action
 * (fetched via getAction — reconnect recovery comes from the server, never
 * from component-pending state). Approve submits id + digest +
 * expectedVersion for the CURRENT snapshot only; editing content calls
 * prepareAction producing a NEW digest, so the old approval is invalid and
 * the action shows 等待操作批准 again. An unknown outcome offers ONLY a
 * query path — never "再发一次".
 */
export function ActionApproval({ client, actionId, prepare }: ActionApprovalProps) {
  const [currentId, setCurrentId] = useState(actionId);
  const [detail, setDetail] = useState<ActionDetail | null>(null);
  const [draftContent, setDraftContent] = useState('');
  const [status, setStatus] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const activeRef = useRef(true);
  const lastFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    activeRef.current = true;
    return () => { activeRef.current = false; };
  }, []);

  const load = async (id: string, note?: string) => {
    try {
      const next = await client.apps.getAction(id);
      if (!activeRef.current) return;
      setDetail(next);
      setDraftContent(next.action.content);
      setError(null);
      setStatus(note ?? actionMessage(next.action.state));
    } catch (e) {
      if (activeRef.current) {
        setStatus('');
        setError(e instanceof Error ? e.message : '操作状态获取失败');
      }
    }
  };

  useEffect(() => { void load(currentId); }, [currentId]); // eslint-disable-line react-hooks/exhaustive-deps

  const captureFocus = () => {
    lastFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  };
  const restoreFocus = () => { lastFocusRef.current?.focus(); };

  const approve = async () => {
    if (detail === null || busy) return;
    captureFocus();
    setBusy(true);
    setStatus('正在提交批准…');
    try {
      const next = await client.apps.approveAction(detail.action.id, {
        digest: detail.action.digest,
        expected_version: detail.expected_version,
      });
      if (activeRef.current) {
        setDetail(next);
        setDraftContent(next.action.content);
        setStatus('已批准当前内容：' + actionMessage(next.action.state));
      }
    } catch (e) {
      if (activeRef.current) { setStatus(''); setError(e instanceof Error ? e.message : '批准失败'); }
    } finally {
      setBusy(false);
      restoreFocus();
    }
  };

  const execute = async () => {
    if (detail === null || busy) return;
    captureFocus();
    setBusy(true);
    setStatus('正在执行已批准操作…');
    try {
      const next = await client.apps.executeAction(detail.action.id);
      if (activeRef.current) {
        setDetail(next);
        setDraftContent(next.action.content);
        setStatus('执行已提交：' + actionMessage(next.action.state));
      }
    } catch (e) {
      if (activeRef.current) { setStatus(''); setError(e instanceof Error ? e.message : '执行失败'); }
    } finally {
      setBusy(false);
      restoreFocus();
    }
  };

  const reprepare = async () => {
    if (detail === null || prepare === undefined || busy) return;
    try { JSON.parse(draftContent); } catch { setError('内容必须是合法 JSON'); return; }
    captureFocus();
    setBusy(true);
    setStatus('正在生成新的待批准内容…');
    try {
      const next = await client.apps.prepareAction({
        connection_id: prepare.connection_id,
        target: detail.action.target,
        risk: prepare.risk,
        content: draftContent,
        app_version: prepare.app_version,
      });
      if (activeRef.current) {
        setCurrentId(next.action.id);
        setDetail(next);
        setDraftContent(next.action.content);
        setError(null);
        setStatus('内容已变更，原批准失效：' + actionMessage(next.action.state));
      }
    } catch (e) {
      if (activeRef.current) { setStatus(''); setError(e instanceof Error ? e.message : '重新准备失败'); }
    } finally {
      setBusy(false);
      restoreFocus();
    }
  };

  const state = detail?.action.state;

  return (
    <Card>
      <h3>外部写操作审批</h3>
      <p className="wk-muted">展示服务端快照的完整目标与内容；批准仅对当前内容摘要有效，与任务预算追加互相独立。</p>
      <div aria-live="polite" role="status">
        {status !== '' ? <Status>{status}</Status> : null}
        {error !== null ? <Status tone="error">{error}</Status> : null}
      </div>
      {detail === null ? (
        error === null ? <Status>正在加载操作状态…</Status> : null
      ) : (
        <>
          <ul className="wk-list">
            <li><strong>状态</strong><span>{actionMessage(detail.action.state)}</span></li>
            <li><strong>目标</strong><span>{detail.action.target}</span></li>
            <li><strong>内容</strong><span><code>{detail.action.content}</code></span></li>
            <li><strong>连接</strong><span>{detail.action.connection_name}</span></li>
            <li><strong>内容摘要</strong><span title={detail.action.digest}>{detail.action.digest.slice(0, 16)}…</span></li>
          </ul>
          {state === 'unknown' ? (
            <>
              <Status tone="error">外部执行结果未确认。此处不提供“再发一次”，重复发送可能造成重复写操作。</Status>
              <Button type="button" aria-label="查询外部执行结果" onClick={() => void load(currentId, '已重新查询：' + actionMessage('unknown'))}>查询结果</Button>
              <p className="wk-muted">若查询后仍未确认，请联系管理员与提供方核对后再处理。</p>
            </>
          ) : (
            <>
              {state === 'awaiting_approval' || state === 'authorized' ? (
                <Button type="button" aria-label="批准当前内容摘要" disabled={busy} onClick={() => void approve()}>批准此内容</Button>
              ) : null}
              {state === 'authorized' ? (
                <Button type="button" aria-label="执行已批准的操作" disabled={busy} onClick={() => void execute()}>执行</Button>
              ) : null}
              {prepare !== undefined && state === 'awaiting_approval' ? (
                <div>
                  <label htmlFor="action-content-edit">修改内容（生成新摘要，原批准失效）</label>
                  <textarea
                    id="action-content-edit"
                    aria-label="编辑操作内容 JSON"
                    value={draftContent}
                    rows={4}
                    onChange={(event) => setDraftContent(event.target.value)}
                  />
                  <Button type="button" aria-label="以编辑后的内容重新准备" disabled={busy} onClick={() => void reprepare()}>重新准备内容</Button>
                </div>
              ) : null}
              <Button type="button" aria-label="刷新操作状态" onClick={() => void load(currentId)}>刷新状态</Button>
            </>
          )}
        </>
      )}
    </Card>
  );
}
