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
const SYSTEM_COPY: Record<string, Record<string, string>> = {
  'zh-CN': { confirmTitle: '确认高风险配置变更？', confirmBody: '此修改可能影响整个平台的访问或执行能力。', confirm: '确认', cancel: '取消', groups: '系统设置分组', empty: '暂无系统设置', emptyGroup: '此分组暂无配置', restart: '需要重启', secret: '敏感配置', enable: '启用', reset: '恢复默认', saved: '已保存', saveFailed: '保存失败', resetSuccess: '已恢复默认值', resetFailed: '恢复失败' },
  'en-US': { confirmTitle: 'Confirm high-risk configuration change?', confirmBody: 'This change may affect platform access or execution.', confirm: 'Confirm', cancel: 'Cancel', groups: 'System setting groups', empty: 'No system settings', emptyGroup: 'No settings in this group', restart: 'Restart required', secret: 'Sensitive configuration', enable: 'Enabled', reset: 'Restore default', saved: 'Saved', saveFailed: 'Save failed', resetSuccess: 'Default restored', resetFailed: 'Restore failed' },
  'ja-JP': { confirmTitle: '高リスク設定の変更を確認しますか？', confirmBody: 'この変更はプラットフォームのアクセスや実行に影響する可能性があります。', confirm: '確認', cancel: 'キャンセル', groups: 'システム設定グループ', empty: 'システム設定はありません', emptyGroup: 'このグループに設定はありません', restart: '再起動が必要', secret: '機密設定', enable: '有効', reset: 'デフォルトに戻す', saved: '保存しました', saveFailed: '保存に失敗しました', resetSuccess: 'デフォルトに戻しました', resetFailed: '復元に失敗しました' },
  'ko-KR': { confirmTitle: '고위험 설정 변경을 확인하시겠습니까?', confirmBody: '이 변경은 플랫폼 접근 또는 실행에 영향을 줄 수 있습니다.', confirm: '확인', cancel: '취소', groups: '시스템 설정 그룹', empty: '시스템 설정이 없습니다', emptyGroup: '이 그룹에는 설정이 없습니다', restart: '재시작 필요', secret: '민감한 설정', enable: '사용', reset: '기본값 복원', saved: '저장됨', saveFailed: '저장 실패', resetSuccess: '기본값으로 복원됨', resetFailed: '복원 실패' },
  'ru-RU': { confirmTitle: 'Подтвердить изменение опасной настройки?', confirmBody: 'Это изменение может повлиять на доступ и выполнение на платформе.', confirm: 'Подтвердить', cancel: 'Отмена', groups: 'Группы системных настроек', empty: 'Системных настроек нет', emptyGroup: 'В этой группе нет настроек', restart: 'Требуется перезапуск', secret: 'Секретная настройка', enable: 'Включить', reset: 'Восстановить по умолчанию', saved: 'Сохранено', saveFailed: 'Ошибка сохранения', resetSuccess: 'Значение по умолчанию восстановлено', resetFailed: 'Ошибка восстановления' },
};
const GROUP_COPY: Record<string, Record<Group, string>> = {
  'zh-CN': { access: '访问控制', tenant: '空间', runtime: '运行时', security: '安全', other: '其他' },
  'en-US': { access: 'Access', tenant: 'Tenant', runtime: 'Runtime', security: 'Security', other: 'Other' },
  'ja-JP': { access: 'アクセス', tenant: 'テナント', runtime: 'ランタイム', security: 'セキュリティ', other: 'その他' },
  'ko-KR': { access: '접근 제어', tenant: '테넌트', runtime: '런타임', security: '보안', other: '기타' },
  'ru-RU': { access: 'Доступ', tenant: 'Тенант', runtime: 'Среда выполнения', security: 'Безопасность', other: 'Другое' },
};
const highRiskKeys = new Set(['auth.registration_mode', 'sandbox.docker_enabled']);
function label(key: string): string { return key.replaceAll('.', ' · ').replaceAll('_', ' '); }
function valuesFor(item: SystemSetting): string[] { return Array.isArray(item.enum) ? item.enum : []; }

