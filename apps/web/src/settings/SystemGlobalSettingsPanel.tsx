import type { SystemAdminUser, SystemSetting, WeKnoraClient } from '@weknora/api-client';
import { Badge, Button, Card, Checkbox, Dialog, Input, NumberInput, Select, Status, Switch } from '@weknora/ui';
import { useEffect, useMemo, useState } from 'react';
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
  return <div className="wk-system-global-confirm rounded-lg border border-[rgba(178,106,8,0.35)] bg-[#fff8e6] px-4 py-3.5" role="alertdialog" aria-modal="true" aria-label={state.title}>
    <strong className={`text-sm font-semibold ${state.danger ? 'text-[#b23b34]' : 'text-[#1f2937]'}`}>{state.title}</strong>
    <p className="m-0 mt-1.5 mb-3 text-[13px] whitespace-pre-line text-[#5c6b83]">{state.body}</p>
    <div>
      <button type="button" className={`mr-2 cursor-pointer rounded-[4px] border px-3 py-[5px] text-white ${state.danger ? 'border-[#b23b34] bg-[#b23b34]' : 'border-[#b26a08] bg-[#b26a08]'}`} onClick={state.onConfirm}>{state.confirmLabel}</button>
      <button type="button" className="wk-system-global-confirm-cancel cursor-pointer rounded-[4px] border border-[rgba(120,135,155,0.35)] bg-transparent px-3 py-[5px]" onClick={state.onCancel}>{cancelLabel}</button>
    </div>
  </div>;
}

