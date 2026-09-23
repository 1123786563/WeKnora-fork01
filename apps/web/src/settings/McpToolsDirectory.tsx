import { useEffect, useMemo, useRef, useState } from 'react';
import * as React from 'react';
import { createPortal } from 'react-dom';
import { Button as TButton, Input as TInput } from 'tdesign-react';
import { WkStatus as Status } from '../shared/wk-legacy.tsx';

type PolicyField = 'enabled' | 'requireApproval';
type McpTool = { name: string; description?: string; inputSchema?: unknown };
type McpToolApproval = { toolName: string; enabled: boolean; requireApproval: boolean };
type Props = { tools: McpTool[]; serviceId?: string; approvals: McpToolApproval[]; busy: boolean; busyTools?: ReadonlySet<string>; policyError: string | null; onRetryPolicies: () => void; onPolicyChange: (name: string, field: PolicyField, value: boolean) => void };

/* S6 Tailwind 收编：tab 样式 → settings-wrapper.css .wk-mcp-tool-detail-tab。 */
const detailTabs = ['description', 'parameters', 'schema'] as const;

function parametersOf(schema: unknown): Array<{ name: string; type?: string; required: boolean; description?: string }> {
  if (!schema || typeof schema !== 'object') return [];
  const value = schema as { properties?: Record<string, { type?: string; description?: string }>; required?: unknown };
  const required = new Set(Array.isArray(value.required) ? value.required.filter((name): name is string => typeof name === 'string') : []);
  return Object.entries(value.properties ?? {}).map(([name, parameter]) => ({ name, type: parameter?.type, description: parameter?.description, required: required.has(name) }));
}