export function SystemGlobalSettingsPanel({ client, initialSettings }: { client: WeKnoraClient; initialSettings: SystemSetting[] }) {
  const locale = useSettingsLocale();
  const baseT = settingsT(locale);
  const copy = SYSTEM_COPY[locale] ?? SYSTEM_COPY['zh-CN'];
  const groupCopy = GROUP_COPY[locale] ?? GROUP_COPY['zh-CN'];
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
    try { await client.administration.settings.update(item.key, value); setMessage(copy.saved); }
    catch (reason) { setMessage(reason instanceof Error ? reason.message : copy.saveFailed); }
    finally { setSaving(null); }
  }
  function requestPersist(item: SystemSetting, value: unknown) {
    if (highRiskKeys.has(item.key)) setPending({ item, value });
    else void persist(item, value);
  }
  async function reset(item: SystemSetting) {
    setSaving(item.key); setMessage(null);
    try { await client.administration.settings.reset(item.key); setMessage(copy.resetSuccess); }
    catch (reason) { setMessage(reason instanceof Error ? reason.message : copy.resetFailed); }
    finally { setSaving(null); }
  }
  return <section className="wk-system-global grid gap-4" aria-label={text('system.globalSettings.title', '系统全局设置')}>
    <header className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col"><h2 className="my-1">{text('system.globalSettings.title', '系统全局设置')}</h2><p className="m-0">{text('system.globalSettings.description', '管理平台级配置。修改会立即保存。')}</p></header>
    {message ? <p className="wk-system-global-message m-0 text-[13px] text-[#0a8f4c]" role="status">{message}</p> : null}
    {pending ? <div className="wk-system-global-confirm rounded-lg border border-[rgba(178,106,8,0.35)] bg-[#fff8e6] px-4 py-3.5" role="alertdialog" aria-modal="true" aria-label={copy.confirmTitle}><strong className="text-sm font-semibold text-[#1f2937]">{copy.confirmTitle}</strong><p className="m-0 mt-1.5 mb-3 text-[13px] text-[#5c6b83]">{copy.confirmBody}</p><div><button type="button" className="mr-2 cursor-pointer rounded-[4px] border border-[#b26a08] bg-[#b26a08] px-3 py-[5px] text-white" onClick={() => { const next = pending; setPending(null); void persist(next.item, next.value); }}>{copy.confirm}</button><button type="button" className="cursor-pointer rounded-[4px] border border-[rgba(120,135,155,0.35)] bg-transparent px-3 py-[5px]" onClick={() => setPending(null)}>{copy.cancel}</button></div></div> : null}
    {initialSettings.length === 0 ? <Card><Status>{copy.empty}</Status></Card> : <>
      <div className="wk-system-global-tabs flex overflow-x-auto gap-1 border-b border-[rgba(120,135,155,0.22)]" role="tablist" aria-label={copy.groups}>{(Object.keys(groups) as Group[]).concat(initialSettings.some((item) => !known.has(item.key)) ? ['other'] : []).map((key) => <button key={key} type="button" role="tab" aria-selected={group === key} className={`cursor-pointer whitespace-nowrap border-0 border-b-2 bg-transparent px-3 py-[9px] font-[inherit] text-[#5c6b83] transition-colors ${group === key ? 'is-active border-b-[#0a8f4c] font-semibold text-[#0a8f4c]' : 'border-b-transparent'}`} onClick={() => setGroup(key)}>{groupCopy[key]}</button>)}</div>
      <div className="wk-system-global-rows grid gap-2.5">{rows.length === 0 ? <Card><Status>{copy.emptyGroup}</Status></Card> : rows.map((item) => { const current = values[item.key]; const enums = valuesFor(item); return <Card key={item.key} className="wk-system-global-row flex items-start justify-between gap-6"><div className="wk-system-global-info min-w-0 flex-1"><strong className="text-sm">{label(item.key)}</strong>{item.description ? <p className="wk-muted text-muted">{item.description}</p> : null}<div className="wk-system-global-meta text-xs text-[#8a97ab]">{item.requires_restart ? `${copy.restart} · ` : ''}{item.is_secret ? copy.secret : ''}</div></div><div className="wk-system-global-control flex shrink-0 flex-col items-end gap-2 [&_input]:box-border [&_input]:h-8 [&_input]:min-w-[180px] [&_select]:box-border [&_select]:h-8 [&_select]:min-w-[180px] [&_select]:[font:inherit] [&_input]:[font:inherit] [&_input[type='checkbox']]:h-auto [&_input[type='checkbox']]:min-w-0">{item.value_type === 'bool' ? <label className="grid gap-[.35rem] text-[#27364d] font-semibold"><input type="checkbox" checked={current === true} disabled={saving === item.key} onChange={(event) => requestPersist(item, event.target.checked)} /> {copy.enable}</label> : enums.length > 0 ? <select value={String(current ?? '')} disabled={saving === item.key} onChange={(event) => requestPersist(item, event.target.value)}>{enums.map((option) => <option key={option} value={option}>{option}</option>)}</select> : item.value_type === 'int' ? <input type="number" value={typeof current === 'number' ? current : Number(current ?? 0)} disabled={saving === item.key} onBlur={(event) => void persist(item, Number(event.target.value))} /> : <input value={Array.isArray(current) ? current.join(', ') : String(current ?? '')} disabled={saving === item.key} onBlur={(event) => void persist(item, event.target.value.split(',').map((part) => part.trim()).filter(Boolean))} />}{saving !== item.key && <button type="button" className="wk-system-global-reset cursor-pointer border-0 bg-transparent p-0 text-xs text-[#0a8f4c]" onClick={() => void reset(item)}>{copy.reset}</button>}</div></Card>; })}</div>
    </>}
  </section>;
}
