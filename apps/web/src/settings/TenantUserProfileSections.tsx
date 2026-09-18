import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Input, Status, Textarea } from '@weknora/ui';
import { formatMessage, type Locale } from '@weknora/i18n';
import { profilePasswordPatch, tenantPatch } from './surface.ts';

// ---------------------------------------------------------------------------
// Localized tenant + userprofile sections (item A5). Ported from
// frontend/src/views/settings/TenantInfo.vue (read-only setting rows with
// owner-only inline name/description editing, template lines 24-186) and
// frontend/src/views/settings/UserProfile.vue (account rows plus the inline
// change-password form, template lines 24-167).

function formatDateTime(value: unknown, locale: Locale): string {
  if (typeof value !== 'string' || !value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return new Intl.DateTimeFormat(locale || 'zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date);
}

function text(value: unknown): string { return typeof value === 'string' ? value : ''; }

// TenantInfo.vue getStatusText / getStatusTheme (lines 607-630).
function tenantStatus(status: string): { key: string; tone: string } {
  if (status === 'active') return { key: 'tenant.statusActive', tone: 'success' };
  if (status === 'inactive') return { key: 'tenant.statusInactive', tone: 'warning' };
  if (status === 'suspended') return { key: 'tenant.statusSuspended', tone: 'danger' };
  return { key: 'tenant.statusUnknown', tone: 'default' };
}

export function TenantInfoSection({ client, tenantId, role, locale, payload }: {
  client: WeKnoraClient;
  tenantId: number;
  role: string;
  locale: Locale;
  payload: unknown;
}) {
  const t = (key: string) => formatMessage(locale, key);
  const info = payload !== null && typeof payload === 'object' && !Array.isArray(payload) ? payload as Record<string, unknown> : {};
  const canEditTenant = role === 'owner' || role === 'admin' || role === 'system-admin';
  const [editingName, setEditingName] = useState(false);
  const [nameDraft, setNameDraft] = useState('');
  const [editingDescription, setEditingDescription] = useState(false);
  const [descriptionDraft, setDescriptionDraft] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const currentName = text(info.name);
  const currentDescription = text(info.description);

  function startEditName() { setError(null); setNotice(null); setNameDraft(currentName); setEditingName(true); }
  function startEditDescription() { setError(null); setNotice(null); setDescriptionDraft(currentDescription); setEditingDescription(true); }
  function cancelEdits() { setEditingName(false); setEditingDescription(false); setError(null); }

  async function saveName(reload: () => void) {
    if (saving) return;
    setSaving(true); setError(null); setNotice(null);
    try {
      const patch = tenantPatch(nameDraft, currentDescription);
      await client.settings.tenant.update(tenantId, patch);
      setNotice(t('tenant.details.editNameConfirm'));
      setEditingName(false);
      reload();
    } catch (reason) {
      setError(reason instanceof Error && reason.message ? reason.message : t('tenant.messages.fetchFailed'));
    } finally { setSaving(false); }
  }

  async function saveDescription(reload: () => void) {
    if (saving) return;
    setSaving(true); setError(null); setNotice(null);
    try {
      const patch = tenantPatch(currentName || ' ', descriptionDraft);
      await client.settings.tenant.update(tenantId, patch);
      setEditingDescription(false);
      reload();
    } catch (reason) {
      setError(reason instanceof Error && reason.message ? reason.message : t('tenant.messages.fetchFailed'));
    } finally { setSaving(false); }
  }

  const status = tenantStatus(text(info.status));
  const hasQuota = info.storage_quota !== undefined && info.storage_quota !== null;
  const quota = Number(info.storage_quota);
  const used = Number(info.storage_used ?? 0);
  const fmtBytes = (bytes: number): string => {
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const index = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
    // Vue TenantInfo formatBytes: parseFloat(toFixed(2)) drops trailing zeros
    // (10 GB, not 10.0 GB).
    return parseFloat((bytes / Math.pow(1024, index)).toFixed(2)) + ' ' + units[index];
  };
  const usage = hasQuota && quota > 0 ? Math.min(100, Math.round((used / quota) * 100)) : 0;

  return (
    <div className="tenant-info" data-testid="tenant-info-section">
      {error ? <Status tone="error">{error}</Status> : null}
      <div className="settings-group">
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('tenant.details.idLabel')}</label>
            <p className="desc">{t('tenant.details.idDescription')}</p>
          </div>
          <div className="setting-control">
            <span className="info-value">{typeof info.id === 'number' || typeof info.id === 'string' ? String(info.id) : '-'}</span>
          </div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('tenant.details.nameLabel')}</label>
            <p className="desc">{t('tenant.details.nameDescription')}</p>
          </div>
          <div className="setting-control">
            {editingName ? (
              <div className="inline-edit">
                <Input
                  className="box-border border border-[#cbd5e1] rounded-control bg-white text-ink [font:inherit] px-[.65rem] py-[.55rem]"
                  autoFocus
                  maxLength={64}
                  aria-label={t('tenant.details.nameLabel')}
                  placeholder={t('tenant.details.editNamePlaceholder')}
                  disabled={saving}
                  value={nameDraft}
                  onChange={(event) => setNameDraft(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') { event.preventDefault(); void saveName(() => window.location.reload()); }
                    if (event.key === 'Escape') cancelEdits();
                  }}
                />
                <Button type="button" disabled={saving || !nameDraft.trim()} onClick={() => void saveName(() => window.location.reload())}>{t('tenant.details.editNameConfirm')}</Button>
                <Button type="button" disabled={saving} onClick={cancelEdits}>{t('tenant.details.editNameCancel')}</Button>
              </div>
            ) : (
              <>
                <span className="info-value">{currentName || '-'}</span>
                {canEditTenant ? (
                  <button type="button" className="edit-btn" aria-label={t('tenant.details.editName')} title={t('tenant.details.editName')} onClick={startEditName}>{/* t-icon "edit" counterpart */}<svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M17 3a2.85 2.85 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" /></svg></button>
                ) : null}
              </>
            )}
          </div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('tenant.details.descriptionLabel')}</label>
            <p className="desc">{t('tenant.details.descriptionDescription')}</p>
          </div>
          <div className="setting-control">
            {editingDescription ? (
              <div className="inline-edit inline-edit-description">
                <Textarea
                  className="box-border border border-[#cbd5e1] rounded-control bg-white text-ink [font:inherit] px-[.65rem] py-[.55rem]"
                  autoFocus
                  rows={2}
                  maxLength={512}
                  aria-label={t('tenant.details.descriptionLabel')}
                  placeholder={t('tenant.details.editDescriptionPlaceholder')}
                  disabled={saving}
                  value={descriptionDraft}
                  onChange={(event) => setDescriptionDraft(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Escape') cancelEdits();
                    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) { event.preventDefault(); void saveDescription(() => window.location.reload()); }
                  }}
                />
                <div className="inline-edit-actions">
                  <Button type="button" disabled={saving} onClick={() => void saveDescription(() => window.location.reload())}>{t('tenant.details.editNameConfirm')}</Button>
                  <Button type="button" disabled={saving} onClick={cancelEdits}>{t('tenant.details.editNameCancel')}</Button>
                </div>
              </div>
            ) : (
              <>
                <span className="info-value description-value">{currentDescription || t('tenant.details.descriptionEmptyPlaceholder')}</span>
                {canEditTenant ? (
                  <button type="button" className="edit-btn" aria-label={t('tenant.details.editDescription')} title={t('tenant.details.editDescription')} onClick={startEditDescription}>{/* t-icon "edit" counterpart */}<svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M17 3a2.85 2.85 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" /></svg></button>
                ) : null}
              </>
            )}
          </div>
        </div>
        {info.business ? (
          <div className="setting-row">
            <div className="setting-info">
              <label>{t('tenant.details.businessLabel')}</label>
              <p className="desc">{t('tenant.details.businessDescription')}</p>
            </div>
            <div className="setting-control"><span className="info-value">{text(info.business)}</span></div>
          </div>
        ) : null}
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('tenant.details.statusLabel')}</label>
            <p className="desc">{t('tenant.details.statusDescription')}</p>
          </div>
          <div className="setting-control"><span className={'wk-tag wk-tag--' + status.tone + ' inline-flex items-center shrink-0 py-[1px]! px-[8px]! leading-[1.6]' + (status.tone === 'warning' ? ' text-[#b45309]! bg-[#fffaeb]! border border-solid border-[#fedf89]' : '')}>{t(status.key)}</span></div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('tenant.details.createdAtLabel')}</label>
            <p className="desc">{t('tenant.details.createdAtDescription')}</p>
          </div>
          <div className="setting-control"><span className="info-value">{formatDateTime(info.created_at, locale)}</span></div>
        </div>
        {hasQuota ? (
          <>
            <div className="setting-row">
              <div className="setting-info">
                <label>{t('tenant.storage.quotaLabel')}</label>
                <p className="desc">{t('tenant.storage.quotaDescription')}</p>
              </div>
              <div className="setting-control"><span className="info-value">{fmtBytes(quota)}</span></div>
            </div>
            <div className="setting-row">
              <div className="setting-info">
                <label>{t('tenant.storage.usedLabel')}</label>
                <p className="desc">{t('tenant.storage.usedDescription')}</p>
              </div>
              <div className="setting-control"><span className="info-value">{fmtBytes(used)}</span></div>
            </div>
            <div className="setting-row">
              <div className="setting-info">
                <label>{t('tenant.storage.usageLabel')}</label>
                <p className="desc">{t('tenant.storage.usageDescription')}</p>
              </div>
              <div className="setting-control"><span className="info-value">{usage}%</span></div>
            </div>
          </>
        ) : null}
      </div>
      {notice ? <Status tone="success">{notice}</Status> : null}
    </div>
  );
}

