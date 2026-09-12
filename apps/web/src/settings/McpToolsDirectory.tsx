import { useEffect, useMemo, useState } from 'react';
import * as React from 'react';
import type { McpTool, WeKnoraClient } from '@weknora/api-client';
import { Button, Status } from '@weknora/ui';

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
  const policy = (name: string) => approvals.find((row) => row.toolName === name) ?? { enabled: true, requireApproval: false };
  if (policyError) return <div className="wk-mcp-tools-directory"><Status tone="error">{policyError}</Status><Button type="button" onClick={onRetryPolicies}>Retry</Button></div>;
  return <div className="wk-mcp-tools-directory">
    {tools.length > pageSize ? <label className="wk-mcp-tool-search">Search tools<input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search tools" /></label> : null}
    {visible.length ? <ul className="wk-mcp-directory-list">{visible.map((tool) => { const current = policy(tool.name); const isOpen = openTool === tool.name; return <li key={tool.name}>
      <div className="wk-mcp-directory-heading"><strong>{tool.name}</strong><button type="button" aria-expanded={isOpen} onClick={() => { setOpenTool(isOpen ? null : tool.name); setTab('description'); }}>{isOpen ? 'Hide details' : 'Details'}</button></div>
      {tool.description ? <p>{tool.description}</p> : null}
      {isOpen ? <div className="wk-mcp-tool-detail"><div role="tablist" className="wk-mcp-tool-tabs">{(['description', 'parameters', 'schema'] as const).map((item) => <button key={item} type="button" role="tab" aria-selected={tab === item} onClick={() => setTab(item)}>{item === 'description' ? 'Description' : item === 'parameters' ? 'Parameters' : 'Full schema'}</button>)}</div>{tab === 'description' ? <p>{tool.description || 'No description.'}</p> : tab === 'parameters' ? <>{parametersOf(tool.inputSchema).length ? <ul>{parametersOf(tool.inputSchema).map((parameter) => <li key={parameter.name}><code>{parameter.name}</code> {parameter.type ?? 'unknown'} {parameter.required ? '(required)' : ''}{parameter.description ? ` — ${parameter.description}` : ''}</li>)}</ul> : <p>No parameters.</p>}</> : tool.inputSchema ? <pre>{JSON.stringify(tool.inputSchema, null, 2)}</pre> : <p>No parameters.</p>}</div> : null}
      <div className="wk-mcp-directory-controls"><label><input type="checkbox" disabled={busy} checked={current.enabled} onChange={(event) => onPolicyChange(tool.name, 'enabled', event.target.checked)} /> Enabled</label><label><input type="checkbox" disabled={busy} checked={current.requireApproval} onChange={(event) => onPolicyChange(tool.name, 'requireApproval', event.target.checked)} /> Approval</label></div>
    </li>; })}</ul> : <Status>No tools found.</Status>}
    {filtered.length > pageSize ? <nav className="wk-mcp-directory-pagination" aria-label="MCP tools pagination"><Button type="button" disabled={page === 1} onClick={() => setPage((value) => value - 1)}>Previous</Button><span>{page} / {pageCount}</span><Button type="button" disabled={page === pageCount} onClick={() => setPage((value) => value + 1)}>Next</Button></nav> : null}
  </div>;
}
