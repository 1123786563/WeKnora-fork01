import type { ApiKey, WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { useState } from 'react';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';

const capabilities = ['system_tenants_read', 'system_tenants_manage', 'system_settings_read', 'system_settings_manage', 'system_runtime_read', 'system_runtime_manage', 'system_audit_read'];
function date(value?: string) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '从未使用'; }
export function PlatformApiKeysPanel({ client, initialKeys }: { client: WeKnoraClient; initialKeys: ApiKey[] }) {
  const locale = useSettingsLocale();
  const t = settingsT(locale);
  const [keys, setKeys] = useState(initialKeys);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [selected, setSelected] = useState<string[]>([]);
  const [token, setToken] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  async function create() {
    if (!name.trim() || selected.length === 0) { setMessage('请填写名称并至少选择一个权限'); return; }
    setCreating(true); setMessage(null);
    try { const created = await client.administration.apiKeys.create({ name: name.trim(), capabilities: selected }); setKeys((current) => [...current, created]); setToken(created.token ?? created.api_key); setName(''); setSelected([]); }
    catch (reason) { setMessage(reason instanceof Error ? reason.message : '创建失败'); }
    finally { setCreating(false); }
  }
  async function revoke(key: ApiKey) {
    try { await client.administration.apiKeys.revoke(key.id); setKeys((current) => current.filter((item) => item.id !== key.id)); setMessage('已撤销'); }
    catch (reason) { setMessage(reason instanceof Error ? reason.message : '撤销失败'); }
  }
  const toggle = (value: string) => setSelected((current) => current.includes(value) ? current.filter((item) => item !== value) : [...current, value]);
  return <section className="wk-platform-api-keys grid gap-4"><header className="wk-settings-panel-heading"><h2>{t('platformApiKeys.title') === 'platformApiKeys.title' ? '平台 API 密钥' : t('platformApiKeys.title')}</h2><p>{t('platformApiKeys.description') === 'platformApiKeys.description' ? '创建和撤销 system-admin 使用的 API 密钥。' : t('platformApiKeys.description')}</p></header><Card className="wk-api-key-create"><strong className="text-sm font-semibold text-[#1f2733]">创建 API 密钥</strong><div className="wk-api-key-create-row mt-3 flex gap-2"><input aria-label="密钥名称" placeholder="密钥名称" className="h-8 min-w-0 flex-1 rounded-[4px] border border-[#dcdcdc] bg-white px-2 text-[13px] text-black/90 outline-none focus:border-accent" value={name} onChange={(event) => setName(event.target.value)} disabled={creating} /><button type="button" className="cursor-pointer rounded-[4px] border border-[rgba(120,135,155,0.35)] bg-transparent px-3 py-[5px]" onClick={() => void create()} disabled={creating}>{creating ? '创建中...' : '创建'}</button></div><div className="wk-api-key-capabilities mt-3 flex flex-wrap gap-x-4 gap-y-2">{capabilities.map((value) => <label className="text-xs" key={value}><input type="checkbox" checked={selected.includes(value)} onChange={() => toggle(value)} disabled={creating} />{value}</label>)}</div></Card>{message ? <p className="wk-api-key-message m-0 text-[13px] text-[#c23434]" role="status">{message}</p> : null}{token ? <Card className="wk-api-key-token grid gap-2" role="alert"><strong className="text-sm font-semibold text-[#1f2733]">密钥已创建，请立即复制保存</strong><code className="bg-[rgba(120,135,155,0.1)] px-2 py-2 [overflow-wrap:anywhere]">{token}</code><button type="button" className="cursor-pointer rounded-[4px] border border-[rgba(120,135,155,0.35)] bg-transparent px-3 py-[5px]" onClick={() => setToken(null)}>关闭</button></Card> : null}<Card className="wk-api-key-list">{keys.length === 0 ? <Status>暂无平台 API 密钥</Status> : <div className="wk-api-key-table-wrap overflow-x-auto"><table className="w-full min-w-[680px] border-collapse text-xs [&_th]:border-b [&_th]:border-[rgba(120,135,155,0.18)] [&_th]:px-2 [&_th]:py-3 [&_th]:text-left [&_td]:border-b [&_td]:border-[rgba(120,135,155,0.18)] [&_td]:px-2 [&_td]:py-3 [&_td]:text-left [&_thead_th]:font-medium [&_thead_th]:text-[#5c6b83]"><thead><tr><th>名称</th><th>密钥</th><th>权限</th><th>最近使用</th><th>创建时间</th><th>操作</th></tr></thead><tbody>{keys.map((key) => { const keyCapabilities = key.capabilities ?? []; return <tr key={key.id}><th>{key.name}</th><td><code>{key.api_key}</code></td><td>{keyCapabilities.slice(0, 4).join('、')}{keyCapabilities.length > 4 ? ` +${keyCapabilities.length - 4}` : ''}</td><td>{date(key.last_used_at)}</td><td>{date(key.created_at)}</td><td><button type="button" className="wk-api-key-revoke cursor-pointer rounded-[4px] border border-[rgba(120,135,155,0.35)] bg-transparent px-3 py-[5px]" onClick={() => void revoke(key)}>撤销</button></td></tr>; })}</tbody></table></div>}</Card></section>;
}