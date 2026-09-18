// C02 craft interaction panel: the complete pending-question and permission
// decision surface, standalone by design.
//
// This module is deliberately an INDEPENDENT component with an exported
// mount-point API (createSessionCraftInteractionClient +
// mountCraftInteractionPanel): the workbench wiring belongs to the integrator
// (workbench.tsx is a parallel C05 ownership file), so mounting is a one-line
// adoption at integration time. The panel talks to the real C02 HTTP surface:
//
//   GET  /api/v1/sessions/:sid/craft/interactions
//   POST /api/v1/sessions/:sid/craft/interactions/:id/decide   (202 accepted;
//        delivery_unknown keeps an honest "recorded but unconfirmed" notice)
//
// Fail-closed rules mirrored from the server: a question is answered or
// rejected (never approved), a permission is approve-once or reject (no
// session grant in the first phase), single-choice accepts at most one
// selection, custom text is bounded by 8 KiB, and a decided interaction only
// ever re-displays its recorded state — it can never be re-decided.
import React from 'react';
import { useCallback, useEffect, useRef, useState } from 'react';

// ---------------------------------------------------------------------------
// Wire types (snake_case, server-derived identity only)
// ---------------------------------------------------------------------------

export type CraftInteractionKindView = 'question' | 'permission' | 'unknown';
export type CraftInteractionActionView = 'answer' | 'approve' | 'reject';

export interface CraftPermissionItemView {
  tool?: string;
  command?: string;
  path?: string;
  scope?: string;
}

export interface CraftPendingPayloadView {
  options?: Record<string, string[]>;
  multiple?: Record<string, boolean>;
  permissions?: CraftPermissionItemView[];
}

export interface CraftRecordedAnswerView {
  question_id: string;
  choices?: string[];
  text?: string;
}

export interface CraftPendingDecisionView {
  id: string;
  kind: CraftInteractionKindView;
  prompt: string;
  status: string;
  delivery: string;
  revision: number;
  args_hash: string;
  pending_id: string;
  decision_id?: string;
  decided_action?: string;
  pending?: CraftPendingPayloadView;
  answers?: CraftRecordedAnswerView[];
}

export interface CraftDecideOutcomeView {
  decided: boolean;
  delivered: boolean;
  delivery: string;
  note?: string;
}

export interface CraftDecideInput {
  interactionId: string;
  decisionId: string;
  action: CraftInteractionActionView;
  argsHash: string;
  expectedRevision: number;
  answers: { question_id: string; choices: string[]; text: string }[];
}

// ---------------------------------------------------------------------------
// Client (fetch-injectable so any auth layer can wrap it)
// ---------------------------------------------------------------------------

export interface CraftInteractionClient {
  list(signal?: AbortSignal): Promise<CraftPendingDecisionView[]>;
  decide(input: CraftDecideInput, signal?: AbortSignal): Promise<CraftDecideOutcomeView>;
}

async function errorText(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as { error?: { message?: string }; message?: string };
    return body.error?.message ?? body.message ?? response.statusText;
  } catch {
    return response.statusText;
  }
}

/** 409: the payload or revision raced another writer (another tab decided first). */
export class CraftInteractionConflictError extends Error {}
/** 410: the interaction is canceled or terminal — it only re-displays now. */
export class CraftInteractionGoneError extends Error {}

function mapStatusError(status: number, text: string): Error {
  if (status === 409) return new CraftInteractionConflictError(text);
  if (status === 410) return new CraftInteractionGoneError(text);
  return new Error('craft interaction HTTP ' + status + ': ' + text);
}

/** Builds the C02 interaction client bound to one session over an injectable fetch. */
export function createSessionCraftInteractionClient(
  fetchImpl: typeof fetch,
  sessionId: string,
  baseUrl = '',
): CraftInteractionClient {
  const base = baseUrl.replace(/\/$/, '');
  return {
    list: async (signal) => {
      const response = await fetchImpl(base + '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/interactions', { signal });
      if (!response.ok) throw mapStatusError(response.status, await errorText(response));
      const body = (await response.json()) as { data?: CraftPendingDecisionView[] };
      return body.data ?? [];
    },
    decide: async (input, signal) => {
      const response = await fetchImpl(
        base + '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/interactions/' + encodeURIComponent(input.interactionId) + '/decide',
        {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          signal,
          body: JSON.stringify({
            decision_id: input.decisionId,
            action: input.action,
            args_hash: input.argsHash,
            expected_revision: input.expectedRevision,
            answers: input.answers.map((answer) => ({
              question_id: answer.question_id, choices: answer.choices, text: answer.text,
            })),
          }),
        },
      );
      if (!response.ok) throw mapStatusError(response.status, await errorText(response));
      const body = (await response.json()) as { data?: CraftDecideOutcomeView };
      return body.data ?? { decided: false, delivered: false, delivery: 'unknown' };
    },
  };
}