// UserProfile.vue password rules: the shared auth.* message keys mirror the
// newPasswordRules table (frontend/src/utils/passwordPolicy.ts).
const PASSWORD_SPECIAL_CHARS = '!@#$%^&*()_+-=[]{}|;:,.<>?';

function newPasswordError(value: string, complexEnabled: boolean): string | null {
  if (!value) return 'auth.passwordRequired';
  if (value.length < 8) return 'auth.passwordMinLength';
  if (value.length > 32) return 'auth.passwordMaxLength';
  const hasLower = value.split('').some((ch) => ch >= 'a' && ch <= 'z');
  const hasUpper = value.split('').some((ch) => ch >= 'A' && ch <= 'Z');
  const hasDigit = value.split('').some((ch) => ch >= '0' && ch <= '9');
  const hasSpecial = value.split('').some((ch) => PASSWORD_SPECIAL_CHARS.includes(ch));
  if (complexEnabled) {
    if (!hasLower) return 'auth.passwordMustContainLowercaseLetter';
    if (!hasUpper) return 'auth.passwordMustContainUppercaseLetter';
    if (!hasDigit) return 'auth.passwordMustContainNumber';
    if (!hasSpecial) return 'auth.passwordMustContainSpecialChar';
  } else {
    if (!hasLower && !hasUpper) return 'auth.passwordMustContainLetter';
    if (!hasDigit) return 'auth.passwordMustContainNumber';
  }
  return null;
}