export function McpToolsDirectory({ tools, serviceId, approvals, busy, busyTools, policyError, onRetryPolicies, onPolicyChange }: Props) {
  const t = (key: string) => ({
    'mcpMetadata.searchTools': '搜索工具名称或描述',
    'mcpMetadata.retry': '重试',
    'mcpMetadata.details': '详情',
    'mcpMetadata.description': '描述',
    'mcpMetadata.parameters': '参数',
    'mcpMetadata.fullSchema': '完整 Schema',
    'mcpMetadata.noDescription': '暂无描述',
    'mcpMetadata.required': '必填',
    'mcpMetadata.noParameters': '无参数',
    'mcpMetadata.enabled': '启用工具',
    'mcpMetadata.approval': '调用需审批',
    'mcpMetadata.tools': 'MCP 工具',
    'mcpMetadata.previous': '上一步',
    'mcpMetadata.next': '下一页',
    'mcpMetadata.noTools': '暂无 MCP 工具',
  }[key] ?? key);
  const pageSize = 20;
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(1);
  const [openTool, setOpenTool] = useState<string | null>(null);
  const [tab, setTab] = useState<'description' | 'parameters' | 'schema'>('description');
  const detailRef = useRef<HTMLDivElement | null>(null);
  const triggerRefs = useRef(new Map<string, HTMLButtonElement>());
  const tabRefs = useRef(new Map<(typeof detailTabs)[number], HTMLButtonElement>());
  const [popupPosition, setPopupPosition] = useState<{ top: number; left: number } | null>(null);
  const closeToolDetail = () => {
    const trigger = openTool ? triggerRefs.current.get(openTool) : undefined;
    setOpenTool(null);
    if (trigger) {
      const schedule = typeof requestAnimationFrame === 'function' ? requestAnimationFrame : (callback: FrameRequestCallback) => window.setTimeout(callback, 0);
      schedule(() => trigger.focus());
    }
  };
  const filtered = useMemo(() => { const needle = query.trim().toLocaleLowerCase(); return needle ? tools.filter((tool) => `${tool.name} ${tool.description ?? ''}`.toLocaleLowerCase().includes(needle)) : tools; }, [tools, query]);
  const pageCount = Math.max(1, Math.ceil(filtered.length / pageSize));
  const visible = filtered.slice((page - 1) * pageSize, page * pageSize);
  useEffect(() => { setPage(1); setOpenTool(null); }, [query, tools]);
  useEffect(() => { if (page > pageCount) setPage(pageCount); }, [page, pageCount]);
  useEffect(() => { setOpenTool(null); }, [page]);
  useEffect(() => {
    if (!openTool) { setPopupPosition(null); return; }
    const updatePosition = () => {
      const trigger = triggerRefs.current.get(openTool);
      if (!trigger) return;
      const rect = trigger.getBoundingClientRect();
      const width = Math.min(400, window.innerWidth - 24);
      setPopupPosition({ top: rect.bottom + 4, left: Math.max(12, Math.min(rect.right - width, window.innerWidth - width - 12)) });
    };
    updatePosition();
    window.addEventListener('resize', updatePosition);
    window.addEventListener('scroll', updatePosition, true);
    return () => { window.removeEventListener('resize', updatePosition); window.removeEventListener('scroll', updatePosition, true); };
  }, [openTool]);
  useEffect(() => {
    if (!openTool) return;
    const activeTab = tabRefs.current.get(tab);
    activeTab?.focus();
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') { event.preventDefault(); closeToolDetail(); } };
    const onPointerDown = (event: PointerEvent) => { if (event.target && !detailRef.current?.contains(event.target as Node)) closeToolDetail(); };
    document.addEventListener('keydown', onKeyDown);
    document.addEventListener('pointerdown', onPointerDown);
    return () => { document.removeEventListener('keydown', onKeyDown); document.removeEventListener('pointerdown', onPointerDown); };
  }, [openTool]);
  const policy = (name: string) => approvals.find((row) => row.toolName === name) ?? { enabled: true, requireApproval: false };
  return <div className="wk-mcp-directory">
    {policyError ? <div className="wk-mcp-directory-error"><Status tone="error">{policyError}</Status><TButton type="button" onClick={onRetryPolicies}>{t('mcpMetadata.retry')}</TButton></div> : null}
    {tools.length > pageSize ? <label className="wk-mcp-directory-search"><span className="wk-visually-hidden">{t('mcpMetadata.searchTools')}</span><TInput aria-label={t('mcpMetadata.searchTools')} className="wk-mcp-directory-search-input" value={query} onChange={(value) => setQuery(String(value))} placeholder={t('mcpMetadata.searchTools')} /></label> : null}
    {visible.length ? <ul className="wk-mcp-directory-list">{visible.map((tool, index) => { const current = policy(tool.name); const isOpen = openTool === tool.name; const detailId = `mcp-tool-detail-${index}`; const panelId = `${detailId}-panel`; return <li key={tool.name} className="wk-mcp-directory-item">
      <div className="wk-mcp-directory-heading"><strong>{tool.name}</strong><button ref={(node) => { if (node) triggerRefs.current.set(tool.name, node); else triggerRefs.current.delete(tool.name); }} type="button" aria-expanded={isOpen} aria-controls={detailId} className={'wk-mcp-directory-details' + (isOpen ? ' is-open' : '')} onClick={() => { setOpenTool(isOpen ? null : tool.name); setTab('description'); }}>{t('mcpMetadata.details')}</button></div>
      {tool.description ? <p className="wk-mcp-directory-desc">{tool.description}</p> : null}
      {isOpen ? (() => { const detail = <div id={detailId} ref={detailRef} className="wk-mcp-tool-detail-popup" role="dialog" aria-modal="true" aria-label={`${tool.name} ${t('mcpMetadata.details')}`} style={popupPosition ? { top: popupPosition.top, left: popupPosition.left } : undefined}><div className="wk-mcp-tool-detail-card"><div role="tablist" aria-label={t('mcpMetadata.details')} className="wk-mcp-tool-detail-tabs">{detailTabs.map((item, itemIndex) => <button key={item} ref={(node) => { if (node) tabRefs.current.set(item, node); else tabRefs.current.delete(item); }} id={`${detailId}-tab-${item}`} type="button" role="tab" tabIndex={tab === item ? 0 : -1} aria-selected={tab === item} aria-controls={panelId} className={'wk-mcp-tool-detail-tab' + (tab === item ? ' is-active' : '')} onClick={() => setTab(item)} onKeyDown={(event) => { if (event.key === 'ArrowRight' || event.key === 'ArrowLeft' || event.key === 'Home' || event.key === 'End') { event.preventDefault(); const nextIndex = event.key === 'Home' ? 0 : event.key === 'End' ? detailTabs.length - 1 : (itemIndex + (event.key === 'ArrowRight' ? 1 : -1) + detailTabs.length) % detailTabs.length; const next = detailTabs[nextIndex]; setTab(next); tabRefs.current.get(next)?.focus(); } else if (event.key === 'Tab') { const tabs = Array.from(tabRefs.current.values()); const first = tabs[0]; const last = tabs[tabs.length - 1]; if ((event.shiftKey && document.activeElement === first) || (!event.shiftKey && document.activeElement === last)) { event.preventDefault(); (event.shiftKey ? last : first)?.focus(); } } }}>{item === 'description' ? t('mcpMetadata.description') : item === 'parameters' ? t('mcpMetadata.parameters') : t('mcpMetadata.fullSchema')}</button>)}</div><div id={panelId} role="tabpanel" aria-labelledby={`${detailId}-tab-${tab}`} className="wk-mcp-tool-detail-panel">{tab === 'description' ? <p className="wk-mcp-tool-detail-body">{tool.description || t('mcpMetadata.noDescription')}</p> : tab === 'parameters' ? <>{parametersOf(tool.inputSchema).length ? <ul className="wk-mcp-tool-detail-body">{parametersOf(tool.inputSchema).map((parameter) => <li key={parameter.name}><code>{parameter.name}</code> {parameter.type ?? 'unknown'} {parameter.required ? t('mcpMetadata.required') : ''}{parameter.description ? ` — ${parameter.description}` : ''}</li>)}</ul> : <p className="wk-mcp-tool-detail-body">{t('mcpMetadata.noParameters')}</p>}</> : tool.inputSchema ? <pre className="wk-mcp-tool-detail-body wk-mcp-tool-detail-schema">{JSON.stringify(tool.inputSchema, null, 2)}</pre> : <p className="wk-mcp-tool-detail-body">{t('mcpMetadata.noParameters')}</p>}</div></div></div>; return typeof document === 'undefined' || !document.body ? detail : createPortal(detail, document.body); })() : null}
      {serviceId ? <div className="wk-mcp-directory-policy"><label><span>{t('mcpMetadata.enabled')}</span><input type="checkbox" role="switch" aria-label={`${tool.name} ${t('mcpMetadata.enabled')}`} disabled={busy || busyTools?.has(tool.name) === true || Boolean(policyError)} checked={current.enabled} onChange={(event) => onPolicyChange(tool.name, 'enabled', event.target.checked)} /></label><label><span>{t('mcpMetadata.approval')}</span><input type="checkbox" role="switch" aria-label={`${tool.name} ${t('mcpMetadata.approval')}`} disabled={busy || busyTools?.has(tool.name) === true || Boolean(policyError)} checked={current.requireApproval} onChange={(event) => onPolicyChange(tool.name, 'requireApproval', event.target.checked)} /></label></div> : null}
    </li>; })}</ul> : <Status>{t('mcpMetadata.noTools')}</Status>}
    {filtered.length > pageSize ? <nav className="wk-mcp-directory-pager" aria-label={t('mcpMetadata.tools')}><TButton type="button" disabled={page === 1} onClick={() => setPage((value) => value - 1)}>{t('mcpMetadata.previous')}</TButton><span aria-live="polite" aria-atomic="true">{page} / {pageCount}</span><TButton type="button" disabled={page === pageCount} onClick={() => setPage((value) => value + 1)}>{t('mcpMetadata.next')}</TButton></nav> : null}
  </div>;
}
