import { useState } from 'react';
import * as React from 'react';
import type { McpTestResult, McpTool } from '@weknora/api-client';

type PolicyField = 'enabled' | 'requireApproval';
type Props = { result: McpTestResult | null; approvals?: { toolName: string; enabled: boolean; requireApproval: boolean }[]; busy?: boolean; onPolicyChange?: (name: string, field: PolicyField, value: boolean) => void };

export function McpTestResultBody({ result, approvals = [], busy = false, onPolicyChange }: Props) {
  const [expanded, setExpanded] = useState<number | null>(null);
  if (!result) return null;
  const tools = result.tools ?? [];
  const resources = result.resources ?? [];
  return <div className={result.success ? 'wk-mcp-test-success' : 'wk-mcp-test-error'} data-testid="mcp-test-result">
    <strong>{result.success ? 'Connection succeeded' : 'Connection failed'}</strong>
    {result.message ? <p>{result.message}</p> : null}
    {result.success && result.description ? <div><span>Description</span><p>{result.description}</p></div> : null}
    {result.success && tools.length ? <section aria-label="MCP tools"><h5>Tools ({tools.length})</h5><ul className="wk-mcp-test-items">{tools.map((tool, index) => <li key={`${tool.name}-${index}`}>
      <button type="button" aria-expanded={expanded === index} onClick={() => setExpanded(expanded === index ? null : index)}>{tool.name} <span>{expanded === index ? '▴' : '▾'}</span></button>
      <div className="wk-mcp-test-policy"><label><input type="checkbox" disabled={busy || !onPolicyChange} checked={approvals.find((row) => row.toolName === tool.name)?.enabled ?? true} onChange={(event) => onPolicyChange?.(tool.name, 'enabled', event.target.checked)} /> Enabled</label><label><input type="checkbox" disabled={busy || !onPolicyChange} checked={approvals.find((row) => row.toolName === tool.name)?.requireApproval ?? false} onChange={(event) => onPolicyChange?.(tool.name, 'requireApproval', event.target.checked)} /> Approval</label></div>
      {tool.description ? <p>{tool.description}</p> : null}
      {expanded === index && tool.inputSchema ? <pre>{JSON.stringify(tool.inputSchema, null, 2)}</pre> : null}
    </li>)}</ul></section> : null}
    {result.success && resources.length ? <section aria-label="MCP resources"><h5>Resources ({resources.length})</h5><ul className="wk-mcp-test-items">{resources.map((resource, index) => <li key={`${resource.uri}-${index}`}>
      <strong>{resource.name || resource.uri}</strong>{resource.mimeType ? <span> {resource.mimeType}</span> : null}
      {resource.description ? <p>{resource.description}</p> : null}<code>{resource.uri}</code>
    </li>)}</ul></section> : null}
    {result.success && tools.length === 0 && resources.length === 0 ? <p>No tools or resources returned.</p> : null}
  </div>;
}