/** Email/tag input mirroring the Vue t-tag-input rows (admins + SSRF). */
function TagInput({ values, placeholder, ariaLabel, disabled, onCommit }: { values: string[]; placeholder: string; ariaLabel: string; disabled?: boolean; onCommit: (next: string[]) => void }) {
  const [draft, setDraft] = useState('');
  const commit = (next: string[]) => { setDraft(''); onCommit(next); };
  const addDraft = () => { const value = draft.trim(); if (!value) return; if (values.includes(value)) { setDraft(''); return; } commit([...values, value]); };
  return <div className={`wk-tag-input flex w-full max-w-[320px] flex-wrap items-center gap-1.5 rounded-[3px] border border-line-input bg-surface px-2 py-1 ${disabled ? 'opacity-60' : ''}`}>
    {values.map((value) => <span key={value} className="wk-tag-input-tag inline-flex items-center gap-1 rounded-pill bg-surface-wash px-2 py-px text-xs text-ink">
      {value}
      <button type="button" aria-label={`${ariaLabel} ${value}`} disabled={disabled} className="cursor-pointer border-0 bg-transparent p-0 text-xs text-muted hover:text-danger" onClick={() => commit(values.filter((item) => item !== value))}>×</button>
    </span>)}
    <input
      className="wk-tag-input-field min-w-[120px] flex-1 border-0 bg-transparent text-[13px] outline-none"
      value={draft}
      placeholder={placeholder}
      aria-label={ariaLabel}
      disabled={disabled}
      onChange={(event) => setDraft(event.target.value)}
      onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); addDraft(); } }}
      onBlur={addDraft}
    />
  </div>;
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

  // ---- render ---------------------------------------------------------------
  const cancelLabel = t('system.globalSettings.confirm.cancelBtn');
  return <section className="wk-system-global grid gap-4" aria-label={t('system.globalSettings.title')}>
    <header className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col">
      <div className="w-full">
        <h2 className="m-0 mb-2 text-[20px] font-semibold leading-[normal]">{t('system.globalSettings.title')}</h2>
        <p className="wk-muted text-muted m-0">{t('system.globalSettings.description')}</p>
        <details className="mt-1.5 text-[12px] text-muted">
          <summary className="wk-system-global-priority-summary cursor-pointer list-none text-muted hover:text-accent" aria-label={t('system.globalSettings.priorityHint.disclosure')} role="button">{t('system.globalSettings.priorityHint.disclosure')}</summary>
          <ul className="wk-system-global-priority-list m-1 mb-0 list-disc pl-4 text-[12px] leading-relaxed">
            <li>{t('system.globalSettings.priorityHint.tier1')}</li>
            <li>{t('system.globalSettings.priorityHint.tier2')}</li>
            <li>{t('system.globalSettings.priorityHint.tier3')}</li>
          </ul>
        </details>
      </div>
    </header>
    {message ? <p className={`wk-system-global-message m-0 text-[13px] ${messageTone === 'success' ? 'text-[#0a8f4c]' : 'text-[#b23b34]'}`} role="status">{message}</p> : null}
    {confirm ? <ConfirmInline state={confirm} cancelLabel={cancelLabel} /> : null}
    {settings.length === 0 ? <Card><Status>{t('system.globalSettings.empty')}</Status></Card> : <>
      <div className="wk-system-global-tabs flex overflow-x-auto gap-1 border-b border-[rgba(120,135,155,0.22)]" role="tablist" aria-label={t('system.globalSettings.title')}>
        {tabs.map((key) => <button key={key} type="button" role="tab" aria-selected={section === key} className={`cursor-pointer whitespace-nowrap border-0 border-b-2 bg-transparent px-3 py-[9px] font-[inherit] text-[#5c6b83] transition-colors ${section === key ? 'is-active border-b-[#0a8f4c] font-semibold text-[#0a8f4c]' : 'border-b-transparent'}`} onClick={() => setSection(key)}>{t(`system.globalSettings.sections.${key}.tab`, { count: key === 'other' ? unknownSettings.length : sectionCount(key) })}</button>)}
      </div>
      <div className="wk-system-global-section grid gap-3" aria-label={t(`system.globalSettings.sections.${section}.title`)}>
        <div className="wk-system-global-intro flex items-start justify-between gap-4 border-b border-line-soft pb-3 text-[13px] text-muted">
          <p className="m-0">{t(`system.globalSettings.sections.${section}.description`)}</p>
          {section === 'runtime' ? <Badge tone="warning">{t('system.globalSettings.sections.runtime.restartHint')}</Badge> : null}
        </div>
        {section === 'runtime' ? <div className="wk-system-global-runtime-header grid grid-cols-[minmax(0,1fr)_210px] gap-6 rounded-t-lg border border-line-soft border-b-0 bg-surface-alt px-4 py-2 text-xs font-medium text-muted" aria-hidden="true">
          <span>{t('system.globalSettings.runtimeTable.setting')}</span>
          <span className="text-right">{t('system.globalSettings.runtimeTable.value')}</span>
        </div> : null}
        <div className={`wk-system-global-rows grid gap-2.5 ${section === 'runtime' ? 'wk-system-global-rows--runtime' : ''}`}>
          {section === 'access' ? <>
            <div className="wk-system-global-row wk-system-global-row--admins grid grid-cols-[minmax(0,1fr)_minmax(280px,340px)] items-start gap-6 border-b border-line-soft pb-4">
              <div className="min-w-0">
                <div className="wk-system-global-label m-0 mb-1 flex flex-wrap items-center gap-1.5 text-[15px] font-medium text-ink">{t('system.globalSettings.admins.label')}<Badge tone="danger">{t('system.globalSettings.badgeHighRisk')}</Badge></div>
                <p className="wk-muted m-0 text-[13px] leading-normal text-muted">{t('system.globalSettings.admins.description')}</p>
              </div>
              <div className="flex flex-col items-end gap-2">
                <TagInput values={adminEmails} placeholder={t('system.globalSettings.admins.placeholder')} ariaLabel={t('system.globalSettings.admins.label')} disabled={adminBusy} onCommit={onAdminsCommit} />
                {adminBusy ? <span className="text-xs text-muted" role="status">{t('system.globalSettings.saving')}</span> : null}
              </div>
            </div>
            <div className="wk-system-global-row wk-system-global-row--password-reset grid grid-cols-[minmax(0,1fr)_minmax(280px,340px)] items-start gap-6 border-b border-line-soft pb-4">
              <div className="min-w-0">
                <div className="wk-system-global-label m-0 mb-1 flex flex-wrap items-center gap-1.5 text-[15px] font-medium text-ink">{t('system.globalSettings.passwordReset.label')}<Badge tone="danger">{t('system.globalSettings.badgeHighRisk')}</Badge></div>
                <p className="wk-muted m-0 text-[13px] leading-normal text-muted">{t('system.globalSettings.passwordReset.description')}</p>
              </div>
              <div className="flex justify-end">
                <button type="button" className="wk-password-reset-trigger cursor-pointer rounded-[6px] border border-transparent bg-[rgba(213,73,65,0.08)] px-3 py-1.5 text-[13px] text-danger hover:bg-[rgba(213,73,65,0.14)]" onClick={() => setResetPasswordVisible(true)}>{t('system.globalSettings.passwordReset.action')}</button>
              </div>
            </div>
            <div className="wk-system-global-row wk-system-global-row--create-user grid grid-cols-[minmax(0,1fr)_minmax(280px,340px)] items-start gap-6 border-b border-line-soft pb-4">
              <div className="min-w-0">
                <div className="wk-system-global-label m-0 mb-1 flex flex-wrap items-center gap-1.5 text-[15px] font-medium text-ink">{t('system.globalSettings.createUser.label')}<Badge tone="danger">{t('system.globalSettings.badgeHighRisk')}</Badge></div>
                <p className="wk-muted m-0 text-[13px] leading-normal text-muted">{t('system.globalSettings.createUser.description')}</p>
              </div>
              <div className="flex justify-end">
                <button type="button" className="wk-create-user-trigger cursor-pointer rounded-[6px] border border-accent bg-transparent px-3 py-1.5 text-[13px] text-accent hover:bg-accent/10" onClick={() => setCreateUserVisible(true)}>{t('system.globalSettings.createUser.action')}</button>
              </div>
            </div>
          </> : null}
          {rows.length === 0 ? <Card><Status>{t('system.globalSettings.empty')}</Status></Card> : rows.map((item) => {
            const current = editValues[item.key];
            const enums = Array.isArray(item.enum) ? item.enum : [];
            const itemSaving = savingKey === item.key;
            const dirty = isDirty(item);
            return <div key={item.key} className={`wk-system-global-row grid items-start gap-6 border-b border-line-soft pb-4 ${section === 'runtime' ? 'grid-cols-[minmax(0,1fr)_210px] bg-surface px-4' : 'grid-cols-[minmax(0,1fr)_minmax(280px,340px)]'}`}>
              <div className="wk-system-global-info min-w-0">
                <div className="wk-system-global-label m-0 mb-1 flex flex-wrap items-center gap-1.5 text-[15px] font-medium text-ink">
                  {keyLabel(item.key)}
                  {item.requires_restart ? <Badge tone="warning">{t('system.globalSettings.badgeRequiresRestart')}</Badge> : null}
                  {item.is_secret ? <Badge tone="primary">{t('system.globalSettings.badgeSecret')}</Badge> : null}
                  {HIGH_IMPACT_KEYS.has(item.key) ? <Badge tone="danger">{t('system.globalSettings.badgeHighRisk')}</Badge> : null}
                  {hasOverride(item) ? <Badge tone="success" title={t('system.globalSettings.badgeOverrideTooltip')}>{t('system.globalSettings.badgeOverride')}</Badge> : null}
                </div>
                {keyDescription(item) ? <p className="wk-muted m-0 max-w-[480px] text-[13px] leading-normal text-muted">{keyDescription(item)}</p> : null}
                {modifiedMeta(item) ? <p className="m-0 mt-1.5 text-xs text-muted/70">{modifiedMeta(item)}</p> : null}
              </div>
              <div className="wk-system-global-control flex flex-col items-end gap-2">
                <div className="flex w-full items-center justify-end gap-2">
                  {item.value_type === 'bool'
                    ? <Switch checked={current === true} disabled={itemSaving} aria-label={keyLabel(item.key)} onCheckedChange={(checked) => requestPersist(item, checked)} />
                    : enums.length > 0
                      ? <Select className="max-w-[240px]" value={String(current ?? '')} disabled={itemSaving} aria-label={keyLabel(item.key)} onChange={(event) => requestPersist(item, event.target.value)}>{enums.map((option) => <option key={option} value={option}>{enumLabel(item.key, option)}</option>)}</Select>
                      : item.value_type === 'int'
                        ? <NumberInput className="max-w-[210px]" value={typeof current === 'number' ? current : Number(current ?? 0)} min={minimumFor(item.key)} max={9999} disabled={itemSaving} aria-label={keyLabel(item.key)} onValueChange={(value) => setEditValues((state) => ({ ...state, [item.key]: value === '' ? '' : value }))} onBlur={(event) => { const parsed = event.target.value === '' ? null : Number(event.target.value); if (parsed !== null && !Number.isNaN(parsed)) void persist(item, parsed); }} />
                        : item.value_type === 'string_list'
                          ? <TagInput values={Array.isArray(current) ? (current as string[]) : []} placeholder={t('system.globalSettings.tagInputPlaceholder')} ariaLabel={keyLabel(item.key)} disabled={itemSaving} onCommit={onSsrfTagsCommit} />
                          : <Input className="max-w-[240px]" value={String(current ?? '')} disabled={itemSaving} aria-label={keyLabel(item.key)} onChange={(event) => setEditValues((state) => ({ ...state, [item.key]: event.target.value }))} onBlur={(event) => void persist(item, event.target.value)} />}
                  {itemSaving ? <span className="wk-system-global-saving inline-flex min-w-[52px] items-center gap-1 text-xs text-muted" role="status">{t('system.globalSettings.saving')}</span> : null}
                  {savedKey === item.key ? <span className="inline-flex min-w-[52px] items-center gap-1 text-xs text-[#0a8f4c]" role="status">{t('system.globalSettings.saved')}</span> : null}
                </div>
                {hasOverride(item) || item.key === 'tenant.default_storage_quota_gb' ? <div className="flex justify-end gap-2">
                  {item.key === 'tenant.default_storage_quota_gb' ? <button type="button" className="wk-system-global-bulk cursor-pointer border-0 bg-transparent p-0 text-xs text-accent disabled:cursor-not-allowed disabled:text-muted" disabled={itemSaving || dirty} title={t('system.globalSettings.bulkApply.tooltip')} onClick={() => runBulkAction(item)}>{t('system.globalSettings.bulkApply.label')}</button> : null}
                  {hasOverride(item) ? <button type="button" className="wk-system-global-reset cursor-pointer border-0 bg-transparent p-0 text-xs text-accent disabled:cursor-not-allowed disabled:text-muted" title={t('system.globalSettings.reset.tooltip')} onClick={() => resetSetting(item)}>{t('system.globalSettings.reset.label')}</button> : null}
                </div> : null}
              </div>
            </div>;
          })}
        </div>
      </div>
    </>}
    <ResetPasswordDialog client={client} open={resetPasswordVisible} onClose={() => setResetPasswordVisible(false)} onAnnounced={(text) => { setMessage(text); setMessageTone('success'); }} onFailed={(text) => { setMessage(text); setMessageTone('error'); }} />
    <CreateUserDialog client={client} open={createUserVisible} onClose={() => setCreateUserVisible(false)} onAnnounced={(text) => { setMessage(text); setMessageTone('success'); }} onFailed={(text) => { setMessage(text); setMessageTone('error'); }} />
    <div className="sr-only" role="status" aria-live="polite">{announcement}</div>
  </section>;
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