// ---------------------------------------------------------------------------
// Strings
// ---------------------------------------------------------------------------

export interface CraftInteractionStrings {
  title: string;
  empty: string;
  questionTitle: string;
  permissionTitle: string;
  unknownTitle: string;
  submitAnswer: string;
  approveOnce: string;
  reject: string;
  processed: string;
  deliveryUnknown: string;
  conflict: string;
  loadFailed: string;
  customAnswerPlaceholder: string;
  scopeLabel: string;
  commandLabel: string;
  pathLabel: string;
  toolLabel: string;
  recordedAnswers: string;
  reload: string;
}

const zhStrings: CraftInteractionStrings = {
  title: '需要你的处理',
  empty: '暂无待处理事项',
  questionTitle: 'Agent 提问',
  permissionTitle: '权限申请',
  unknownTitle: '等待处理',
  submitAnswer: '提交回答',
  approveOnce: '本次同意',
  reject: '拒绝',
  processed: '已处理',
  deliveryUnknown: '已记录决定，但送达未确认',
  conflict: '该事项已被处理，正在刷新状态',
  loadFailed: '加载待处理事项失败',
  customAnswerPlaceholder: '自定义回答（可选）…',
  scopeLabel: '作用域',
  commandLabel: '命令',
  pathLabel: '路径',
  toolLabel: '工具',
  recordedAnswers: '已提交的回答',
  reload: '刷新',
};

const enStrings: CraftInteractionStrings = {
  title: 'Needs your input',
  empty: 'Nothing pending',
  questionTitle: 'Agent question',
  permissionTitle: 'Permission request',
  unknownTitle: 'Waiting',
  submitAnswer: 'Submit answer',
  approveOnce: 'Approve once',
  reject: 'Reject',
  processed: 'Processed',
  deliveryUnknown: 'Decision recorded, delivery unconfirmed',
  conflict: 'Already handled elsewhere — refreshing',
  loadFailed: 'Failed to load pending decisions',
  customAnswerPlaceholder: 'Custom answer (optional)…',
  scopeLabel: 'Scope',
  commandLabel: 'Command',
  pathLabel: 'Path',
  toolLabel: 'Tool',
  recordedAnswers: 'Submitted answers',
  reload: 'Refresh',
};

export function craftInteractionStrings(locale: 'zh' | 'en'): CraftInteractionStrings {
  return locale === 'zh' ? zhStrings : enStrings;
}

// ---------------------------------------------------------------------------
// Panel
// ---------------------------------------------------------------------------

export interface CraftInteractionPanelProps {
  locale: 'zh' | 'en';
  sessionId: string;
  client: CraftInteractionClient;
  /** False hides every deciding control (viewer/read-only mount). */
  canDecide: boolean;
  /** Poll interval for pending items; <= 0 disables polling. */
  pollMs?: number;
  onDecided?(view: CraftPendingDecisionView, outcome: CraftDecideOutcomeView): void;
}

interface DraftState {
  choices: Record<string, string[]>;
  text: Record<string, string>;
  busy: boolean;
  error: string | null;
  notice: string | null;
}

const emptyDraft: DraftState = { choices: {}, text: {}, busy: false, error: null, notice: null };