export function UserProfileSection({ client, locale, payload }: {
  client: WeKnoraClient;
  locale: Locale;
  payload: unknown;
}) {
  const t = (key: string, values?: Record<string, string | number>) => formatMessage(locale, key, values);
  const info = payload !== null && typeof payload === 'object' && !Array.isArray(payload) ? payload as Record<string, unknown> : {};
  const [complexPasswordEnabled, setComplexPasswordEnabled] = useState(false);
  // Vue UserProfile: the change-password form lives in a click popup off the
  // masked row's edit button, not inline below the settings rows.
  const [passwordPopupOpen, setPasswordPopupOpen] = useState(false);
  const [form, setForm] = useState({ oldPassword: '', newPassword: '', confirmation: '' });
  const [fieldErrors, setFieldErrors] = useState<Record<string, string | null>>({});
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    void client.auth.registrationConfig()
      .then((config) => setComplexPasswordEnabled(config.complexPasswordEnabled === true))
      .catch(() => setComplexPasswordEnabled(false));
  }, [client]);

  type Field = 'oldPassword' | 'newPassword' | 'confirmation';
  function validateField(field: Field, value: string, current: typeof form): string | null {
    if (field === 'oldPassword') return value ? null : t('userProfile.changePassword.currentRequired');
    if (field === 'newPassword') {
      const policy = newPasswordError(value, complexPasswordEnabled);
      if (policy) return t(policy);
      if (value && value === current.oldPassword) return t('userProfile.changePassword.sameAsCurrent');
      return null;
    }
    if (!value) return t('auth.confirmPasswordRequired');
    if (value !== current.newPassword) return t('auth.passwordMismatch');
    return null;
  }

  function blurField(field: Field) {
    setFieldErrors((current) => ({ ...current, [field]: validateField(field, form[field], form) }));
  }

  function update(field: Field, value: string) {
    setForm((current) => {
      const next = { ...current, [field]: value };
      setFieldErrors((errors) => ({
        ...errors,
        [field]: errors[field] ? null : errors[field],
        ...(field === 'newPassword' && errors.confirmation ? { confirmation: null } : {}),
        ...(field === 'oldPassword' && errors.newPassword && next.newPassword !== errors.newPassword ? { newPassword: null } : {}),
      }));
      return next;
    });
  }

  function resetForm() {
    setForm({ oldPassword: '', newPassword: '', confirmation: '' });
    setFieldErrors({});
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;
    const errors = {
      oldPassword: validateField('oldPassword', form.oldPassword, form),
      newPassword: validateField('newPassword', form.newPassword, form),
      confirmation: validateField('confirmation', form.confirmation, form),
    };
    setFieldErrors(errors);
    if (errors.oldPassword || errors.newPassword || errors.confirmation) return;
    setSubmitting(true); setError(null); setNotice(null);
    try {
      const patch = profilePasswordPatch(form.oldPassword, form.newPassword, form.confirmation, { complexPasswordEnabled });
      await client.settings.profile.changePassword(patch);
      setNotice(t('userProfile.changePassword.success'));
      resetForm();
    } catch (reason) {
      setError(reason instanceof Error && reason.message ? reason.message : t('userProfile.changePassword.failed'));
    } finally { setSubmitting(false); }
  }

  return (
    <div className="user-profile" data-testid="user-profile-section">
      {error ? <Status tone="error">{error}</Status> : null}
      <div className="settings-group">
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('tenant.api.userIdLabel')}</label>
            <p className="desc">{t('tenant.api.userIdDescription')}</p>
          </div>
          <div className="setting-control"><span className="info-value">{text(info.id) || '-'}</span></div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('tenant.api.usernameLabel')}</label>
            <p className="desc">{t('tenant.api.usernameDescription')}</p>
          </div>
          <div className="setting-control"><span className="info-value">{text(info.username) || '-'}</span></div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('tenant.api.emailLabel')}</label>
            <p className="desc">{t('tenant.api.emailDescription')}</p>
          </div>
          <div className="setting-control"><span className="info-value">{text(info.email) || '-'}</span></div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('tenant.api.createdAtLabel')}</label>
            <p className="desc">{t('tenant.api.createdAtDescription')}</p>
          </div>
          <div className="setting-control"><span className="info-value">{formatDateTime(info.created_at, locale)}</span></div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('userProfile.changePassword.label')}</label>
            <p className="desc">{t('userProfile.changePassword.description')}</p>
          </div>
          <div className="setting-control flex items-center gap-2">
            <span className="info-value password-mask" aria-hidden="true">••••••••</span>
            <button type="button" className="edit-btn inline-flex h-6 w-6 cursor-pointer items-center justify-center rounded-[5px] border-0 bg-transparent p-0 text-[#87909d] hover:bg-[#f3f3f3] hover:text-[rgba(0,0,0,0.9)]" aria-label={t('userProfile.changePassword.label')} title={t('userProfile.changePassword.label')} aria-expanded={passwordPopupOpen} onClick={() => setPasswordPopupOpen((open) => !open)}>
              {/* t-icon "edit" counterpart */}
              <svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M17 3a2.85 2.85 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" /></svg>
            </button>
          </div>
        </div>
      </div>
      {passwordPopupOpen ? (
      <div className="user-profile-password-popup relative ml-auto w-full max-w-[360px] rounded-[10px] border border-[#e7e7ea] bg-white p-4 shadow-[0_6px_24px_rgba(15,23,42,0.12)]">
        <div className="text-[14px] font-semibold text-ink">{t('userProfile.changePassword.label')}</div>
        <p className="m-0 mb-2 mt-1 text-[12px] text-muted-strong">{t('userProfile.changePassword.description')}</p>
      <form className="grid gap-3" onSubmit={(event) => void submit(event)}>
        <label className="grid gap-[.35rem] text-[#27364d] font-semibold">
          {t('userProfile.changePassword.currentLabel')}
          <Input
            className="w-full box-border border border-[#cbd5e1] rounded-control bg-white text-ink [font:inherit] px-[.65rem] py-[.55rem]"
            type="password"
            autoComplete="current-password"
            placeholder={t('userProfile.changePassword.currentPlaceholder')}
            value={form.oldPassword}
            onChange={(event) => update('oldPassword', event.target.value)}
            onBlur={() => blurField('oldPassword')}
          />
          {fieldErrors.oldPassword ? <span className="wk-field-error text-xs leading-[1.4] text-[#c23434]">{fieldErrors.oldPassword}</span> : null}
        </label>
        <label className="grid gap-[.35rem] text-[#27364d] font-semibold">
          {t('userProfile.changePassword.newLabel')}
          <Input
            className="w-full box-border border border-[#cbd5e1] rounded-control bg-white text-ink [font:inherit] px-[.65rem] py-[.55rem]"
            type="password"
            autoComplete="new-password"
            placeholder={t('userProfile.changePassword.newPlaceholder')}
            value={form.newPassword}
            onChange={(event) => update('newPassword', event.target.value)}
            onBlur={() => blurField('newPassword')}
          />
          {fieldErrors.newPassword ? <span className="wk-field-error text-xs leading-[1.4] text-[#c23434]">{fieldErrors.newPassword}</span> : null}
        </label>
        <label className="grid gap-[.35rem] text-[#27364d] font-semibold">
          {t('userProfile.changePassword.confirmLabel')}
          <Input
            className="w-full box-border border border-[#cbd5e1] rounded-control bg-white text-ink [font:inherit] px-[.65rem] py-[.55rem]"
            type="password"
            autoComplete="new-password"
            placeholder={t('userProfile.changePassword.confirmPlaceholder')}
            value={form.confirmation}
            onChange={(event) => update('confirmation', event.target.value)}
            onBlur={() => blurField('confirmation')}
          />
          {fieldErrors.confirmation ? <span className="wk-field-error text-xs leading-[1.4] text-[#c23434]">{fieldErrors.confirmation}</span> : null}
        </label>
        <div className="inline-edit-actions">
          <Button type="button" disabled={submitting} onClick={() => { resetForm(); setPasswordPopupOpen(false); }}>{t('common.cancel')}</Button>
          <Button type="submit" loading={submitting}>{t('userProfile.changePassword.submit')}</Button>
        </div>
      </form>
      </div>
      ) : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
    </div>
  );
}
