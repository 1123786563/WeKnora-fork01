import type { SystemAdminUser, SystemSetting, WeKnoraClient } from '@weknora/api-client';
import { Button as TButton, Switch as TSwitch } from 'tdesign-react';
// T12c：SystemSettings.vue 控制域平移——t-tabs/t-select/t-switch/
// t-input-number/t-tag-input/t-button/t-tag/t-loading + t-icon sprite
// glyph；样式平移至 settings.td.css §17（.system-settings scoped 块）。
// B4：创建用户/重置密码/优先级 hint 换 t-popup 同构（Vue CreateUserDialog.vue /
// ResetPasswordDialog.vue / SystemSettings.vue:38-52——锚定 popover，非居中 dialog）。
import { Button, Checkbox, Input, InputNumber, Loading, Popup, Select, Switch, Tag, TagInput, Tabs, Form } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { formatMessage } from '@weknora/i18n';
import { useSettingsLocale } from './PortedSectionsPanel.tsx';
import { PASSWORD_SPECIAL_CHARS } from '../auth/validation.ts';

/**
 * SystemGlobalSettingsPanel — React port of frontend/src/views/system/
 * SystemSettings.vue (platform-wide tunables for SystemAdmin). R491 agent I:
 * realigned to the Vue shape — section tabs with counts + descriptions,
 * per-key Vue labels/descriptions/badges, and the three high-risk Access
 * rows (system admins management, reset user password, create user).
 *
 * Like the Vue pane, every control auto-persists (switch/select commit on
 * change, inputs on blur) and high-risk keys inline-confirm before the PUT.
 */

type Section = 'access' | 'tenant' | 'runtime' | 'security' | 'other';

// Product-oriented order, mirroring SETTINGS_SECTION_KEYS in
// SystemSettings.vue. Unknown keys stay visible in a conditional
// "other" tab so backend diagnostics are preserved.
const SECTION_KEYS: Record<Exclude<Section, 'other'>, readonly string[]> = {
  access: ['auth.registration_mode', 'auth.complex_password_enabled', 'auth.default_tenant_mode', 'tenant.self_service_creation_enabled', 'tenant.max_owned_per_user'],
  tenant: ['tenant.default_storage_quota_gb', 'tenant.auto_create_api_key', 'tenant.auto_accept_invitation'],
  runtime: ['asynq.core_concurrency', 'asynq.enrichment_concurrency', 'asynq.postprocess_concurrency', 'asynq.maintenance_concurrency', 'asynq.shared_concurrency', 'asynq.wiki_concurrency', 'model.max_concurrency'],
  security: ['ssrf.whitelist', 'sandbox.docker_enabled'],
};

// Enum/bool keys whose change triggers a whole-value confirmation before
// the PUT (SystemSettings.vue HIGH_RISK_KEYS).
const HIGH_RISK_KEYS = new Set(['auth.registration_mode', 'sandbox.docker_enabled']);
// Rows that carry an extra "high risk" badge (HIGH_IMPACT_KEYS).
const HIGH_IMPACT_KEYS = new Set(['auth.registration_mode', 'tenant.auto_create_api_key', 'ssrf.whitelist', 'sandbox.docker_enabled']);
const SSRF_WHITELIST_KEY = 'ssrf.whitelist';

