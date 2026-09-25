import type { ApiKey, WeKnoraClient } from '@weknora/api-client';
// T12c：t-alert warning + t-button(size small, variant outline, add icon)
// 对齐 Vue PlatformAPIKeys.vue:8-18 / :31-37（sprite glyph 图标）。
// 批 3 终局：整面板按 Vue PlatformAPIKeys.vue 同构——列表表格 api-key-table
// 家族 + 能力 chip inline/popup + popconfirm 删除 + SettingDrawer 创建抽屉
// （api-key-create-drawer 家族，width 560/min480/max920/storageKey
// setting-drawer:width:platform-api-key-create/closeOnOverlayClick false）+
// 平台控制面/四空间能力分组 + 组全选/清空行。样式平移 settings.td.css §19。
import { Alert, Button as TButton, Checkbox as TCheckbox, Dialog as TDialog, Input as TInput, Popconfirm as TPopconfirm, Popup as TPopup, Textarea as TTextarea } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { useState } from 'react';
import { SettingDrawer } from './SettingDrawer.tsx';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';

/* 能力分组（frontend/src/config/apiKeyCapabilities.ts 同构）：
 * SYSTEM 组（平台控制面）在前 + 四个空间能力组。labelKey/hintKey 直指
 * packages/i18n settingsMessages 平移键。 */
type CapabilityOption = { value: string; labelKey: string; hintKey: string };
type CapabilityGroup = { key: string; labelKey: string; capabilities: CapabilityOption[] };

const SPACE_CAPABILITY_VALUES = [
  'retrieve', 'chat', 'read_agents', 'ingest', 'manage_kbs',
  'message_history', 'manage_agents', 'manage_mcp_services',
  'manage_datasources', 'manage_models', 'manage_vector_stores',
  'manage_storage_backends', 'manage_web_search', 'manage_channels',
  'run_evaluations', 'manage_members', 'manage_spaces',
  'manage_tenant_settings',
] as const;

const SYSTEM_CAPABILITY_GROUP: CapabilityGroup = {
  key: 'system',
  labelKey: 'platformApiKeys.systemCapabilityGroup',
  capabilities: [
    { value: 'system_tenants_read', labelKey: 'platformApiKeys.capabilities.tenantsRead', hintKey: 'platformApiKeys.capabilityHints.tenantsRead' },
    { value: 'system_tenants_manage', labelKey: 'platformApiKeys.capabilities.tenantsManage', hintKey: 'platformApiKeys.capabilityHints.tenantsManage' },
    { value: 'system_settings_read', labelKey: 'platformApiKeys.capabilities.settingsRead', hintKey: 'platformApiKeys.capabilityHints.settingsRead' },
    { value: 'system_settings_manage', labelKey: 'platformApiKeys.capabilities.settingsManage', hintKey: 'platformApiKeys.capabilityHints.settingsManage' },
    { value: 'system_runtime_read', labelKey: 'platformApiKeys.capabilities.runtimeRead', hintKey: 'platformApiKeys.capabilityHints.runtimeRead' },
    { value: 'system_runtime_manage', labelKey: 'platformApiKeys.capabilities.runtimeManage', hintKey: 'platformApiKeys.capabilityHints.runtimeManage' },
    { value: 'system_audit_read', labelKey: 'platformApiKeys.capabilities.auditRead', hintKey: 'platformApiKeys.capabilityHints.auditRead' },
  ],
};

