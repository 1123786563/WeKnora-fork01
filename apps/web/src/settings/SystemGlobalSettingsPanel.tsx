import type { SystemSetting, WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { useEffect, useState } from 'react';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';

type Group = 'access' | 'tenant' | 'runtime' | 'security' | 'other';
const groups: Record<Exclude<Group, 'other'>, string[]> = {
  access: ['auth.registration_mode', 'auth.complex_password_enabled', 'auth.default_tenant_mode', 'tenant.self_service_creation_enabled', 'tenant.max_owned_per_user'],
  tenant: ['tenant.default_storage_quota_gb', 'tenant.auto_create_api_key', 'tenant.auto_accept_invitation'],
  runtime: ['asynq.core_concurrency', 'asynq.enrichment_concurrency', 'asynq.postprocess_concurrency', 'asynq.maintenance_concurrency', 'asynq.shared_concurrency', 'asynq.wiki_concurrency', 'model.max_concurrency'],
  security: ['ssrf.whitelist', 'sandbox.docker_enabled'],
};
const groupLabels: Record<Group, string> = { access: '访问控制', tenant: '空间', runtime: '运行时', security: '安全', other: '其他' };
const highRiskKeys = new Set(['auth.registration_mode', 'sandbox.docker_enabled']);
function label(key: string): string { return key.replaceAll('.', ' · ').replaceAll('_', ' '); }
function valuesFor(item: SystemSetting): string[] { return Array.isArray(item.enum) ? item.enum : []; }

export function SystemGlobalSettingsPanel({ client, initialSettings }: { client: WeKnoraClient; initialSettings: SystemSetting[] }) {
  const locale = useSettingsLocale();
  const baseT = settingsT(locale);
  const [group, setGroup] = useState<Group>('access');
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [saving, setSaving] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [pending, setPending] = useState<{ item: SystemSetting; value: unknown } | null>(null);
  useEffect(() => setValues(Object.fromEntries(initialSettings.map((item) => [item.key, item.value]))), [initialSettings]);
  const known = new Set(Object.values(groups).flat());
  const rows = initialSettings.filter((item) => group === 'other' ? !known.has(item.key) : groups[group].includes(item.key));
  const text = (key: string, fallback: string) => { const value = baseT(key); return value === key ? fallback : value; };
  async function persist(item: SystemSetting, value: unknown) {
    setSaving(item.key); setMessage(null); setValues((current) => ({ ...current, [item.key]: value }));
    try { await client.administration.settings.update(item.key, value); setMessage('已保存'); }
    catch (reason) { setMessage(reason instanceof Error ? reason.message : '保存失败'); }
    finally { setSaving(null); }
  }
  function requestPersist(item: SystemSetting, value: unknown) {
    if (highRiskKeys.has(item.key)) setPending({ item, value });
    else void persist(item, value);
  }
  async function reset(item: SystemSetting) {
    setSaving(item.key); setMessage(null);
    try { await client.administration.settings.reset(item.key); setMessage('已恢复默认值'); }
    catch (reason) { setMessage(reason instanceof Error ? reason.message : '恢复失败'); }
    finally { setSaving(null); }
  }
  return <section className="wk-system-global" aria-label={text('system.globalSettings.title', '系统全局设置')}>
    <header className="wk-settings-panel-heading"><h2>{text('system.globalSettings.title', '系统全局设置')}</h2><p>{text('system.globalSettings.description', '管理平台级配置。修改会立即保存。')}</p></header>
    {message ? <p className="wk-system-global-message" role="status">{message}</p> : null}
    {pending ? <div className="wk-system-global-confirm" role="alertdialog" aria-modal="true" aria-label="确认高风险配置变更"><strong>确认高风险配置变更？</strong><p>此修改可能影响整个平台的访问或执行能力。</p><div><button type="button" onClick={() => { const next = pending; setPending(null); void persist(next.item, next.value); }}>确认</button><button type="button" onClick={() => setPending(null)}>取消</button></div></div> : null}
    {initialSettings.length === 0 ? <Card><Status>{text('system.globalSettings.empty', '暂无系统设置')}</Status></Card> : <>
      <div className="wk-system-global-tabs" role="tablist" aria-label="系统设置分组">{(Object.keys(groups) as Group[]).concat(initialSettings.some((item) => !known.has(item.key)) ? ['other'] : []).map((key) => <button key={key} type="button" role="tab" aria-selected={group === key} className={group === key ? 'is-active' : ''} onClick={() => setGroup(key)}>{groupLabels[key]}</button>)}</div>
      <div className="wk-system-global-rows">{rows.length === 0 ? <Card><Status>此分组暂无配置</Status></Card> : rows.map((item) => { const current = values[item.key]; const enums = valuesFor(item); return <Card key={item.key} className="wk-system-global-row"><div className="wk-system-global-info"><strong>{label(item.key)}</strong>{item.description ? <p className="wk-muted">{item.description}</p> : null}<div className="wk-system-global-meta">{item.requires_restart ? '需要重启 · ' : ''}{item.is_secret ? '敏感配置' : ''}</div></div><div className="wk-system-global-control">{item.value_type === 'bool' ? <label><input type="checkbox" checked={current === true} disabled={saving === item.key} onChange={(event) => requestPersist(item, event.target.checked)} /> 启用</label> : enums.length > 0 ? <select value={String(current ?? '')} disabled={saving === item.key} onChange={(event) => requestPersist(item, event.target.value)}>{enums.map((option) => <option key={option} value={option}>{option}</option>)}</select> : item.value_type === 'int' ? <input type="number" value={typeof current === 'number' ? current : Number(current ?? 0)} disabled={saving === item.key} onBlur={(event) => void persist(item, Number(event.target.value))} /> : <input value={Array.isArray(current) ? current.join(', ') : String(current ?? '')} disabled={saving === item.key} onBlur={(event) => void persist(item, event.target.value.split(',').map((part) => part.trim()).filter(Boolean))} />}{saving !== item.key && <button type="button" className="wk-system-global-reset" onClick={() => void reset(item)}>恢复默认</button>}</div></Card>; })}</div>
    </>}
  </section>;
}