interface ConfirmState {
  title: string;
  body: string;
  confirmLabel: string;
  danger: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

/** Inline confirmation bubble mirroring the Vue t-popconfirm (alertdialog so
 * the existing SettingsPage.test contract keeps working). */
function ConfirmInline({ state, cancelLabel }: { state: ConfirmState; cancelLabel: string }) {
  return <div className={'wk-system-global-confirm' + (state.danger ? ' is-danger' : '')} role="alertdialog" aria-modal="true" aria-label={state.title}>
    <strong>{state.title}</strong>
    <p>{state.body}</p>
    <div>
      <button type="button" className="wk-system-global-confirm-btn" onClick={state.onConfirm}>{state.confirmLabel}</button>
      <button type="button" className="wk-system-global-confirm-cancel" onClick={state.onCancel}>{cancelLabel}</button>
    </div>
  </div>;
}

/** Email/tag input mirroring the Vue t-tag-input rows (admins + SSRF). */
/** T12c：手搓 TagInput 组件已删——控制域全部换 tdesign TagInput（Vue
 * t-tag-input 同构：break-line wrap、clearable、t-tag 芯片随库走）。 */

/** Vue t-popup hover hint (SystemSettings.vue:38-52) — info-circle trigger
 * whose bottom-start popover carries the priority tiers.（B4：React 保留实现
 * 的 in-tree 手写弹层已删，换 tdesign Popup + Vue hint-popover DOM 同构；
 * Vue placement="bottom-start"→react bottom-left，台账 #15。） */
function PriorityHint({ locale }: { locale: ReturnType<typeof useSettingsLocale> }) {
  const t = (key: string) => formatMessage(locale, key);
  return <Popup
    placement="bottom-left"
    trigger="hover"
    overlayInnerStyle={{ maxWidth: '420px' }}
    content={(
      <div className="hint-popover" data-testid="system-priority-popover">
        <p className="hint-popover__title">{t('system.globalSettings.priorityHint.disclosure')}</p>
        <ul className="hint-popover__list">
          <li>{t('system.globalSettings.priorityHint.tier1')}</li>
          <li>{t('system.globalSettings.priorityHint.tier2')}</li>
          <li>{t('system.globalSettings.priorityHint.tier3')}</li>
        </ul>
      </div>
    )}
  >
    <button type="button" className="hint-trigger" aria-label={t('system.globalSettings.priorityHint.disclosure')}>
      <TIcon name="info-circle" size="16px" />
    </button>
  </Popup>;
}

export function SystemGlobalSettingsPanel({ client, initialSettings }: { client: WeKnoraClient; initialSettings: SystemSetting[] }) {
  const locale = useSettingsLocale();
  const t = (key: string, values?: Record<string, string | number>) => formatMessage(locale, key, values);

  const [settings, setSettings] = useState<SystemSetting[]>(initialSettings);
  const [editValues, setEditValues] = useState<Record<string, unknown>>(() => Object.fromEntries(initialSettings.map((item) => [item.key, Array.isArray(item.value) ? [...(item.value as unknown[])] : item.value])));
  const [section, setSection] = useState<Section>('access');
  const [savingKey, setSavingKey] = useState<string | null>(null);
  const [savedKey, setSavedKey] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [messageTone, setMessageTone] = useState<'success' | 'error'>('success');
  const [announcement, setAnnouncement] = useState('');
  const [confirm, setConfirm] = useState<ConfirmState | null>(null);

  // ---- derived grouping ----------------------------------------------------
  const settingsByKey = useMemo(() => new Map(settings.map((item) => [item.key, item])), [settings]);
  const knownKeys = useMemo(() => new Set(Object.values(SECTION_KEYS).flat()), []);
  const unknownSettings = settings.filter((item) => !knownKeys.has(item.key));
  const hasUnknown = unknownSettings.length > 0;
  const sectionCount = (key: Exclude<Section, 'other'>) => SECTION_KEYS[key].filter((settingKey) => settingsByKey.has(settingKey)).length + (key === 'access' ? 2 : 0);
  const tabs: Section[] = [...(Object.keys(SECTION_KEYS) as Exclude<Section, 'other'>[]), ...(hasUnknown ? (['other'] as Section[]) : [])];
  const rows = section === 'other' ? unknownSettings : SECTION_KEYS[section].map((key) => settingsByKey.get(key)).filter((item): item is SystemSetting => Boolean(item));

  // ---- copy helpers --------------------------------------------------------
  // Like the Vue te()-guarded lookups, fall back to the raw key / backend
  // description when i18n has no entry (a misregistered deploy still renders).
  const keyLabel = (key: string) => {
    const path = `system.globalSettings.keyLabels.${key}`;
    const rendered = t(path);
    return rendered === path ? key : rendered;
  };
  const keyDescription = (item: SystemSetting) => {
    const path = `system.globalSettings.keyDescriptions.${item.key}`;
    const rendered = item.key === 'auth.complex_password_enabled' ? t(path, { specialChars: PASSWORD_SPECIAL_CHARS }) : t(path);
    return rendered === path ? (item.description ?? '') : rendered;
  };
  const enumLabel = (itemKey: string, optionValue: string) => {
    const path = `system.globalSettings.enumLabels.${itemKey}.${optionValue}`;
    const rendered = t(path);
    return rendered === path ? optionValue : rendered;
  };
  const hasOverride = (item: SystemSetting) => Boolean(item.last_modified_by && item.last_modified_by.trim() !== '');
  const modifiedMeta = (item: SystemSetting) => {
    if (!hasOverride(item)) return '';
    if (!item.updated_at || item.updated_at.startsWith('0001-')) return '';
    const actor = item.last_modified_by_name?.trim() ? item.last_modified_by_name : (item.last_modified_by || '').slice(0, 8);
    return t('system.globalSettings.modifiedAt', { value: `${formatDate(item.updated_at)} · ${actor}` });
  };
  const formatDate = (iso: string) => { try { return new Date(iso).toLocaleString('zh-CN', { hour12: false }); } catch { return iso; } };
  const minimumFor = (key: string) => key.startsWith('asynq.') && key.endsWith('_concurrency') ? 1 : 0;

  // ---- persistence (mirrors persistSetting/resetSetting in SystemSettings.vue)
  const applyUpdatedItem = (updated: SystemSetting) => {
    setSettings((current) => current.map((item) => (item.key === updated.key ? updated : item)));
    setEditValues((current) => ({ ...current, [updated.key]: Array.isArray(updated.value) ? [...(updated.value as unknown[])] : updated.value }));
    markSaved(updated.key);
  };
  const markSaved = (key: string) => {
    setSavedKey(key);
    setAnnouncement(t('system.globalSettings.saveAnnouncement', { label: keyLabel(key) }));
    window.setTimeout(() => setSavedKey((current) => (current === key ? null : current)), 2000);
  };
  const fail = (error: unknown, fallbackKey: string) => {
    const text = error instanceof Error && error.message ? error.message : t(fallbackKey);
    setMessage(text); setMessageTone('error'); setAnnouncement(text);
    return text;
  };
  async function persist(item: SystemSetting, value: unknown) {
    setSavingKey(item.key); setMessage(null);
    try {
      const updated = await client.administration.settings.update(item.key, value);
      applyUpdatedItem(updated);
      setMessage(t('system.globalSettings.messages.saveSuccess')); setMessageTone('success');
    } catch (error) {
      fail(error, 'system.globalSettings.messages.saveFailed');
      const canonical = settingsByKey.get(item.key);
      if (canonical) setEditValues((current) => ({ ...current, [item.key]: Array.isArray(canonical.value) ? [...(canonical.value as unknown[])] : canonical.value }));
    } finally { setSavingKey(null); }
  }
  async function refreshSettings() {
    try {
      const list = await client.administration.settings.list();
      setSettings(list);
      setEditValues(Object.fromEntries(list.map((item) => [item.key, Array.isArray(item.value) ? [...(item.value as unknown[])] : item.value])));
    } catch { /* reload races are surfaced by the original action's error */ }
  }
  const isDirty = (item: SystemSetting) => {
    const current = editValues[item.key];
    const original = item.value;
    if (Array.isArray(current) && Array.isArray(original)) return current.length !== original.length || current.some((value, index) => value !== original[index]);
    return current !== original;
  };

  // ---- high-risk confirm flows (onHighRiskSelectChange/onHighRiskBoolChange)
  const requestPersist = (item: SystemSetting, value: unknown) => {
    if (!HIGH_RISK_KEYS.has(item.key)) { void persist(item, value); return; }
    const body = item.key === 'sandbox.docker_enabled'
      ? t('system.globalSettings.confirm.bodySandboxDockerEnabled')
      : t('system.globalSettings.confirm.bodyAuthRegistrationMode', { label: keyLabel(item.key), value: Array.isArray(value) ? (value.length === 0 ? t('system.globalSettings.confirm.emptyValue') : value.join(', ')) : String(value) });
    setConfirm({
      title: t('system.globalSettings.confirm.confirmBtn'),
      body,
      confirmLabel: t('system.globalSettings.confirm.confirmBtn'),
      danger: true,
      onConfirm: () => { setConfirm(null); void persist(item, value); },
      onCancel: () => setConfirm(null),
    });
  };

  // ---- SSRF whitelist per-entry confirm (handleSSRFListChange) ------------
  const onSsrfTagsCommit = (next: string[]) => {
    const item = settingsByKey.get(SSRF_WHITELIST_KEY);
    if (!item) return;
    setEditValues((current) => ({ ...current, [SSRF_WHITELIST_KEY]: next }));
    const oldArr = Array.isArray(item.value) ? (item.value as string[]) : [];
    const oldSet = new Set(oldArr);
    const nextSet = new Set(next.filter((value) => value.trim()));
    const added = [...nextSet].filter((value) => !oldSet.has(value));
    const removed = [...oldSet].filter((value) => !nextSet.has(value));
    if (added.length === 0 && removed.length === 0) return;
    const finalSet = new Set(oldArr);
    const deltas: Array<{ action: 'add' | 'remove'; entry: string }> = [
      ...added.map((entry) => ({ action: 'add' as const, entry })),
      ...removed.map((entry) => ({ action: 'remove' as const, entry })),
    ];
    const askNext = (index: number) => {
      const delta = deltas[index];
      if (!delta) {
        const finalArr = Array.from(finalSet);
        const sameAsSaved = finalArr.length === oldArr.length && finalArr.every((value, i) => value === oldArr[i]);
        if (sameAsSaved) snapSsrfToSaved(item);
        else void persist(item, finalArr);
        return;
      }
      const base = `system.globalSettings.listConfirm.${SSRF_WHITELIST_KEY}.${delta.action}`;
      setConfirm({
        title: t(`${base}.header`),
        body: t(`${base}.body`, { entry: delta.entry }),
        confirmLabel: t(`${base}.confirmBtn`),
        danger: delta.action === 'add',
        onConfirm: () => {
          if (delta.action === 'add') finalSet.add(delta.entry); else finalSet.delete(delta.entry);
          setConfirm(null);
          askNext(index + 1);
        },
        onCancel: () => { setConfirm(null); snapSsrfToSaved(item); },
      });
    };
    askNext(0);
  };
  const snapSsrfToSaved = (item: SystemSetting) => {
    const saved = Array.isArray(item.value) ? (item.value as string[]) : [];
    setEditValues((current) => ({ ...current, [SSRF_WHITELIST_KEY]: [...saved] }));
  };

  // ---- reset-to-default / bulk apply (resetSetting/runBulkAction) ---------
  const resetSetting = (item: SystemSetting) => {
    setConfirm({
      title: t('system.globalSettings.reset.confirmBtn'),
      body: t('system.globalSettings.reset.confirmBody', { label: keyLabel(item.key) }),
      confirmLabel: t('system.globalSettings.reset.confirmBtn'),
      danger: false,
      onCancel: () => setConfirm(null),
      onConfirm: () => {
        setConfirm(null);
        setSavingKey(item.key); setMessage(null);
        void (async () => {
          try {
            await client.administration.settings.reset(item.key);
            await refreshSettings();
            markSaved(item.key);
            setMessage(t('system.globalSettings.reset.success')); setMessageTone('success');
          } catch (error) { fail(error, 'system.globalSettings.reset.failed'); }
          finally { setSavingKey(null); }
        })();
      },
    });
  };
  const runBulkAction = (item: SystemSetting) => {
    const valueText = item.value === null || item.value === undefined ? '' : String(item.value);
    setConfirm({
      title: t('system.globalSettings.bulkApply.confirmBtn'),
      body: t('system.globalSettings.bulkApply.confirmBody', { value: valueText }),
      confirmLabel: t('system.globalSettings.bulkApply.confirmBtn'),
      danger: false,
      onCancel: () => setConfirm(null),
      onConfirm: () => {
        setConfirm(null);
        setSavingKey(item.key); setMessage(null);
        void (async () => {
          try {
            const result = await client.administration.settings.applyDefaultStorageQuota();
            setMessage(t('system.globalSettings.bulkApply.success', { count: result.affected, gb: result.quotaGb })); setMessageTone('success');
            markSaved(item.key);
          } catch (error) { fail(error, 'system.globalSettings.bulkApply.failed'); }
          finally { setSavingKey(null); }
        })();
      },
    });
  };

  // ---- system admins management (loadAdmins/onAdminsChange) ---------------
  const [adminEmails, setAdminEmails] = useState<string[]>([]);
  const [adminEmailToId, setAdminEmailToId] = useState<Record<string, string>>({});
  const [adminBusy, setAdminBusy] = useState(false);
  const [currentUserId, setCurrentUserId] = useState<string | null>(null);
  const [profileResolved, setProfileResolved] = useState(false);
  const [resetPasswordVisible, setResetPasswordVisible] = useState(false);
  const [createUserVisible, setCreateUserVisible] = useState(false);

  const loadAdmins = async () => {
    try {
      const response = await client.administration.admins.list({ limit: 200 });
      const map: Record<string, string> = {};
      const emails: string[] = [];
      for (const admin of (response.items ?? []) as SystemAdminUser[]) {
        if (!admin.email) continue;
        map[admin.email] = admin.id;
        if (admin.id !== currentUserId) emails.push(admin.email);
      }
      setAdminEmailToId(map);
      setAdminEmails(emails);
    } catch (error) { fail(error, 'system.globalSettings.admins.loadFailed'); }
  };
  // Resolve the caller's id once (Vue reads it synchronously from the auth
  // store) so the admins tag list can hide the "you can't revoke yourself" row.
  useEffect(() => {
    void (async () => {
      try { const profile = await client.settings.profile.get(); setCurrentUserId(typeof (profile as { id?: unknown }).id === 'string' ? (profile as { id: string }).id : null); } catch { setCurrentUserId(null); }
      setProfileResolved(true);
    })();
  }, []);
  useEffect(() => { if (profileResolved) void loadAdmins(); }, [profileResolved, currentUserId]);

  const onAdminsCommit = (next: string[]) => {
    const authoritative = new Set(Object.entries(adminEmailToId).filter(([, id]) => id !== currentUserId).map(([email]) => email));
    const nextSet = new Set(next.map((value) => value.trim()).filter(Boolean));
    const added = [...nextSet].filter((email) => !authoritative.has(email));
    const removed = [...authoritative].filter((email) => !nextSet.has(email));
    if (added.length === 0 && removed.length === 0) return;
    const deltas: Array<{ action: 'promote' | 'revoke'; email: string }> = [
      ...added.map((email) => ({ action: 'promote' as const, email })),
      ...removed.map((email) => ({ action: 'revoke' as const, email })),
    ];
    const askNext = (index: number) => {
      const delta = deltas[index];
      if (!delta) {
        setAdminBusy(true);
        void (async () => {
          try {
            for (const { action, email } of deltas) {
              if (action === 'promote') await client.administration.admins.promote({ email });
              else {
                const userId = adminEmailToId[email];
                if (userId) await client.administration.admins.revoke(userId);
              }
            }
            await loadAdmins();
            setMessage(t('system.globalSettings.admins.saveSuccess')); setMessageTone('success');
            setAnnouncement(t('system.globalSettings.admins.saveSuccess'));
          } catch (error) {
            fail(error, 'system.globalSettings.admins.saveFailed');
            await loadAdmins();
          } finally { setAdminBusy(false); }
        })();
        return;
      }
      const base = `system.globalSettings.admins.confirm.${delta.action}`;
      setConfirm({
        title: t(`${base}.header`),
        body: t(`${base}.body`, { email: delta.email }),
        confirmLabel: t(`${base}.confirmBtn`),
        danger: delta.action === 'revoke',
        onConfirm: () => { setConfirm(null); askNext(index + 1); },
        onCancel: () => { setConfirm(null); void loadAdmins(); },
      });
    };
    askNext(0);
  };

  // ---- render（SystemSettings.vue 模板逐节点平移，类名/结构与 Vue 一致）----
  const cancelLabel = t('system.globalSettings.confirm.cancelBtn');
  // Input/InputNumber 的 @blur 提交语义（Vue :233）——onChange 只回写 editValues，
  // onBlur 才 persist；ref 记录最新草稿值防闭包过期。
  const draftRef = useRef<Record<string, unknown>>({});
  const sectionTabs = tabs.map((key) => ({ value: key, label: t(`system.globalSettings.sections.${key}.tab`, { count: key === 'other' ? unknownSettings.length : sectionCount(key as Exclude<Section, 'other'>) }) }));
  return <div className="system-settings" aria-label={t('system.globalSettings.title')}>
    <div className="section-header">
      <div className="section-header__titlewrap">
        <h2>{t('system.globalSettings.title')}</h2>
        <PriorityHint locale={locale} />
      </div>
      <p className="section-description">{t('system.globalSettings.description')}</p>
    </div>
    {/* 消息条/内联确认为 React 保留实现（Vue 用 MessagePlugin toast + t-popconfirm
        弹层，稳态扫描不可见）。 */}
    {message ? <p className={'wk-system-global-message' + (messageTone === 'success' ? ' is-success' : ' is-error')} role="status">{message}</p> : null}
    {confirm ? <ConfirmInline state={confirm} cancelLabel={cancelLabel} /> : null}
    {settings.length === 0 ? <div className="empty-state"><TIcon name="info-circle" size="24px" /><span>{t('system.globalSettings.empty')}</span></div> : <>
      <Tabs value={section} onChange={(value) => setSection(value as Section)} className="settings-section-tabs" list={sectionTabs} />
      <section className="settings-section-panel" aria-label={t(`system.globalSettings.sections.${section}.title`)}>
        <div className={`settings-section-intro${section === 'runtime' ? ' settings-section-intro--runtime' : ''}`}>
          <p>{t(`system.globalSettings.sections.${section}.description`)}</p>
          {section === 'runtime' ? <Tag theme="warning" variant="light" size="small">{t('system.globalSettings.sections.runtime.restartHint')}</Tag> : null}
        </div>
        {section === 'runtime' ? <div className="runtime-table-header" aria-hidden="true">
          <span>{t('system.globalSettings.runtimeTable.setting')}</span>
          <span>{t('system.globalSettings.runtimeTable.value')}</span>
        </div> : null}
        <div className={`settings-group${section === 'runtime' ? ' settings-group--runtime' : ''}`}>
          {section === 'access' ? <>
            <div className="setting-row setting-row--admin">
              <div className="setting-info">
                <div className="setting-label">
                  <span>{t('system.globalSettings.admins.label')}</span>
                  <Tag theme="danger" variant="light" size="small" className="setting-badge">{t('system.globalSettings.badgeHighRisk')}</Tag>
                </div>
                <p className="desc">{t('system.globalSettings.admins.description')}</p>
              </div>
              <div className="setting-control">
                <div className="setting-control-row">
                  <TagInput value={adminEmails} placeholder={t('system.globalSettings.admins.placeholder')} aria-label={t('system.globalSettings.admins.label')} disabled={adminBusy} className="setting-input setting-input--wide" clearable onChange={(value) => onAdminsCommit(value as string[])} />
                  {adminBusy ? <div className="setting-save-state" role="status"><Loading size="small" /><span>{t('system.globalSettings.saving')}</span></div> : null}
                </div>
              </div>
            </div>
            <div className="setting-row setting-row--password-reset">
              <div className="setting-info">
                <div className="setting-label">
                  <span>{t('system.globalSettings.passwordReset.label')}</span>
                  <Tag theme="danger" variant="light" size="small" className="setting-badge">{t('system.globalSettings.badgeHighRisk')}</Tag>
                </div>
                <p className="desc">{t('system.globalSettings.passwordReset.description')}</p>
              </div>
              <div className="setting-control">
                <ResetPasswordPopup client={client} visible={resetPasswordVisible} onVisibleChange={setResetPasswordVisible} onAnnounced={(text) => { setMessage(text); setMessageTone('success'); }} onFailed={(text) => { setMessage(text); setMessageTone('error'); }}>
                  <TButton theme="danger" variant="text" className="password-reset-trigger" icon={<TIcon name="lock-on" />}>{t('system.globalSettings.passwordReset.action')}</TButton>
                </ResetPasswordPopup>
              </div>
            </div>
            <div className="setting-row setting-row--create-user">
              <div className="setting-info">
                <div className="setting-label">
                  <span>{t('system.globalSettings.createUser.label')}</span>
                  <Tag theme="danger" variant="light" size="small" className="setting-badge">{t('system.globalSettings.badgeHighRisk')}</Tag>
                </div>
                <p className="desc">{t('system.globalSettings.createUser.description')}</p>
              </div>
              <div className="setting-control">
                <CreateUserPopup client={client} visible={createUserVisible} onVisibleChange={setCreateUserVisible} onAnnounced={(text) => { setMessage(text); setMessageTone('success'); setAnnouncement(text); }} onFailed={(text) => { setMessage(text); setMessageTone('error'); setAnnouncement(text); }}>
                  <TButton theme="primary" variant="text" className="create-user-trigger" icon={<TIcon name="user-add" />}>{t('system.globalSettings.createUser.action')}</TButton>
                </CreateUserPopup>
              </div>
            </div>
          </> : null}
          {rows.length === 0 ? <div className="empty-state"><TIcon name="info-circle" size="24px" /><span>{t('system.globalSettings.empty')}</span></div> : rows.map((item) => {
            const current = editValues[item.key];
            const enums = Array.isArray(item.enum) ? item.enum : [];
            const itemSaving = savingKey === item.key;
            const dirty = isDirty(item);
            return <div key={item.key} className="setting-row">
              <div className="setting-info">
                <div className="setting-label">
                  <span>{keyLabel(item.key)}</span>
                  {item.requires_restart ? <Tag theme="warning" variant="light" size="small" className="setting-badge">{t('system.globalSettings.badgeRequiresRestart')}</Tag> : null}
                  {item.is_secret ? <Tag theme="primary" variant="light" size="small" className="setting-badge">{t('system.globalSettings.badgeSecret')}</Tag> : null}
                  {HIGH_IMPACT_KEYS.has(item.key) ? <Tag theme="danger" variant="light" size="small" className="setting-badge">{t('system.globalSettings.badgeHighRisk')}</Tag> : null}
                  {hasOverride(item) ? <Tag theme="success" variant="light" size="small" className="setting-badge" title={t('system.globalSettings.badgeOverrideTooltip')}>{t('system.globalSettings.badgeOverride')}</Tag> : null}
                </div>
                {keyDescription(item) ? <p className="desc">{keyDescription(item)}</p> : null}
                {modifiedMeta(item) ? <div className="setting-meta">{t('system.globalSettings.modifiedAt', { value: modifiedMeta(item) })}</div> : null}
              </div>
              <div className="setting-control">
                <div className="setting-control-row">
                  {item.value_type === 'bool'
                    ? <TSwitch value={current === true} disabled={itemSaving} aria-label={keyLabel(item.key)} onChange={(value) => requestPersist(item, value)} />
                    : enums.length > 0
                      ? <Select className="setting-input" value={String(current ?? '')} disabled={itemSaving} aria-label={keyLabel(item.key)} options={enums.map((option) => ({ label: enumLabel(item.key, option), value: option }))} onChange={(value) => requestPersist(item, value)} />
                      : item.value_type === 'int'
                        ? <InputNumber className="setting-input" value={typeof current === 'number' ? current : Number(current ?? 0)} min={minimumFor(item.key)} disabled={itemSaving} aria-label={keyLabel(item.key)} theme="normal" step={1} placeholder={t('system.globalSettings.tagInputPlaceholder')} onChange={(value) => { draftRef.current[item.key] = value; setEditValues((state) => ({ ...state, [item.key]: value })); }} onBlur={(value) => { const parsed = value === '' || value === null || value === undefined ? null : Number(value); if (parsed !== null && !Number.isNaN(parsed)) void persist(item, parsed); }} />
                        : item.value_type === 'string_list'
                          ? <TagInput value={Array.isArray(current) ? (current as string[]) : []} placeholder={t('system.globalSettings.tagInputPlaceholder')} aria-label={keyLabel(item.key)} disabled={itemSaving} className="setting-input setting-input--wide" clearable onChange={(value) => { setEditValues((state) => ({ ...state, [item.key]: value })); onSsrfTagsCommit(value as string[]); }} />
                          : <Input className="setting-input" value={String(current ?? '')} disabled={itemSaving} aria-label={keyLabel(item.key)} clearable placeholder={t('system.globalSettings.tagInputPlaceholder')} onChange={(value) => { draftRef.current[item.key] = value; setEditValues((state) => ({ ...state, [item.key]: value })); }} onBlur={() => { const draft = draftRef.current[item.key]; void persist(item, typeof draft === 'string' ? draft : String(current ?? '')); }} />}
                  {itemSaving ? <div className="setting-save-state" role="status"><Loading size="small" /><span>{t('system.globalSettings.saving')}</span></div> : null}
                  {savedKey === item.key ? <div className="setting-save-state setting-save-state--success" role="status"><TIcon name="check-circle-filled" /><span>{t('system.globalSettings.saved')}</span></div> : null}
                </div>
                {hasOverride(item) || item.key === 'tenant.default_storage_quota_gb' ? <div className="setting-control-actions">
                  {item.key === 'tenant.default_storage_quota_gb' ? <TButton variant="text" size="small" className="setting-bulk-btn" icon={<TIcon name="usergroup" />} disabled={itemSaving || dirty} title={t('system.globalSettings.bulkApply.tooltip')} onClick={() => runBulkAction(item)}>{t('system.globalSettings.bulkApply.label')}</TButton> : null}
                  {hasOverride(item) ? <TButton variant="text" size="small" className="setting-reset-btn" icon={<TIcon name="refresh" />} disabled={itemSaving} title={t('system.globalSettings.reset.tooltip')} onClick={() => resetSetting(item)}>{t('system.globalSettings.reset.label')}</TButton> : null}
                </div> : null}
              </div>
            </div>;
          })}
        </div>
      </section>
    </>}
    <div className="wk-visually-hidden" role="status" aria-live="polite">{announcement}</div>
  </div>;
}

/** Vue newPasswordRules (frontend/src/utils/passwordPolicy.ts), message form. */
function passwordErrors(password: string, complexEnabled: boolean, locale: ReturnType<typeof useSettingsLocale>): string[] {
  const t = (key: string, values?: Record<string, string>) => formatMessage(locale, key, values);
  const errors: string[] = [];
  if (!password) errors.push(t('auth.passwordRequired'));
  else {
    if (password.length < 8 || password.length > 32) errors.push(t('system.globalSettings.createUser.validation.passwordLength'));
    if (!/[a-zA-Z]/.test(password)) errors.push(t('system.globalSettings.createUser.validation.passwordLetter'));
    if (!/\d/.test(password)) errors.push(t('system.globalSettings.createUser.validation.passwordNumber'));
    if (complexEnabled && !/[a-z]/.test(password)) errors.push(t('auth.passwordMustContainLowercaseLetter'));
    if (complexEnabled && !/[A-Z]/.test(password)) errors.push(t('auth.passwordMustContainUppercaseLetter'));
    if (complexEnabled && !/[!@#$%^&*()_+\-=[\]{}|;:,.<>?]/.test(password)) errors.push(t('auth.passwordMustContainSpecialChar', { specialChars: PASSWORD_SPECIAL_CHARS }));
  }
  return errors;
}

/** Vue ResetPasswordDialog.vue 逐节点平移（B4）：t-popup 锚定 popover，
 * default slot 包触发按钮；可见性由父组件持有。校验错误行为 React 保留
 * 实现（Vue t-form rules 内联错误，提交失败态不在扫描口径内）。 */
function ResetPasswordPopup({ client, visible, onVisibleChange, onAnnounced, onFailed, children }: {
  client: WeKnoraClient; visible: boolean; onVisibleChange: (next: boolean) => void; onAnnounced: (text: string) => void; onFailed: (text: string) => void; children: ReactNode;
}) {
  const locale = useSettingsLocale();
  const t = (key: string, values?: Record<string, string | number>) => formatMessage(locale, key, values);
  const [form, setForm] = useState({ email: '', newPassword: '', confirmPassword: '' });
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState<string[]>([]);
  const [complexEnabled, setComplexEnabled] = useState(false);
  const { email, newPassword, confirmPassword } = form;
  useEffect(() => {
    if (!visible) return;
    setForm({ email: '', newPassword: '', confirmPassword: '' }); setErrors([]);
    void client.auth.registrationConfig().then((config) => setComplexEnabled(Boolean(config?.complexPasswordEnabled))).catch(() => setComplexEnabled(false));
  }, [visible]);
  const submit = async () => {
    if (submitting) return;
    const problems: string[] = [];
    if (!email.trim()) problems.push(t('system.globalSettings.createUser.validation.emailRequired'));
    else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) problems.push(t('system.globalSettings.createUser.validation.emailInvalid'));
    problems.push(...passwordErrors(newPassword, complexEnabled, locale));
    if (!confirmPassword) problems.push(t('auth.confirmPasswordRequired'));
    else if (confirmPassword !== newPassword) problems.push(t('auth.passwordMismatch'));
    setErrors(problems);
    if (problems.length > 0) return;
    setSubmitting(true);
    try {
      await client.administration.admins.resetPassword({ email: email.trim(), new_password: newPassword });
      onAnnounced(t('system.globalSettings.passwordReset.success'));
      onVisibleChange(false);
    } catch (error) {
      onFailed(error instanceof Error && error.message ? error.message : t('system.globalSettings.passwordReset.failed'));
    } finally { setSubmitting(false); }
  };
  return <Popup
    visible={visible}
    trigger="click"
    placement="left-top"
    destroyOnClose
    overlayClassName="system-admin-action-popup-overlay"
    onVisibleChange={(next) => { if (!next && submitting) return; onVisibleChange(next); }}
    content={(
      <div className="system-admin-action-popup-inner" onClick={(event) => event.stopPropagation()}>
        <div className="system-admin-action-popup-title">{t('system.globalSettings.passwordReset.dialogTitle')}</div>
        <p className="system-admin-action-popup-hint">{t('system.globalSettings.passwordReset.warning')}</p>
        <Form
          labelAlign="top"
          className="system-admin-action-popup-form"
          initialData={form}
          onValuesChange={(_, allValues) => setForm((current) => ({ ...current, ...(allValues as Partial<typeof current>) }))}
        >
          <Form.FormItem label={t('system.globalSettings.passwordReset.emailLabel')} name="email" requiredMark>
            <Input type="text" clearable autocomplete="off" disabled={submitting} placeholder={t('system.globalSettings.passwordReset.emailPlaceholder')} />
          </Form.FormItem>
          <Form.FormItem label={t('system.globalSettings.passwordReset.newPasswordLabel')} name="newPassword" requiredMark>
            <Input type="password" autocomplete="new-password" disabled={submitting} placeholder={t('system.globalSettings.passwordReset.newPasswordPlaceholder')} prefixIcon={<TIcon name="lock-on" />} />
          </Form.FormItem>
          <Form.FormItem label={t('system.globalSettings.passwordReset.confirmPasswordLabel')} name="confirmPassword" requiredMark>
            <Input type="password" autocomplete="new-password" disabled={submitting} placeholder={t('system.globalSettings.passwordReset.confirmPasswordPlaceholder')} prefixIcon={<TIcon name="lock-on" />} onEnter={() => void submit()} />
          </Form.FormItem>
        </Form>
        {errors.length > 0 ? <div role="alert" className="wk-popup-errors">{errors.map((error) => <span key={error}>{error}</span>)}</div> : null}
        <div className="system-admin-action-popup-footer">
          <Button type="button" variant="outline" disabled={submitting} onClick={() => onVisibleChange(false)}>{t('system.globalSettings.confirm.cancelBtn')}</Button>
          <Button type="button" theme="danger" loading={submitting} onClick={() => void submit()}>{t('system.globalSettings.passwordReset.confirmBtn')}</Button>
        </div>
      </div>
    )}
  >
    <span className="system-admin-action-popup-anchor">{children}</span>
  </Popup>;
}

interface CreatedReveal { username: string; email: string; generatedPassword: string }

/** Vue CreateUserDialog.vue 逐节点平移（B4）：t-popup 锚定 popover（username/
 * email/自动生成开关），服务端签发一次性密码时切换 reveal 视图且未确认前
 * 禁止关闭（locked → destroyOnClose=false + onVisibleChange 拦截）。校验
 * 错误行为 React 保留实现（Vue t-form rules 内联错误，提交失败态不在
 * 扫描口径内）。 */
function CreateUserPopup({ client, visible, onVisibleChange, onAnnounced, onFailed, children }: {
  client: WeKnoraClient; visible: boolean; onVisibleChange: (next: boolean) => void; onAnnounced: (text: string) => void; onFailed: (text: string) => void; children: ReactNode;
}) {
  const locale = useSettingsLocale();
  const t = (key: string, values?: Record<string, string | number>) => formatMessage(locale, key, values);
  const [form, setForm] = useState({ username: '', email: '', autoGenerate: true, newPassword: '', confirmPassword: '' });
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState<string[]>([]);
  const [reveal, setReveal] = useState<CreatedReveal | null>(null);
  const [complexEnabled, setComplexEnabled] = useState(false);
  const { username, email, autoGenerate, newPassword, confirmPassword } = form;
  useEffect(() => {
    if (!visible) return;
    setForm({ username: '', email: '', autoGenerate: true, newPassword: '', confirmPassword: '' }); setErrors([]); setReveal(null);
    void client.auth.registrationConfig().then((config) => setComplexEnabled(Boolean(config?.complexPasswordEnabled))).catch(() => setComplexEnabled(false));
  }, [visible]);
  const locked = submitting || reveal !== null;
  const submit = async () => {
    if (submitting) return;
    const problems: string[] = [];
    if (!username.trim()) problems.push(t('system.globalSettings.createUser.validation.usernameRequired'));
    else if (username.trim().length < 2 || username.trim().length > 50) problems.push(t('system.globalSettings.createUser.validation.usernameLength'));
    if (!email.trim()) problems.push(t('system.globalSettings.createUser.validation.emailRequired'));
    else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) problems.push(t('system.globalSettings.createUser.validation.emailInvalid'));
    if (!autoGenerate) {
      problems.push(...passwordErrors(newPassword, complexEnabled, locale));
      if (!confirmPassword) problems.push(t('system.globalSettings.createUser.validation.confirmRequired'));
      else if (confirmPassword !== newPassword) problems.push(t('system.globalSettings.createUser.validation.passwordMismatch'));
    }
    setErrors(problems);
    if (problems.length > 0) return;
    setSubmitting(true);
    try {
      const result = await client.administration.admins.createUser({
        username: username.trim(),
        email: email.trim(),
        ...(autoGenerate ? {} : { password: newPassword }),
      });
      if (result.generatedPassword) {
        setReveal({ username: username.trim(), email: email.trim(), generatedPassword: result.generatedPassword });
      } else {
        onAnnounced(t('system.globalSettings.createUser.success'));
        onVisibleChange(false);
      }
    } catch (error) {
      onFailed(error instanceof Error && error.message ? error.message : t('system.globalSettings.createUser.failed'));
    } finally { setSubmitting(false); }
  };
  const copyDetails = async () => {
    if (!reveal) return;
    const text = `${reveal.username} / ${reveal.email} / ${reveal.generatedPassword}`;
    try { await navigator.clipboard?.writeText(text); onAnnounced(t('system.globalSettings.createUser.generated.copySuccess')); } catch { /* clipboard is best-effort */ }
  };
  return <Popup
    visible={visible}
    trigger="click"
    placement="left-top"
    destroyOnClose={!locked}
    overlayClassName="system-admin-action-popup-overlay"
    onVisibleChange={(next) => { if (!next && locked) return; onVisibleChange(next); }}
    content={(
      <div className="system-admin-action-popup-inner" onClick={(event) => event.stopPropagation()}>
        <div className="system-admin-action-popup-title">{reveal ? t('system.globalSettings.createUser.generated.successTitle') : t('system.globalSettings.createUser.dialogTitle')}</div>
        <p className="system-admin-action-popup-hint">{reveal ? t('system.globalSettings.createUser.generated.successBody') : t('system.globalSettings.createUser.warning')}</p>
        {reveal ? <>
          <div className="create-user-reveal">
            <div className="create-user-reveal-item">
              <span className="create-user-reveal-label">{t('system.globalSettings.createUser.generated.usernameLabel')}</span>
              <span className="create-user-reveal-value">{reveal.username}</span>
            </div>
            <div className="create-user-reveal-item">
              <span className="create-user-reveal-label">{t('system.globalSettings.createUser.generated.emailLabel')}</span>
              <span className="create-user-reveal-value">{reveal.email}</span>
            </div>
            <div className="create-user-reveal-item">
              <span className="create-user-reveal-label">{t('system.globalSettings.createUser.generated.passwordLabel')}</span>
              <pre className="create-user-reveal-value create-user-reveal-value--mono">{reveal.generatedPassword}</pre>
            </div>
          </div>
          <div className="system-admin-action-popup-footer">
            <Button type="button" theme="primary" variant="outline" onClick={() => void copyDetails()}>{t('system.globalSettings.createUser.generated.copyBtn')}</Button>
            <Button type="button" theme="primary" onClick={() => { setReveal(null); onVisibleChange(false); }}>{t('system.globalSettings.createUser.generated.acknowledgeBtn')}</Button>
          </div>
        </> : <>
          <Form
            labelAlign="top"
            className="system-admin-action-popup-form"
            initialData={form}
            onValuesChange={(_, allValues) => setForm((current) => ({ ...current, ...(allValues as Partial<typeof current>) }))}
          >
            <Form.FormItem label={t('system.globalSettings.createUser.usernameLabel')} name="username" requiredMark>
              <Input type="text" clearable autocomplete="off" disabled={submitting} placeholder={t('system.globalSettings.createUser.usernamePlaceholder')} />
            </Form.FormItem>
            <Form.FormItem label={t('system.globalSettings.createUser.emailLabel')} name="email" requiredMark>
              <Input type="text" clearable autocomplete="off" disabled={submitting} placeholder={t('system.globalSettings.createUser.emailPlaceholder')} />
            </Form.FormItem>
            <Form.FormItem name="autoGenerate">
              <Checkbox disabled={submitting}>{t('system.globalSettings.createUser.autoGenerateLabel')}</Checkbox>
            </Form.FormItem>
            {!autoGenerate ? <>
              <Form.FormItem label={t('system.globalSettings.createUser.newPasswordLabel')} name="newPassword" requiredMark>
                <Input type="password" autocomplete="new-password" disabled={submitting} placeholder={t('system.globalSettings.createUser.newPasswordPlaceholder')} prefixIcon={<TIcon name="lock-on" />} />
              </Form.FormItem>
              <Form.FormItem label={t('system.globalSettings.createUser.confirmPasswordLabel')} name="confirmPassword" requiredMark>
                <Input type="password" autocomplete="new-password" disabled={submitting} placeholder={t('system.globalSettings.createUser.confirmPasswordPlaceholder')} prefixIcon={<TIcon name="lock-on" />} onEnter={() => void submit()} />
              </Form.FormItem>
            </> : null}
          </Form>
          {errors.length > 0 ? <div role="alert" className="wk-popup-errors">{errors.map((error) => <span key={error}>{error}</span>)}</div> : null}
          <div className="system-admin-action-popup-footer">
            <Button type="button" variant="outline" disabled={submitting} onClick={() => onVisibleChange(false)}>{t('system.globalSettings.confirm.cancelBtn')}</Button>
            <Button type="button" theme="primary" loading={submitting} onClick={() => void submit()}>{t('system.globalSettings.createUser.confirmBtn')}</Button>
          </div>
        </>}
      </div>
    )}
  >
    <span className="system-admin-action-popup-anchor">{children}</span>
  </Popup>;
}