function ResetPasswordDialog({ client, open, onClose, onAnnounced, onFailed }: { client: WeKnoraClient; open: boolean; onClose: () => void; onAnnounced: (text: string) => void; onFailed: (text: string) => void }) {
  const locale = useSettingsLocale();
  const t = (key: string, values?: Record<string, string | number>) => formatMessage(locale, key, values);
  const [email, setEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState<string[]>([]);
  const [complexEnabled, setComplexEnabled] = useState(false);
  useEffect(() => {
    if (!open) return;
    setEmail(''); setNewPassword(''); setConfirmPassword(''); setErrors([]);
    void client.auth.registrationConfig().then((config) => setComplexEnabled(Boolean(config?.complexPasswordEnabled))).catch(() => setComplexEnabled(false));
  }, [open]);
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
      const success = t('system.globalSettings.passwordReset.success');
      onAnnounced(success);
      onClose();
    } catch (error) {
      onFailed(error instanceof Error && error.message ? error.message : t('system.globalSettings.passwordReset.failed'));
    } finally { setSubmitting(false); }
  };
  return <Dialog open={open} title={t('system.globalSettings.passwordReset.dialogTitle')} onClose={() => { if (!submitting) onClose(); }} className="wk-reset-password-dialog">
    <p className="wk-muted m-0 mt-1 mb-3 text-[13px] leading-normal text-muted">{t('system.globalSettings.passwordReset.warning')}</p>
    <div className="grid gap-3">
      <label className="grid gap-1 text-[13px] font-medium text-ink">{t('system.globalSettings.passwordReset.emailLabel')}
        <Input value={email} disabled={submitting} placeholder={t('system.globalSettings.passwordReset.emailPlaceholder')} onChange={(event) => setEmail(event.target.value)} />
      </label>
      <label className="grid gap-1 text-[13px] font-medium text-ink">{t('system.globalSettings.passwordReset.newPasswordLabel')}
        <Input type="password" value={newPassword} disabled={submitting} placeholder={t('system.globalSettings.passwordReset.newPasswordPlaceholder')} onChange={(event) => setNewPassword(event.target.value)} />
      </label>
      <label className="grid gap-1 text-[13px] font-medium text-ink">{t('system.globalSettings.passwordReset.confirmPasswordLabel')}
        <Input type="password" value={confirmPassword} disabled={submitting} placeholder={t('system.globalSettings.passwordReset.confirmPasswordPlaceholder')} onChange={(event) => setConfirmPassword(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void submit(); }} />
      </label>
      {errors.length > 0 ? <div role="alert" className="grid gap-0.5 text-[12px] text-danger">{errors.map((error) => <span key={error}>{error}</span>)}</div> : null}
      <div className="mt-1 flex justify-end gap-2">
        <Button type="button" disabled={submitting} onClick={onClose}>{t('system.globalSettings.confirm.cancelBtn')}</Button>
        <Button type="button" className="wk-reset-password-submit !bg-danger !text-white" disabled={submitting} onClick={() => void submit()}>{t('system.globalSettings.passwordReset.confirmBtn')}</Button>
      </div>
    </div>
  </Dialog>;
}