export function CraftInteractionPanel(props: CraftInteractionPanelProps) {
  const strings = craftInteractionStrings(props.locale);
  const [items, setItems] = useState<CraftPendingDecisionView[]>([]);
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading');
  const [error, setError] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<Record<string, DraftState>>({});
  const [busyAny, setBusyAny] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const views = await props.client.list();
      setItems(views);
      setStatus('ready');
      setError(null);
    } catch (loadError) {
      setStatus('error');
      setError(loadError instanceof Error ? loadError.message : String(loadError));
    }
  }, [props.client]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const pollMs = props.pollMs ?? 5000;
  useEffect(() => {
    if (pollMs <= 0) return;
    const timer = window.setInterval(() => {
      if (!busyAny) void refresh();
    }, pollMs);
    return () => window.clearInterval(timer);
  }, [pollMs, refresh, busyAny]);

  const decide = useCallback(async (
    view: CraftPendingDecisionView,
    action: CraftInteractionActionView,
    answers: { question_id: string; choices: string[]; text: string }[],
  ) => {
    const decisionId = 'dec_' + view.id + '_' + Date.now().toString(36);
    setBusyAny(true);
    setDrafts((prev) => ({ ...prev, [view.id]: { ...(prev[view.id] ?? emptyDraft), busy: true, error: null, notice: null } }));
    try {
      const outcome = await props.client.decide({
        interactionId: view.id,
        decisionId,
        action,
        argsHash: view.args_hash,
        expectedRevision: view.revision,
        answers,
      });
      setDrafts((prev) => ({ ...prev, [view.id]: { ...(prev[view.id] ?? emptyDraft), busy: false } }));
      props.onDecided?.(view, outcome);
      await refresh();
    } catch (decideError) {
      if (decideError instanceof CraftInteractionConflictError || decideError instanceof CraftInteractionGoneError) {
        setDrafts((prev) => ({ ...prev, [view.id]: { ...(prev[view.id] ?? emptyDraft), busy: false, notice: strings.conflict } }));
        await refresh();
        return;
      }
      setDrafts((prev) => ({ ...prev, [view.id]: { ...(prev[view.id] ?? emptyDraft), busy: false, error: decideError instanceof Error ? decideError.message : String(decideError) } }));
    } finally {
      setBusyAny(false);
    }
  }, [props, refresh, strings.conflict]);

  const pending = items.filter((item) => item.status === 'pending');
  const decided = items.filter((item) => item.status !== 'pending');

  return (
    <section className="wk-craft-interaction-panel" data-testid="craft-interaction-panel" data-state={status}>
      <header style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
        <h3 style={{ margin: 0 }}>{strings.title}</h3>
        <button type="button" className="wk-button" onClick={() => void refresh()}>{strings.reload}</button>
      </header>
      {status === 'error' ? <p role="alert">{strings.loadFailed}{error === null ? '' : '：' + error}</p> : null}
      {status === 'ready' && items.length === 0 ? <p className="wk-craft-muted">{strings.empty}</p> : null}
      {pending.map((view) => (
        <PendingCard
          key={view.id}
          view={view}
          strings={strings}
          canDecide={props.canDecide}
          draft={drafts[view.id] ?? emptyDraft}
          onDraft={(next) => setDrafts((prev) => ({ ...prev, [view.id]: next }))}
          onDecide={decide}
        />
      ))}
      {decided.map((view) => (
        <DecidedCard key={view.id} view={view} strings={strings} />
      ))}
    </section>
  );
}

