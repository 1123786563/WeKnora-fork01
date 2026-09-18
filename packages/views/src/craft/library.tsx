// CFT-S01-T009 (P02): the craft library page — the authorized sessions list
// with kind filter and search applied CLIENT-side over the server pages the
// home list already loads (same craftApi.list contract). The four states are
// distinct and labelled: loading, empty (no sessions at all), no-match (the
// filter matched nothing) and error — never conflated, never a blank frame.
import React, { useMemo, useState } from 'react';
import type { CraftSessionKind, CraftSessionSummaryView } from '@weknora/contracts';
import { CRAFT_SESSION_KINDS } from '@weknora/contracts';
import { Button } from '@weknora/ui';
import { craftStrings, formatDateTime, type CraftLocale } from './presentation.ts';
import './craft.css';

export type CraftLibraryStatus = 'loading' | 'ready' | 'error';

export interface CraftLibraryProps {
  locale: CraftLocale;
  sessions: readonly CraftSessionSummaryView[];
  status: CraftLibraryStatus;
  error?: string | null;
  hasNextPage: boolean;
  loadingMore: boolean;
  loadingMoreError?: string | null;
  onOpen(sessionId: string): void;
  onLoadMore(): void;
  onRetry(): void;
}

export function CraftLibrary(props: CraftLibraryProps) {
  const strings = craftStrings(props.locale);
  const [kindFilter, setKindFilter] = useState<CraftSessionKind | 'all'>('all');
  const [query, setQuery] = useState('');

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return props.sessions.filter((s) =>
      (kindFilter === 'all' || s.kind === kindFilter) && (q === '' || s.title.toLowerCase().includes(q)),
    );
  }, [props.sessions, kindFilter, query]);

  const isError = props.status === 'error';
  const isLoading = props.status === 'loading';
  const empty = props.status === 'ready' && props.sessions.length === 0;
  const noMatch = props.status === 'ready' && props.sessions.length > 0 && filtered.length === 0;

  return (
    <main className="wk-craft wk-craft-page" aria-label="我的作品">
      <header className="wk-craft-head">
        <div>
          <h1>我的作品</h1>
          <p className="wk-craft-muted">仅列出你有读取权限的作品会话</p>
        </div>
        <div className="wk-craft-actions">
          <input
            className="wk-craft-input"
            aria-label="搜索作品"
            placeholder="搜索标题…"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
          <select
            className="wk-craft-select"
            aria-label="按类型筛选"
            value={kindFilter}
            onChange={(event) => setKindFilter(event.target.value as CraftSessionKind | 'all')}
          >
            <option value="all">全部类型</option>
            {CRAFT_SESSION_KINDS.map((kind) => (
              <option key={kind} value={kind}>{kind}</option>
            ))}
          </select>
        </div>
      </header>

      {isError ? (
        <div role="alert">
          <p className="wk-craft-error">{props.error ?? '列表加载失败'}</p>
          <Button type="button" onClick={props.onRetry}>{strings.craftRetry}</Button>
        </div>
      ) : null}
      {isLoading ? <p className="wk-craft-muted">{strings.craftRecentLoading}</p> : null}
      {empty ? <p className="wk-craft-muted">还没有作品。回到创作首页开始第一次创作。</p> : null}
      {noMatch ? <p className="wk-craft-muted">没有匹配「{query || kindFilter}」的作品。调整筛选后再试。</p> : null}

      {filtered.length > 0 ? (
        <ul className="wk-craft-list" aria-label="作品列表">
          {filtered.map((item) => (
            <li key={item.session_id}>
              <span className="wk-craft-item-title">{item.title || item.session_id}</span>
              <span className="wk-craft-kind">{item.kind}</span>
              <span className="wk-craft-item-meta">{formatDateTime(item.updated_at, props.locale)}</span>
              <Button type="button" size="small" onClick={() => props.onOpen(item.session_id)}>打开</Button>
            </li>
          ))}
        </ul>
      ) : null}

      {props.hasNextPage && !isError ? (
        <div className="wk-craft-actions">
          <Button type="button" disabled={props.loadingMore} onClick={props.onLoadMore}>
            {props.loadingMore ? '加载中…' : '加载更多'}
          </Button>
          {props.loadingMoreError ? <p className="wk-craft-error" role="alert">{props.loadingMoreError} — 可重试本页</p> : null}
        </div>
      ) : null}
    </main>
  );
}