const SPACE_CAPABILITY_OPTION: Record<string, CapabilityOption> = {
  retrieve: { value: 'retrieve', labelKey: 'integrations.api.capabilityRetrieve', hintKey: 'integrations.api.capabilityRetrieveHint' },
  chat: { value: 'chat', labelKey: 'integrations.api.capabilityChat', hintKey: 'integrations.api.capabilityChatHint' },
  read_agents: { value: 'read_agents', labelKey: 'integrations.api.capabilityReadAgents', hintKey: 'integrations.api.capabilityReadAgentsHint' },
  ingest: { value: 'ingest', labelKey: 'integrations.api.capabilityIngest', hintKey: 'integrations.api.capabilityIngestHint' },
  manage_kbs: { value: 'manage_kbs', labelKey: 'integrations.api.capabilityManageKbs', hintKey: 'integrations.api.capabilityManageKbsHint' },
  message_history: { value: 'message_history', labelKey: 'integrations.api.capabilityMessageHistory', hintKey: 'integrations.api.capabilityMessageHistoryHint' },
  manage_agents: { value: 'manage_agents', labelKey: 'integrations.api.capabilityManageAgents', hintKey: 'integrations.api.capabilityManageAgentsHint' },
  manage_mcp_services: { value: 'manage_mcp_services', labelKey: 'integrations.api.capabilityManageMcpServices', hintKey: 'integrations.api.capabilityManageMcpServicesHint' },
  manage_datasources: { value: 'manage_datasources', labelKey: 'integrations.api.capabilityManageDatasources', hintKey: 'integrations.api.capabilityManageDatasourcesHint' },
  manage_models: { value: 'manage_models', labelKey: 'integrations.api.capabilityManageModels', hintKey: 'integrations.api.capabilityManageModelsHint' },
  manage_vector_stores: { value: 'manage_vector_stores', labelKey: 'integrations.api.capabilityManageVectorStores', hintKey: 'integrations.api.capabilityManageVectorStoresHint' },
  manage_storage_backends: { value: 'manage_storage_backends', labelKey: 'integrations.api.capabilityManageStorageBackends', hintKey: 'integrations.api.capabilityManageStorageBackendsHint' },
  manage_web_search: { value: 'manage_web_search', labelKey: 'integrations.api.capabilityManageWebSearch', hintKey: 'integrations.api.capabilityManageWebSearchHint' },
  manage_channels: { value: 'manage_channels', labelKey: 'integrations.api.capabilityManageChannels', hintKey: 'integrations.api.capabilityManageChannelsHint' },
  run_evaluations: { value: 'run_evaluations', labelKey: 'integrations.api.capabilityRunEvaluations', hintKey: 'integrations.api.capabilityRunEvaluationsHint' },
  manage_members: { value: 'manage_members', labelKey: 'integrations.api.capabilityManageMembers', hintKey: 'integrations.api.capabilityManageMembersHint' },
  manage_spaces: { value: 'manage_spaces', labelKey: 'integrations.api.capabilityManageSpaces', hintKey: 'integrations.api.capabilityManageSpacesHint' },
  manage_tenant_settings: { value: 'manage_tenant_settings', labelKey: 'integrations.api.capabilityManageTenantSettings', hintKey: 'integrations.api.capabilityManageTenantSettingsHint' },
};

const SPACE_CAPABILITY_GROUPS: CapabilityGroup[] = [
  {
    key: 'knowledge',
    labelKey: 'integrations.api.apiKeyCapabilityGroupKnowledge',
    capabilities: ['retrieve', 'chat', 'ingest', 'manage_kbs', 'message_history'].map((v) => SPACE_CAPABILITY_OPTION[v]),
  },
  {
    key: 'automation',
    labelKey: 'integrations.api.apiKeyCapabilityGroupAutomation',
    capabilities: ['read_agents', 'manage_agents', 'manage_mcp_services', 'manage_datasources'].map((v) => SPACE_CAPABILITY_OPTION[v]),
  },
  {
    key: 'collaboration',
    labelKey: 'integrations.api.apiKeyCapabilityGroupCollaboration',
    capabilities: ['manage_members', 'manage_spaces'].map((v) => SPACE_CAPABILITY_OPTION[v]),
  },
  {
    key: 'tenant',
    labelKey: 'integrations.api.apiKeyCapabilityGroupTenant',
    capabilities: ['manage_models', 'manage_vector_stores', 'manage_storage_backends', 'manage_web_search', 'manage_channels', 'run_evaluations', 'manage_tenant_settings'].map((v) => SPACE_CAPABILITY_OPTION[v]),
  },
];

