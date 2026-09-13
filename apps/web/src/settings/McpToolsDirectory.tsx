import { useEffect, useMemo, useState } from 'react';
import * as React from 'react';
import type { McpTool, WeKnoraClient } from '@weknora/api-client';
import { Button, Status } from '@weknora/ui';
import { createTranslator, useAppLocale } from '../i18n.ts';

type PolicyField = 'enabled' | 'requireApproval';
type McpToolApproval = Awaited<ReturnType<WeKnoraClient['configuration']['mcp']['toolApprovals']['list']>>[number];
type Props = { tools: McpTool[]; serviceId: string; approvals: McpToolApproval[]; busy: boolean; policyError: string | null; onRetryPolicies: () => void; onPolicyChange: (name: string, field: PolicyField, value: boolean) => void };

function parametersOf(schema: unknown): Array<{ name: string; type?: string; required: boolean; description?: string }> {
  if (!schema || typeof schema !== 'object') return [];
  const value = schema as { properties?: Record<string, { type?: string; description?: string }>; required?: unknown };
  const required = new Set(Array.isArray(value.required) ? value.required.filter((name): name is string => typeof name === 'string') : []);
  return Object.entries(value.properties ?? {}).map(([name, parameter]) => ({ name, type: parameter?.type, description: parameter?.description, required: required.has(name) }));
}

export function McpToolsDirectory({ tools, serviceId, approvals, busy, policyError, onRetryPolicies, onPolicyChange }: Props) {
  const t = createTranslator(useAppLocale());
  const pageSize = 20;
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(1);
  const [openTool, setOpenTool] = useState<string | null>(null);
  const [tab, setTab] = useState<'description' | 'parameters' | 'schema'>('description');
  const filtered = useMemo(() => { const needle = query.trim().toLocaleLowerCase(); return needle ? tools.filter((tool) => `${tool.name} ${tool.description ?? ''}`.toLocaleLowerCase().includes(needle)) : tools; }, [tools, query]);
  const pageCount = Math.max(1, Math.ceil(filtered.length / pageSize));
  const visible = filtered.slice((page - 1) * pageSize, page * pageSize);
  useEffect(() => { setPage(1); setOpenTool(null); }, [query, tools]);
  useEffect(() => { if (page > pageCount) setPage(pageCount); }, [page, pageCount]);
  useEffect(() => {
    if (!openTool) return;
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') setOpenTool(null); };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [openTool]);
  const policy = (name: string) => approvals.find((row) => row.toolName === name) ?? { enabled: true, requireApproval: false };
  if (policyError) return <div className="wk-mcp-tools-directory"><Status tone="error">{policyError}</Status><Button type="button" onClick={onRetryPolicies}>{t('mcpMetadata.retry')}</Button></div>;
  return <div className="wk-mcp-tools-directory">
    {tools.length > pageSize ? <label className="wk-mcp-tool-search"><span className="wk-visually-hidden">{t('mcpMetadata.searchTools')}</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('mcpMetadata.searchTools')} /></label> : null}
    {visible.length ? <ul className="wk-mcp-directory-list">{visible.map((tool) => { const current = policy(tool.name); const isOpen = openTool === tool.name; return <li key={tool.name}>
      <div className="wk-mcp-directory-heading"><strong>{tool.name}</strong><button type="button" aria-expanded={isOpen} onClick={() => { setOpenTool(isOpen ? null : tool.name); setTab('description'); }}>{t('mcpMetadata.details')}</button></div>
      {tool.description ? <p>{tool.description}</p> : null}
      {isOpen ? <div className="wk-mcp-tool-detail-popup" role="dialog" aria-label={`${tool.name} ${t('mcpMetadata.details')}`}><div className="wk-mcp-tool-detail"><div role="tablist" className="wk-mcp-tool-tabs">{(['description', 'parameters', 'schema'] as const).map((item) => <button key={item} type="button" role="tab" aria-selected={tab === item} onClick={() => setTab(item)}>{item === 'description' ? t('mcpMetadata.description') : item === 'parameters' ? t('mcpMetadata.parameters') : t('mcpMetadata.fullSchema')}</button>)}</div>{tab === 'description' ? <p>{tool.description || t('mcpMetadata.noDescription')}</p> : tab === 'parameters' ? <>{parametersOf(tool.inputSchema).length ? <ul>{parametersOf(tool.inputSchema).map((parameter) => <li key={parameter.name}><code>{parameter.name}</code> {parameter.type ?? 'unknown'} {parameter.required ? t('mcpMetadata.required') : ''}{parameter.description ? ` — ${parameter.description}` : ''}</li>)}</ul> : <p>{t('mcpMetadata.noParameters')}</p>}</> : tool.inputSchema ? <pre>{JSON.stringify(tool.inputSchema, null, 2)}</pre> : <p>{t('mcpMetadata.noParameters')}</p>}</div></div> : null}
      <div className="wk-mcp-directory-controls"><label><input type="checkbox" disabled={busy} checked={current.enabled} onChange={(event) => onPolicyChange(tool.name, 'enabled', event.target.checked)} /> {t('mcpMetadata.enabled')}</label><label><input type="checkbox" disabled={busy} checked={current.requireApproval} onChange={(event) => onPolicyChange(tool.name, 'requireApproval', event.target.checked)} /> {t('mcpMetadata.approval')}</label></div>
    </li>; })}</ul> : <Status>{t('mcpMetadata.noTools')}</Status>}
    {filtered.length > pageSize ? <nav className="wk-mcp-directory-pagination" aria-label={t('mcpMetadata.tools')}><Button type="button" disabled={page === 1} onClick={() => setPage((value) => value - 1)}>{t('mcpMetadata.previous')}</Button><span>{page} / {pageCount}</span><Button type="button" disabled={page === pageCount} onClick={() => setPage((value) => value + 1)}>{t('mcpMetadata.next')}</Button></nav> : null}
  </div>;
}