function PendingCard(props: {
  view: CraftPendingDecisionView;
  strings: CraftInteractionStrings;
  canDecide: boolean;
  draft: DraftState;
  onDraft(next: DraftState): void;
  onDecide(view: CraftPendingDecisionView, action: CraftInteractionActionView, answers: { question_id: string; choices: string[]; text: string }[]): void;
}) {
  const { view, strings, canDecide, draft, onDraft, onDecide } = props;
  const questionIds = Object.keys(view.pending?.options ?? {});
  const isQuestion = view.kind === 'question';
  const isPermission = view.kind === 'permission';

  const submitAnswer = () => {
    const answers = questionIds.map((questionId) => ({
      question_id: questionId,
      choices: draft.choices[questionId] ?? [],
      text: (draft.text[questionId] ?? '').slice(0, 8192),
    }));
    onDecide(view, 'answer', answers);
  };

  return (
    <article className="wk-craft-interaction" data-interaction-id={view.id} data-resolved={false} data-kind={view.kind}>
      <h4 style={{ margin: '0.2rem 0' }}>{isQuestion ? strings.questionTitle : isPermission ? strings.permissionTitle : strings.unknownTitle}</h4>
      <p style={{ margin: '0.2rem 0' }}>{view.prompt}</p>

      {isPermission ? (
        <ul style={{ margin: '0.2rem 0', paddingLeft: '1rem' }} data-testid="craft-permission-items">
          {(view.pending?.permissions ?? []).map((item, index) => (
            <li key={index}>
              {item.command ? <div><strong>{strings.commandLabel}:</strong> <code>{item.command}</code></div> : null}
              {item.path ? <div><strong>{strings.pathLabel}:</strong> <code>{item.path}</code></div> : null}
              {item.scope ? <div><strong>{strings.scopeLabel}:</strong> <span>{item.scope}</span></div> : null}
              {!item.command && !item.path && !item.scope && item.tool ? <div><strong>{strings.toolLabel}:</strong> <span>{item.tool}</span></div> : null}
            </li>
          ))}
        </ul>
      ) : null}

      {isQuestion
        ? questionIds.map((questionId) => {
            const options = view.pending?.options?.[questionId] ?? [];
            const multiple = view.pending?.multiple?.[questionId] ?? false;
            const selected = draft.choices[questionId] ?? [];
            return (
              <div key={questionId} style={{ margin: '0.4rem 0' }} data-question-id={questionId}>
                {options.map((option) => (
                  <label key={option} style={{ display: 'block', margin: '0.1rem 0' }}>
                    <input
                      type={multiple ? 'checkbox' : 'radio'}
                      name={view.id + ':' + questionId}
                      value={option}
                      checked={selected.includes(option)}
                      disabled={!canDecide || draft.busy}
                      onChange={(event) => {
                        if (multiple) {
                          const next = event.target.checked ? [...selected, option] : selected.filter((choice) => choice !== option);
                          onDraft({ ...draft, choices: { ...draft.choices, [questionId]: next } });
                        } else {
                          onDraft({ ...draft, choices: { ...draft.choices, [questionId]: event.target.checked ? [option] : [] } });
                        }
                      }}
                    />{' '}{option}
                  </label>
                ))}
                <input
                  type="text"
                  value={draft.text[questionId] ?? ''}
                  disabled={!canDecide || draft.busy}
                  maxLength={8192}
                  placeholder={strings.customAnswerPlaceholder}
                  aria-label={strings.customAnswerPlaceholder}
                  onChange={(event) => onDraft({ ...draft, text: { ...draft.text, [questionId]: event.target.value } })}
                />
              </div>
            );
          })
        : null}

      {canDecide ? (
        <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.5rem' }}>
          {isQuestion ? (
            <button type="button" className="wk-button" disabled={draft.busy} onClick={submitAnswer}>{strings.submitAnswer}</button>
          ) : null}
          {isPermission ? (
            <button type="button" className="wk-button" disabled={draft.busy} onClick={() => onDecide(view, 'approve', [])}>{strings.approveOnce}</button>
          ) : null}
          <button type="button" className="wk-button" disabled={draft.busy} onClick={() => onDecide(view, 'reject', [])}>{strings.reject}</button>
        </div>
      ) : null}
      {draft.error === null ? null : <p role="alert" style={{ margin: '0.3rem 0' }}>{draft.error}</p>}
      {draft.notice === null ? null : <p role="status" style={{ margin: '0.3rem 0' }}>{draft.notice}</p>}
    </article>
  );
}

function DecidedCard(props: { view: CraftPendingDecisionView; strings: CraftInteractionStrings }) {
  const { view, strings } = props;
  const unknown = view.delivery === 'unknown';
  return (
    <article className="wk-craft-interaction" data-interaction-id={view.id} data-resolved data-kind={view.kind} data-delivery={view.delivery}>
      <h4 style={{ margin: '0.2rem 0' }}>{strings.processed}</h4>
      <p style={{ margin: '0.2rem 0' }}>{view.prompt}</p>
      {(view.answers ?? []).length > 0 ? (
        <div>
          <p className="wk-craft-hint" style={{ margin: '0.2rem 0' }}>{strings.recordedAnswers}</p>
          <ul style={{ margin: '0.2rem 0', paddingLeft: '1rem' }} data-testid="craft-recorded-answers">
            {(view.answers ?? []).map((answer, index) => (
              <li key={index}>
                <code>{answer.question_id}</code>
                {answer.choices && answer.choices.length > 0 ? ': ' + answer.choices.join(', ') : ''}
                {answer.text ? ' — ' + answer.text : ''}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {unknown ? <p role="status" data-testid="craft-delivery-unknown">{strings.deliveryUnknown}</p> : null}
    </article>
  );
}

// ---------------------------------------------------------------------------
// Mount-point API (integrator adoption)
// ---------------------------------------------------------------------------
// The workbench wiring belongs to the integrator (workbench.tsx is a parallel
// ownership file). Adoption is one block:

//   const interactions = useMemo(
//     () => createSessionCraftInteractionClient(authedFetch, sessionId, apiBaseUrl),
//     [authedFetch, sessionId, apiBaseUrl],
//   );
//   <CraftInteractionPanel locale={locale} sessionId={sessionId}
//     client={interactions} canDecide={canWrite} />

// CraftInteractionMountProps is the documented adoption surface; the alias
// exists so integrators can import a named mount point that stays stable
// even if the panel gains layout options later.
export type CraftInteractionMountProps = CraftInteractionPanelProps;

/** Stable mount-point alias for the panel (see the adoption snippet above). */
export function CraftInteractionMount(props: CraftInteractionMountProps) {
  return <CraftInteractionPanel {...props} />;
}
