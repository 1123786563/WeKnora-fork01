import type { ApiKey, WeKnoraClient } from '@weknora/api-client';
import { Button, Checkbox, Input } from '@weknora/ui';
import { useEffect, useState } from 'react';
import { shouldShowSwaggerDocs, swaggerDocsUrl } from '@weknora/views/integrations/swagger';
import { resolveApiBaseUrl } from '../platform/api-base.ts';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';

const capabilities = ['system_tenants_read', 'system_tenants_manage', 'system_settings_read', 'system_settings_manage', 'system_runtime_read', 'system_runtime_manage', 'system_audit_read'];
const CAPABILITY_LABELS: Record<string, Record<string, string>> = {
  'zh-CN': { system_tenants_read: '查看空间', system_tenants_manage: '管理空间', system_settings_read: '查看系统设置', system_settings_manage: '管理系统设置', system_runtime_read: '查看运行时', system_runtime_manage: '管理运行时', system_audit_read: '查看审计日志' },
  'en-US': { system_tenants_read: 'View tenants', system_tenants_manage: 'Manage tenants', system_settings_read: 'View system settings', system_settings_manage: 'Manage system settings', system_runtime_read: 'View runtime', system_runtime_manage: 'Manage runtime', system_audit_read: 'View audit log' },
  'ja-JP': { system_tenants_read: 'テナントを表示', system_tenants_manage: 'テナントを管理', system_settings_read: 'システム設定を表示', system_settings_manage: 'システム設定を管理', system_runtime_read: 'ランタイムを表示', system_runtime_manage: 'ランタイムを管理', system_audit_read: '監査ログを表示' },
  'ko-KR': { system_tenants_read: '테넌트 보기', system_tenants_manage: '테넌트 관리', system_settings_read: '시스템 설정 보기', system_settings_manage: '시스템 설정 관리', system_runtime_read: '런타임 보기', system_runtime_manage: '런타임 관리', system_audit_read: '감사 로그 보기' },
  'ru-RU': { system_tenants_read: 'Просмотр тенантов', system_tenants_manage: 'Управление тенантами', system_settings_read: 'Просмотр системных настроек', system_settings_manage: 'Управление системными настройками', system_runtime_read: 'Просмотр среды выполнения', system_runtime_manage: 'Управление средой выполнения', system_audit_read: 'Просмотр журнала аудита' },
};
// One-line semantic summary per capability (SP14 Task 2): the checkbox labels
// above only translate the name, which told operators nothing about what a
// checked capability actually grants to the platform key.
const CAPABILITY_DESCRIPTIONS: Record<string, Record<string, string>> = {
  'zh-CN': { system_tenants_read: '只读访问全部工作空间的列表与详情', system_tenants_manage: '创建、修改和删除工作空间', system_settings_read: '读取平台级系统设置', system_settings_manage: '修改平台级系统设置', system_runtime_read: '查看运行时队列与任务状态', system_runtime_manage: '取消任务等运行时管理操作', system_audit_read: '读取系统审计日志' },
  'en-US': { system_tenants_read: 'Read-only access to every workspace list and detail', system_tenants_manage: 'Create, update and delete workspaces', system_settings_read: 'Read platform-level system settings', system_settings_manage: 'Change platform-level system settings', system_runtime_read: 'View runtime queues and task states', system_runtime_manage: 'Runtime management such as cancelling tasks', system_audit_read: 'Read the system audit log' },
  'ja-JP': { system_tenants_read: 'すべてのワークスペースの一覧と詳細を読み取り専用で参照', system_tenants_manage: 'ワークスペースの作成・更新・削除', system_settings_read: 'プラットフォームシステム設定の読み取り', system_settings_manage: 'プラットフォームシステム設定の変更', system_runtime_read: 'ランタイムキューとタスク状態の閲覧', system_runtime_manage: 'タスクキャンセルなどのランタイム管理操作', system_audit_read: 'システム監査ログの読み取り' },
  'ko-KR': { system_tenants_read: '모든 워크스페이스 목록과 세부 정보 읽기 전용 액세스', system_tenants_manage: '워크스페이스 생성·수정·삭제', system_settings_read: '플랫폼 시스템 설정 읽기', system_settings_manage: '플랫폼 시스템 설정 변경', system_runtime_read: '런타임 큐 및 작업 상태 보기', system_runtime_manage: '작업 취소 등 런타임 관리 작업', system_audit_read: '시스템 감사 로그 읽기' },
  'ru-RU': { system_tenants_read: 'Доступ только для чтения ко всем рабочим пространствам', system_tenants_manage: 'Создание, изменение и удаление рабочих пространств', system_settings_read: 'Чтение системных настроек платформы', system_settings_manage: 'Изменение системных настроек платформы', system_runtime_read: 'Просмотр очередей среды выполнения и состояний задач', system_runtime_manage: 'Управление средой выполнения (отмена задач и т. п.)', system_audit_read: 'Чтение журнала аудита системы' },
};
const COPY: Record<string, Record<string, string>> = {
  'zh-CN': { title: '平台 API Key', description: '为跨空间自动化创建平台级凭据；调用空间接口时通过 X-Tenant-ID 指定目标空间。', securityNotice: '平台 API Key 默认可选择任意空间。请只授予必要能力；密钥明文仅在创建时显示一次。', createTitle: '创建 API 密钥', name: '密钥名称', create: '创建平台 API Key', creating: '创建中...', validation: '请填写名称并至少选择一个权限', failed: '创建失败', created: '密钥已创建，请立即复制保存', copy: '复制密钥', copied: '已复制', copyFailed: '复制失败，请手动选择密钥', close: '关闭', empty: '暂无平台 API Key', revoke: '删除', revoked: '已撤销', revokeFailed: '撤销失败', never: '从未使用', nameHead: '名称', keyHead: '密钥', permissionsHead: '权限', lastUsedHead: '最近使用', createdHead: '创建时间', actionHead: '操作', apiDocs: 'API 文档' },
  'en-US': { title: 'Platform API keys', description: 'Create and revoke API keys for system administrators.', createTitle: 'Create API key', name: 'Key name', create: 'Create', creating: 'Creating...', validation: 'Enter a name and select at least one permission', failed: 'Creation failed', created: 'Key created. Copy and save it now.', copy: 'Copy key', copied: 'Copied', copyFailed: 'Copy failed; select the key manually', close: 'Close', empty: 'No platform API keys', revoke: 'Revoke', revoked: 'Revoked', revokeFailed: 'Revocation failed', never: 'Never used', nameHead: 'Name', keyHead: 'Key', permissionsHead: 'Permissions', lastUsedHead: 'Last used', createdHead: 'Created', actionHead: 'Actions', apiDocs: 'API docs' },
  'ja-JP': { title: 'プラットフォーム API キー', description: 'system-admin 用の API キーを作成・取り消しします。', createTitle: 'API キーを作成', name: 'キー名', create: '作成', creating: '作成中...', validation: '名前を入力し、権限を1つ以上選択してください', failed: '作成に失敗しました', created: 'キーを作成しました。今すぐコピーして保存してください。', copy: 'キーをコピー', copied: 'コピーしました', copyFailed: 'コピーに失敗しました。手動で選択してください', close: '閉じる', empty: 'プラットフォーム API キーはありません', revoke: '取り消す', revoked: '取り消しました', revokeFailed: '取り消しに失敗しました', never: '未使用', nameHead: '名前', keyHead: 'キー', permissionsHead: '権限', lastUsedHead: '最終使用', createdHead: '作成日時', actionHead: '操作', apiDocs: 'API ドキュメント' },
  'ko-KR': { title: '플랫폼 API 키', description: 'system-admin용 API 키를 생성하고 취소합니다.', createTitle: 'API 키 생성', name: '키 이름', create: '생성', creating: '생성 중...', validation: '이름을 입력하고 권한을 하나 이상 선택하세요', failed: '생성 실패', created: '키가 생성되었습니다. 지금 복사해 저장하세요.', copy: '키 복사', copied: '복사됨', copyFailed: '복사 실패: 키를 수동으로 선택하세요', close: '닫기', empty: '플랫폼 API 키가 없습니다', revoke: '취소', revoked: '취소됨', revokeFailed: '취소 실패', never: '사용 안 함', nameHead: '이름', keyHead: '키', permissionsHead: '권한', lastUsedHead: '최근 사용', createdHead: '생성 시간', actionHead: '작업', apiDocs: 'API 문서' },
  'ru-RU': { title: 'API-ключи платформы', description: 'Создание и отзыв API-ключей для системных администраторов.', createTitle: 'Создать API-ключ', name: 'Имя ключа', create: 'Создать', creating: 'Создание...', validation: 'Введите имя и выберите хотя бы одно разрешение', failed: 'Ошибка создания', created: 'Ключ создан. Скопируйте и сохраните его сейчас.', copy: 'Копировать ключ', copied: 'Скопировано', copyFailed: 'Не удалось скопировать, выберите ключ вручную', close: 'Закрыть', empty: 'Нет API-ключей платформы', revoke: 'Отозвать', revoked: 'Отозван', revokeFailed: 'Ошибка отзыва', never: 'Не использовался', nameHead: 'Имя', keyHead: 'Ключ', permissionsHead: 'Разрешения', lastUsedHead: 'Последнее использование', createdHead: 'Создан', actionHead: 'Действия', apiDocs: 'Документация API' },
};
function date(value: string | undefined, locale: string, never: string) { return value ? new Date(value).toLocaleString(locale, { hour12: false }) : never; }