const PLATFORM_API_KEY_CAPABILITY_GROUPS: CapabilityGroup[] = [
  SYSTEM_CAPABILITY_GROUP,
  ...SPACE_CAPABILITY_GROUPS,
];

const ALL_CAPABILITIES: string[] = [
  ...SPACE_CAPABILITY_VALUES,
  ...SYSTEM_CAPABILITY_GROUP.capabilities.map((item) => item.value),
];

const VISIBLE_CAPABILITY_CHIP_COUNT = 4;

// Vue formatDate（PlatformAPIKeys.vue:278-283）——YYYY-MM-DD HH:mm 手工 pad。
function formatDate(value: string | undefined, neverLabel: string): string {
  if (!value) return neverLabel;
  const date = new Date(value);
  const pad = (part: number) => String(part).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function PlatformApiKeysPanel({ client, initialKeys }: { client: WeKnoraClient; initialKeys: ApiKey[] }) {
  const locale = useSettingsLocale();
  const t = settingsT(locale);
  const [keys, setKeys] = useState(initialKeys);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  // Vue selected 是 Record<capability, boolean>（PlatformAPIKeys.vue:231-236）。
  const [selected, setSelected] = useState<Record<string, boolean>>(() =>
    ALL_CAPABILITIES.reduce<Record<string, boolean>>((result, capability) => {
      result[capability] = false;
      return result;
    }, {}));
  const [token, setToken] = useState('');
  const [tokenVisible, setTokenVisible] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  // Vue PlatformAPIKeys.vue 无 swagger/API 文档链接（React SP14 曾加，为
  // 面板同构移除；如需恢复应先在 Vue 端补齐）。

  function resetForm() {
    setName('');
    setSelected(ALL_CAPABILITIES.reduce<Record<string, boolean>>((result, capability) => {
      result[capability] = false;
      return result;
    }, {}));
  }

  const [createDrawerVisible, setCreateDrawerVisible] = useState(false);

  function openCreate() {
    resetForm();
    setCreateDrawerVisible(true);
  }

  function groupSelected(capabilities: CapabilityOption[]): boolean {
    return capabilities.every((item) => selected[item.value]);
  }

  function toggleGroup(capabilities: CapabilityOption[]) {
    const next = !groupSelected(capabilities);
    setSelected((current) => {
      const updated = { ...current };
      capabilities.forEach((item) => { updated[item.value] = next; });
      return updated;
    });
  }

  function capabilityChips(key: ApiKey): { id: string; label: string }[] {
    const chips: { id: string; label: string }[] = [];
    const owned = key.capabilities ?? [];
    for (const group of PLATFORM_API_KEY_CAPABILITY_GROUPS) {
      for (const item of group.capabilities) {
        if (owned.includes(item.value)) {
          chips.push({ id: item.value, label: t(item.labelKey) });
        }
      }
    }
    return chips;
  }

  async function createKey() {
    const capabilities = ALL_CAPABILITIES.filter((capability) => selected[capability]);
    if (!name.trim()) { setMessage(t('platformApiKeys.nameRequired')); return; }
    if (capabilities.length === 0) { setMessage(t('platformApiKeys.capabilityRequired')); return; }
    setCreating(true);
    setMessage(null);
    try {
      const created = await client.administration.apiKeys.create({ name: name.trim(), capabilities });
      setToken(created.token ?? created.api_key ?? '');
      setCreateDrawerVisible(false);
      setTokenVisible(true);
      setKeys((current) => [...current, created]);
    }
    catch (reason) { setMessage(reason instanceof Error ? reason.message : t('platformApiKeys.createFailed')); }
    finally { setCreating(false); }
  }

  async function revoke(key: ApiKey) {
    try { await client.administration.apiKeys.revoke(key.id); setKeys((current) => current.filter((item) => item.id !== key.id)); setMessage(t('platformApiKeys.deleteSuccess')); }
    catch (reason) { setMessage(reason instanceof Error ? reason.message : t('platformApiKeys.deleteFailed')); }
  }

  async function copyToken() {
    if (!token) return;
    try { await navigator.clipboard.writeText(token); setTokenVisible(false); }
    catch { /* Vue copyWithToast 的失败分支不关弹窗；React 侧同样保留。 */ }
  }

  return <section className="platform-api-keys">
    <header className="section-header">
      <h2>{t('platformApiKeys.title')}</h2>
      <p className="section-description">{t('platformApiKeys.description')}</p>
    </header>

    <Alert theme="warning" message={t('platformApiKeys.securityNotice')} className="security-alert" operation={<TButton size="small" variant="outline" icon={<TIcon name="add" />} onClick={openCreate}>{t('platformApiKeys.create')}</TButton>} />
    {message ? <p className="wk-api-key-message" role="status">{message}</p> : null}

    <section className="keys-section">
      {keys.length === 0 ? <div className="keys-state keys-state--empty">
        <span>{t('platformApiKeys.empty')}</span>
        <TButton size="small" variant="outline" icon={<TIcon name="add" />} onClick={openCreate}>{t('platformApiKeys.create')}</TButton>
      </div> : <div className="api-key-table-wrap"><table className="api-key-table">
        <thead><tr>
          <th>{t('platformApiKeys.name')}</th>
          <th>{t('platformApiKeys.key')}</th>
          <th>{t('platformApiKeys.capability')}</th>
          <th>{t('platformApiKeys.lastUsed')}</th>
          <th>{t('platformApiKeys.createdAt')}</th>
          <th className="api-key-table__actions-heading">{t('platformApiKeys.actions')}</th>
        </tr></thead>
        <tbody>{keys.map((key) => {
          const chips = capabilityChips(key);
          const visibleChips = chips.slice(0, VISIBLE_CAPABILITY_CHIP_COUNT);
          const hiddenCount = Math.max(0, chips.length - VISIBLE_CAPABILITY_CHIP_COUNT);
          return <tr key={key.id}>
            <td><span className="api-key-name">{key.name}</span></td>
            <td><code className="api-key-fingerprint">{key.api_key}</code></td>
            <td className="api-key-table__capability-cell">
              <div className="api-key-capability-inline">
                {visibleChips.map((chip) => <span className="api-key-capability-chip" key={chip.id}>{chip.label}</span>)}
                {hiddenCount > 0 ? <TPopup
                  trigger="click"
                  placement="bottom-left"
                  destroyOnClose
                  overlayClassName="platform-api-key-capability-popup-overlay"
                  content={<div className="api-key-capability-popup">
                    <div className="api-key-capability-popup__title">{t('platformApiKeys.capability')}</div>
                    {PLATFORM_API_KEY_CAPABILITY_GROUPS.map((group) => {
                      const labels = group.capabilities
                        .filter((item) => (key.capabilities ?? []).includes(item.value))
                        .map((item) => t(item.labelKey));
                      if (labels.length === 0) return null;
                      return <div className="api-key-capability-block" key={group.key}>
                        <div className="api-key-capability-block__title">{t(group.labelKey)}</div>
                        <div className="api-key-capability-block__chips">
                          {labels.map((label, index) => <span className="api-key-capability-chip" key={group.key + ':' + index}>{label}</span>)}
                        </div>
                      </div>;
                    })}
                  </div>}
                >
                  <button type="button" className="api-key-capability-chip api-key-capability-chip--more" aria-label={t('platformApiKeys.viewAllCapabilities')}>
                    {t('platformApiKeys.capabilityMore', { count: hiddenCount })}
                  </button>
                </TPopup> : null}
              </div>
            </td>
            <td><span className="api-key-meta">{formatDate(key.last_used_at, t('platformApiKeys.never'))}</span></td>
            <td><time className="api-key-date" dateTime={key.created_at}>{formatDate(key.created_at, t('platformApiKeys.never'))}</time></td>
            <td>
              <div className="api-key-table__actions">
                <TPopconfirm
                  content={t('platformApiKeys.deleteConfirm', { name: key.name })}
                  confirmBtn={{ content: t('common.delete'), theme: 'danger' }}
                  cancelBtn={{ content: t('common.cancel') }}
                  placement="bottom-right"
                  onConfirm={() => void revoke(key)}
                >
                  <TButton shape="square" variant="text" theme="danger" title={t('common.delete')} onClick={(event) => event.stopPropagation()}>
                    <TIcon name="delete" />
                  </TButton>
                </TPopconfirm>
              </div>
            </td>
          </tr>;
        })}</tbody>
      </table></div>}
    </section>

    {/* Vue SettingDrawer（PlatformAPIKeys.vue:129-144）——创建抽屉 */}
    <SettingDrawer
      visible={createDrawerVisible}
      drawerClass="api-key-create-drawer"
      title={t('platformApiKeys.create')}
      description={t('platformApiKeys.createDescription')}
      icon="secured"
      width="560px"
      minWidth={480}
      maxWidth={920}
      storageKey="setting-drawer:width:platform-api-key-create"
      closeOnOverlayClick={false}
      confirmText={t('platformApiKeys.create')}
      confirmLoading={creating}
      onVisibleChange={(visible) => { if (!visible) setCreateDrawerVisible(false); }}
      onConfirm={() => void createKey()}
    >
      <div className="api-key-dialog">
        <div className="api-key-dialog-row">
          <div className="api-key-dialog-row__label">
            <label>{t('platformApiKeys.name')}</label>
          </div>
          <TInput value={name} placeholder={t('platformApiKeys.namePlaceholder')} onChange={(value) => setName(String(value))} />
        </div>

        <div className="api-key-dialog-row">
          <div className="api-key-dialog-row__label">
            <label>{t('platformApiKeys.capability')}</label>
          </div>
          <p className="scope-hint">{t('platformApiKeys.capabilityHint')}</p>
          <div className="api-key-capability-list">
            {PLATFORM_API_KEY_CAPABILITY_GROUPS.map((group) => <div className="api-key-capability-group" key={group.key}>
              <div className="api-key-capability-group__header">
                <span>{t(group.labelKey)}</span>
                <TButton size="small" variant="text" onClick={() => toggleGroup(group.capabilities)}>
                  {groupSelected(group.capabilities) ? t('integrations.api.apiKeyCapabilityClearGroup') : t('integrations.api.apiKeyCapabilitySelectGroup')}
                </TButton>
              </div>
              <div className="api-key-capability-group__items">
                {group.capabilities.map((item) => <div className="api-key-capability-item" key={item.value}>
                  <TCheckbox checked={selected[item.value]} onChange={(checked) => setSelected((current) => ({ ...current, [item.value]: Boolean(checked) }))}>{t(item.labelKey)}</TCheckbox>
                  <p className="scope-hint">{t(item.hintKey)}</p>
                </div>)}
              </div>
            </div>)}
          </div>
        </div>
      </div>
    </SettingDrawer>

    {/* Vue t-dialog（PlatformAPIKeys.vue:194-204）——创建成功 token 弹层 */}
    <TDialog
      visible={tokenVisible}
      header={t('platformApiKeys.createdTitle')}
      confirmBtn={{ content: t('platformApiKeys.copy'), theme: 'primary' }}
      cancelBtn={null}
      closeOnOverlayClick={false}
      onConfirm={() => void copyToken()}
      onClose={() => setTokenVisible(false)}
    >
      <p>{t('platformApiKeys.createdDescription')}</p>
      <TTextarea readonly autosize value={token} />
    </TDialog>
  </section>;
}