interface CreatedReveal { username: string; email: string; generatedPassword: string }

function CreateUserDialog({ client, open, onClose, onAnnounced, onFailed }: { client: WeKnoraClient; open: boolean; onClose: () => void; onAnnounced: (text: string) => void; onFailed: (text: string) => void }) {
  const locale = useSettingsLocale();
  const t = (key: string, values?: Record<string, string | number>) => formatMessage(locale, key, values);
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [autoGenerate, setAutoGenerate] = useState(true);
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState<string[]>([]);
  const [reveal, setReveal] = useState<CreatedReveal | null>(null);
  const [complexEnabled, setComplexEnabled] = useState(false);
  useEffect(() => {
    if (!open) return;
    setUsername(''); setEmail(''); setAutoGenerate(true); setNewPassword(''); setConfirmPassword(''); setErrors([]); setReveal(null);
    void client.auth.registrationConfig().then((config) => setComplexEnabled(Boolean(config?.complexPasswordEnabled))).catch(() => setComplexEnabled(false));
  }, [open]);
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
        onClose();
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
  return <Dialog open={open} title={reveal ? t('system.globalSettings.createUser.generated.successTitle') : t('system.globalSettings.createUser.dialogTitle')} onClose={() => { if (!locked) onClose(); }} className="wk-create-user-dialog">
    <p className="wk-muted m-0 mt-1 mb-3 text-[13px] leading-normal text-muted">{reveal ? t('system.globalSettings.createUser.generated.successBody') : t('system.globalSettings.createUser.warning')}</p>
    {reveal ? <>
      <dl className="wk-create-user-reveal grid gap-2 border border-line-soft rounded-lg bg-surface-alt p-3 text-[13px]">
        <div className="grid grid-cols-[auto_minmax(0,1fr)] gap-3"><dt className="font-medium text-muted">{t('system.globalSettings.createUser.generated.usernameLabel')}</dt><dd className="m-0 break-all">{reveal.username}</dd></div>
        <div className="grid grid-cols-[auto_minmax(0,1fr)] gap-3"><dt className="font-medium text-muted">{t('system.globalSettings.createUser.generated.emailLabel')}</dt><dd className="m-0 break-all">{reveal.email}</dd></div>
        <div className="grid grid-cols-[auto_minmax(0,1fr)] gap-3"><dt className="font-medium text-muted">{t('system.globalSettings.createUser.generated.passwordLabel')}</dt><dd className="m-0 break-all font-mono">{reveal.generatedPassword}</dd></div>
      </dl>
      <div className="mt-3 flex justify-end gap-2">
        <Button type="button" onClick={() => void copyDetails()}>{t('system.globalSettings.createUser.generated.copyBtn')}</Button>
        <Button type="button" className="wk-create-user-acknowledge !bg-accent !text-white" onClick={() => { setReveal(null); onClose(); }}>{t('system.globalSettings.createUser.generated.acknowledgeBtn')}</Button>
      </div>
    </> : <div className="grid gap-3">
      <label className="grid gap-1 text-[13px] font-medium text-ink">{t('system.globalSettings.createUser.usernameLabel')}
        <Input value={username} disabled={submitting} placeholder={t('system.globalSettings.createUser.usernamePlaceholder')} onChange={(event) => setUsername(event.target.value)} />
      </label>
      <label className="grid gap-1 text-[13px] font-medium text-ink">{t('system.globalSettings.createUser.emailLabel')}
        <Input value={email} disabled={submitting} placeholder={t('system.globalSettings.createUser.emailPlaceholder')} onChange={(event) => setEmail(event.target.value)} />
      </label>
      <label className="flex items-center gap-2 text-[13px] text-ink">
        <Checkbox checked={autoGenerate} disabled={submitting} onChange={(event) => setAutoGenerate(event.target.checked)} />
        {t('system.globalSettings.createUser.autoGenerateLabel')}
      </label>
      {!autoGenerate ? <>
        <label className="grid gap-1 text-[13px] font-medium text-ink">{t('system.globalSettings.createUser.newPasswordLabel')}
          <Input type="password" value={newPassword} disabled={submitting} placeholder={t('system.globalSettings.createUser.newPasswordPlaceholder')} onChange={(event) => setNewPassword(event.target.value)} />
        </label>
        <label className="grid gap-1 text-[13px] font-medium text-ink">{t('system.globalSettings.createUser.confirmPasswordLabel')}
          <Input type="password" value={confirmPassword} disabled={submitting} placeholder={t('system.globalSettings.createUser.confirmPasswordPlaceholder')} onChange={(event) => setConfirmPassword(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void submit(); }} />
        </label>
      </> : null}
      {errors.length > 0 ? <div role="alert" className="grid gap-0.5 text-[12px] text-danger">{errors.map((error) => <span key={error}>{error}</span>)}</div> : null}
      <div className="mt-1 flex justify-end gap-2">
        <Button type="button" disabled={submitting} onClick={onClose}>{t('system.globalSettings.confirm.cancelBtn')}</Button>
        <Button type="button" className="wk-create-user-submit !bg-accent !text-white" disabled={submitting} onClick={() => void submit()}>{t('system.globalSettings.createUser.confirmBtn')}</Button>
      </div>
    </div>}
  </Dialog>;
}