export function PlatformApiKeysPanel({ client, initialKeys }: { client: WeKnoraClient; initialKeys: ApiKey[] }) {
  const locale = useSettingsLocale();
  const t = settingsT(locale);
  const copy = COPY[locale] ?? COPY['zh-CN'];
  const capabilityLabels = CAPABILITY_LABELS[locale] ?? CAPABILITY_LABELS['zh-CN'];
  const capabilityDescriptions = CAPABILITY_DESCRIPTIONS[locale] ?? CAPABILITY_DESCRIPTIONS['zh-CN'];
  const [keys, setKeys] = useState(initialKeys);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [selected, setSelected] = useState<string[]>([]);
  const [token, setToken] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  // SP14 Task 2 — the header "API 文档" link renders only when /system/info
  // confirms swagger_enabled: release builds disable the route and older
  // backends lack the field, so a failed probe keeps the link hidden.
  const [swaggerEnabled, setSwaggerEnabled] = useState(false);
  useEffect(() => {
    let current = true;
    void client.settings.system.info()
      .then((info) => { if (current) setSwaggerEnabled(shouldShowSwaggerDocs(info.swagger_enabled)); })
      .catch(() => { if (current) setSwaggerEnabled(false); });
    return () => { current = false; };
  }, [client]);
  async function create() {
    if (!name.trim() || selected.length === 0) { setMessage(copy.validation); return; }
    setCreating(true); setMessage(null);
    try { const created = await client.administration.apiKeys.create({ name: name.trim(), capabilities: selected }); setKeys((current) => [...current, created]); setToken(created.token ?? created.api_key); setCopied(false); setName(''); setSelected([]); }
    catch (reason) { setMessage(reason instanceof Error ? reason.message : copy.failed); }
    finally { setCreating(false); }
  }
  async function revoke(key: ApiKey) {
    try { await client.administration.apiKeys.revoke(key.id); setKeys((current) => current.filter((item) => item.id !== key.id)); setMessage(copy.revoked); }
    catch (reason) { setMessage(reason instanceof Error ? reason.message : copy.revokeFailed); }
  }
  async function copyToken() {
    if (!token) return;
    try { await navigator.clipboard.writeText(token); setCopied(true); setMessage(null); }
    catch { setCopied(false); setMessage(copy.copyFailed); }
  }
  const toggle = (value: string) => setSelected((current) => current.includes(value) ? current.filter((item) => item !== value) : [...current, value]);
  const title = copy.title;
  const [drawerOpen, setDrawerOpen] = useState(false);
  const openCreate = () => { setName(''); setSelected([]); setDrawerOpen(true); };
  async function createAndClose() {
    await create();
    if (!message) setDrawerOpen(false);
  }
  return <section className="platform-api-keys">
    <header className="section-header">
      <h2>{title}</h2>
      <p className="section-description">{copy.description}</p>
    </header>
    <div className="pak-security-alert" role="status">
      <span className="pak-security-alert__icon" aria-hidden="true"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="9" /><path d="M12 7v6M12 16h.01" /></svg></span>
      <p className="pak-security-alert__text">{copy.securityNotice}</p>
      <button type="button" className="pak-outline-btn" onClick={openCreate}><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>{copy.create}</button>
    </div>
    {message ? <p className="wk-api-key-message m-0 mb-3 text-[13px] text-[#c23434]" role="status">{message}</p> : null}
    {token ? <div className="pak-token-card" role="alert"><strong className="text-sm font-semibold text-[#1f2733]">{copy.created}</strong><code className="bg-[rgba(120,135,155,0.1)] px-2 py-2 [overflow-wrap:anywhere]">{token}</code><div className="flex gap-2"><Button type="button" onClick={() => void copyToken()}>{copied ? copy.copied : copy.copy}</Button><Button type="button" onClick={() => setToken(null)}>{copy.close}</Button></div></div> : null}
    <section className="pak-keys-section">
      {keys.length === 0 ? <div className="pak-keys-state pak-keys-state--empty">
        <span>{copy.empty}</span>
        <button type="button" className="pak-outline-btn" onClick={openCreate}><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>{copy.create}</button>
      </div> : <div className="pak-table-wrap"><table className="pak-table">
        <thead><tr><th>{copy.nameHead}</th><th>{copy.keyHead}</th><th>{copy.permissionsHead}</th><th>{copy.lastUsedHead}</th><th>{copy.createdHead}</th><th className="pak-table__actions">{copy.actionHead}</th></tr></thead>
        <tbody>{keys.map((key) => { const keyCapabilities = key.capabilities ?? []; return <tr key={key.id}>
          <td><span className="pak-key-name">{key.name}</span></td>
          <td><code className="pak-key-fingerprint">{key.api_key}</code></td>
          <td><div className="pak-chips">{keyCapabilities.slice(0, 4).map((capability) => <span className="pak-chip" key={capability}>{capabilityLabels[capability] ?? capability}</span>)}{keyCapabilities.length > 4 ? <span className="pak-chip pak-chip--more">+{keyCapabilities.length - 4}</span> : null}</div></td>
          <td>{date(key.last_used_at, locale, copy.never)}</td>
          <td>{date(key.created_at, locale, copy.never)}</td>
          <td className="pak-table__actions"><button type="button" className="pak-icon-btn" title={copy.revoke} aria-label={copy.revoke} onClick={() => void revoke(key)}><svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" /></svg></button></td>
        </tr>; })}</tbody>
      </table></div>}
    </section>
    {swaggerEnabled ? <a className="pak-docs-link" href={swaggerDocsUrl(resolveApiBaseUrl())} target="_blank" rel="noreferrer">{copy.apiDocs}</a> : null}
    {drawerOpen ? <div className="pak-drawer" role="dialog" aria-modal="true" aria-label={copy.createTitle}>
      <div className="rq-drawer-backdrop" onClick={() => setDrawerOpen(false)} />
      <div className="pak-drawer-panel">
        <header className="rq-drawer-head">
          <div>
            <h3>{copy.createTitle}</h3>
            <p>{copy.description}</p>
          </div>
          <button type="button" aria-label={copy.close} onClick={() => setDrawerOpen(false)}>×</button>
        </header>
        <label className="pak-field"><span>{copy.name}</span><Input value={name} onChange={(event) => setName(event.target.value)} disabled={creating} aria-label={copy.name} /></label>
        <div className="pak-field"><span>{copy.permissionsHead}</span>
          <div className="pak-cap-group">
            <div className="pak-cap-group__title">平台控制面</div>
            <div className="pak-cap-items">{capabilities.map((value) => <label key={value} className="pak-cap-item"><Checkbox checked={selected.includes(value)} onChange={() => toggle(value)} disabled={creating} />{capabilityLabels[value] ?? value}<span className="block text-[11px] leading-[1.45] font-normal text-[#8a94a6]">{capabilityDescriptions[value] ?? ''}</span></label>)}</div>
          </div>
        </div>
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" onClick={() => setDrawerOpen(false)}>{copy.close}</Button>
          <Button type="button" loading={creating} onClick={() => void createAndClose()}>{creating ? copy.creating : copy.create}</Button>
        </div>
      </div>
    </div> : null}
  </section>;
}
