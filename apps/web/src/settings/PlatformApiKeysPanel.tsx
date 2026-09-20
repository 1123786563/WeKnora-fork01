import type { ApiKey, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Checkbox, Input, Status } from '@weknora/ui';
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
  'zh-CN': { title: '平台 API 密钥', description: '创建和撤销 system-admin 使用的 API 密钥。', createTitle: '创建 API 密钥', name: '密钥名称', create: '创建', creating: '创建中...', validation: '请填写名称并至少选择一个权限', failed: '创建失败', created: '密钥已创建，请立即复制保存', copy: '复制密钥', copied: '已复制', copyFailed: '复制失败，请手动选择密钥', close: '关闭', empty: '暂无平台 API 密钥', revoke: '撤销', revoked: '已撤销', revokeFailed: '撤销失败', never: '从未使用', nameHead: '名称', keyHead: '密钥', permissionsHead: '权限', lastUsedHead: '最近使用', createdHead: '创建时间', actionHead: '操作', apiDocs: 'API 文档' },
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
  const title = t('platformApiKeys.title') === 'platformApiKeys.title' ? copy.title : t('platformApiKeys.title');
  return <section className="wk-platform-api-keys grid gap-4"><header className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col"><div><h2 className="m-0">{title}</h2><p className="m-0">{copy.description}</p></div>{swaggerEnabled ? <a className="shrink-0 whitespace-nowrap rounded-[4px] border border-solid border-[#dcdcdc] px-[0.85rem] py-[0.45rem] text-[13px] text-[#1f2733] [font:inherit] no-underline hover:border-[#2563eb]" href={swaggerDocsUrl(resolveApiBaseUrl())} target="_blank" rel="noreferrer">{copy.apiDocs}</a> : null}</header><Card className="wk-api-key-create"><strong className="text-sm font-semibold text-[#1f2733]">{copy.createTitle}</strong><div className="wk-api-key-create-row mt-3 flex gap-2"><Input aria-label={copy.name} placeholder={copy.name} className="h-8 min-w-0 flex-1 rounded-[4px] border border-[#dcdcdc] px-2 text-[13px]" value={name} onChange={(event) => setName(event.target.value)} disabled={creating} /><Button type="button" onClick={() => void create()} disabled={creating}>{creating ? copy.creating : copy.create}</Button></div><div className="wk-api-key-capabilities mt-3 flex flex-wrap gap-x-4 gap-y-2">{capabilities.map((value) => <label className="text-xs" key={value} title={capabilityDescriptions[value] ?? ''}><Checkbox checked={selected.includes(value)} onChange={() => toggle(value)} disabled={creating} />{capabilityLabels[value] ?? value}<span className="mt-[0.15rem] block text-[11px] leading-[1.45] font-normal text-[#8a94a6]">{capabilityDescriptions[value] ?? ''}</span></label>)}</div></Card>{message ? <p className="wk-api-key-message m-0 text-[13px] text-[#c23434]" role="status">{message}</p> : null}{token ? <Card className="wk-api-key-token grid gap-2" role="alert"><strong className="text-sm font-semibold text-[#1f2733]">{copy.created}</strong><code className="bg-[rgba(120,135,155,0.1)] px-2 py-2 [overflow-wrap:anywhere]">{token}</code><div className="flex gap-2"><Button type="button" onClick={() => void copyToken()}>{copied ? copy.copied : copy.copy}</Button><Button type="button" onClick={() => setToken(null)}>{copy.close}</Button></div></Card> : null}<Card className="wk-api-key-list">{keys.length === 0 ? <Status>{copy.empty}</Status> : <div className="wk-api-key-table-wrap overflow-x-auto"><table className="w-full min-w-[680px] border-collapse text-xs [&_th]:border-b [&_th]:border-[rgba(120,135,155,0.18)] [&_th]:px-2 [&_th]:py-3 [&_th]:text-left [&_td]:border-b [&_td]:border-[rgba(120,135,155,0.18)] [&_td]:px-2 [&_td]:py-3 [&_td]:text-left [&_thead_th]:font-medium [&_thead_th]:text-[#5c6b83]"><thead><tr><th>{copy.nameHead}</th><th>{copy.keyHead}</th><th>{copy.permissionsHead}</th><th>{copy.lastUsedHead}</th><th>{copy.createdHead}</th><th>{copy.actionHead}</th></tr></thead><tbody>{keys.map((key) => { const keyCapabilities = key.capabilities ?? []; return <tr key={key.id}><th>{key.name}</th><td><code>{key.api_key}</code></td><td>{keyCapabilities.slice(0, 4).map((capability) => capabilityLabels[capability] ?? capability).join('、')}{keyCapabilities.length > 4 ? ` +${keyCapabilities.length - 4}` : ''}</td><td>{date(key.last_used_at, locale, copy.never)}</td><td>{date(key.created_at, locale, copy.never)}</td><td><Button type="button" className="wk-api-key-revoke" onClick={() => void revoke(key)}>{copy.revoke}</Button></td></tr>; })}</tbody></table></div>}</Card></section>;
}
