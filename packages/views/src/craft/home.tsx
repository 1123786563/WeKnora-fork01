// W05 craft Home: the creation entry and the recent-works list.
//
// Pure view component: every piece of data and every action arrives through
// props — no global storage reads, no network. The assembly (apps/web
// routes.tsx) owns sessions, uploads and navigation.
import { useState } from 'react';
import type { CraftSessionKind, CraftSessionSummaryView } from '@weknora/contracts';
import { CRAFT_SESSION_KINDS } from '@weknora/contracts';
import { Button } from '@weknora/ui';
import { craftStrings, formatDateTime, type CraftLocale } from './presentation.ts';
import './craft.css';

export interface CraftAttachmentDraft {
  id: string;
  name: string;
  state: 'pending' | 'uploading' | 'ready' | 'error';
}

export interface CraftKnowledgeOption {
  id: string;
  name: string;
}

export interface CraftHomeCreateInput {
  title: string;
  kind: CraftSessionKind;
  knowledgeScope: string;
}

export interface CraftHomeProps {
  locale: CraftLocale;
  canCreate: boolean;
  createBusy: boolean;
  createError: string | null;
  listStatus: 'loading' | 'error' | 'ready';
  listError: string | null;
  recent: CraftSessionSummaryView[];
  nextCursor: string | null;
  knowledgeOptions: CraftKnowledgeOption[];
  attachments: CraftAttachmentDraft[];
  onCreate(input: CraftHomeCreateInput): void;
  onPickAttachments(files: File[]): void;
  onRemoveAttachment(id: string): void;
  onOpen(sessionId: string): void;
  onNextPage(cursor: string): void;
  onRetryList(): void;
}

export function CraftHome(props: CraftHomeProps) {
  const strings = craftStrings(props.locale);
  const [goal, setGoal] = useState('');
  const [kind, setKind] = useState<CraftSessionKind>('web');
  const [knowledgeScope, setKnowledgeScope] = useState('');

  const goalTitle = goal.trim();

  return (
    <main className="wk-craft wk-craft-page" aria-label={strings.craftHomeTitle}>
      <header className="wk-craft-head">
        <div>
          <h1>{strings.craftHomeTitle}</h1>
          <p className="wk-craft-muted">{strings.craftHomeSubtitle}</p>
        </div>
      </header>

      <section className="wk-craft-create" aria-label={strings.craftNewGoalLabel}>
        <div className="wk-craft-field">
          <label htmlFor="craft-goal-input">{strings.craftNewGoalLabel}</label>
          <textarea
            id="craft-goal-input"
            data-testid="craft-goal"
            className="wk-craft-textarea"
            placeholder={strings.craftNewGoalPlaceholder}
            value={goal}
            onChange={(event) => setGoal(event.target.value)}
          />
        </div>
        <div className="wk-craft-row">
          <div className="wk-craft-field">
            <label htmlFor="craft-kind-select">{strings.craftKindLabel}</label>
            <select
              id="craft-kind-select"
              className="wk-craft-select"
              value={kind}
              onChange={(event) => setKind(event.target.value as CraftSessionKind)}
            >
              {CRAFT_SESSION_KINDS.map((option) => (
                <option key={option} value={option}>{option}</option>
              ))}
            </select>
          </div>
          <div className="wk-craft-field">
            <label htmlFor="craft-knowledge-select">{strings.craftKnowledgeScopeLabel}</label>
            <select
              id="craft-knowledge-select"
              className="wk-craft-select"
              value={knowledgeScope}
              onChange={(event) => setKnowledgeScope(event.target.value)}
            >
              <option value="">{strings.craftKnowledgeNone}</option>
              {props.knowledgeOptions.map((option) => (
                <option key={option.id} value={option.id}>{option.name}</option>
              ))}
            </select>
          </div>
        </div>
        <div className="wk-craft-field">
          <label htmlFor="craft-upload-input">{strings.craftAttachmentsLabel}</label>
          <input
            id="craft-upload-input"
            data-testid="craft-upload"
            className="wk-craft-input"
            type="file"
            multiple
            onChange={(event) => {
              props.onPickAttachments(Array.from(event.target.files ?? []));
              event.target.value = '';
            }}
          />
          <p className="wk-craft-hint">{strings.craftUploadHint}</p>
          {props.attachments.length > 0 ? (
            <ul className="wk-craft-chips">
              {props.attachments.map((attachment) => (
                <li key={attachment.id} className="wk-craft-chip" data-state={attachment.state}>
                  <span>{attachment.name}</span>
                  <span aria-hidden="true">
                    {attachment.state === 'uploading'
                      ? strings.craftAttachmentUploading
                      : attachment.state === 'error'
                        ? strings.craftAttachmentFailed
                        : attachment.state === 'ready'
                          ? strings.craftAttachmentReady
                          : ''}
                  </span>
                  <button type="button" onClick={() => props.onRemoveAttachment(attachment.id)} aria-label={strings.craftRetry + ': ' + attachment.name}>×</button>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
        {props.createError !== null ? <p className="wk-craft-error" role="alert">{props.createError}</p> : null}
        {props.canCreate ? (
          <div className="wk-craft-actions">
            <Button
              type="button"
              data-testid="craft-create"
              disabled={props.createBusy || goalTitle === ''}
              onClick={() => props.onCreate({ title: goalTitle, kind, knowledgeScope })}
            >
              {props.createBusy ? strings.craftCreateBusy : strings.craftCreate}
            </Button>
          </div>
        ) : (
          <p className="wk-craft-error" role="alert">{strings.craftLoginRequired}</p>
        )}
      </section>

      <section aria-label={strings.craftRecentTitle}>
        <h2>{strings.craftRecentTitle}</h2>
        {props.listStatus === 'loading' ? <p className="wk-craft-muted">{strings.craftRecentLoading}</p> : null}
        {props.listStatus === 'error' ? (
          <div role="alert" className="wk-craft-error">
            <p>{strings.craftErrorTitle}: {props.listError ?? ''}</p>
            <Button type="button" onClick={props.onRetryList}>{strings.craftRetry}</Button>
          </div>
        ) : null}
        {props.listStatus === 'ready' && props.recent.length === 0 ? (
          <p className="wk-craft-muted">{strings.craftRecentEmpty}</p>
        ) : null}
        {props.listStatus === 'ready' && props.recent.length > 0 ? (
          <ul className="wk-craft-list">
            {props.recent.map((item) => (
              <li key={item.session_id}>
                <span className="wk-craft-item-title">{item.title === '' ? item.session_id : item.title}</span>
                <span className="wk-craft-kind">{item.kind}</span>
                <span className="wk-craft-item-meta">{formatDateTime(item.updated_at, props.locale)}</span>
                <Button type="button" onClick={() => props.onOpen(item.session_id)}>{strings.craftOpen}</Button>
              </li>
            ))}
          </ul>
        ) : null}
        {props.nextCursor !== null && props.listStatus !== 'loading' ? (
          <p>
            <Button type="button" onClick={() => props.onNextPage(props.nextCursor ?? '')}>{strings.craftNextPage}</Button>
          </p>
        ) : null}
      </section>
    </main>
  );
}
