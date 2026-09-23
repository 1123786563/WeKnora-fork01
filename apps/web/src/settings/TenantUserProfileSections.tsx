import { useEffect, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
// S6：Status 无 TDesign 对应（playbook §1 附行），走 shared/wk-legacy。
import { WkStatus as Status } from '../shared/wk-legacy.tsx';
import { formatMessage, type Locale } from '@weknora/i18n';
import { profilePasswordPatch, tenantPatch } from './surface.ts';
import { TenantDeleteZone } from './TenantDeleteZone.tsx';

// T12a：UserProfileSection 直译 UserProfile.vue 的 t-popup / t-alert /
// t-loading / t-button / t-form / t-input（TenantInfoSection 仍走旧栈，
// settings-tenant 提交时迁移）。
import { Icon as TIcon } from 'tdesign-icons-react';
import type { FormInstanceFunctions } from 'tdesign-react/es/form/type';
import { Alert, Button as TButton, Form, Input as TInput, Loading, Popup, Progress, Tag, Textarea } from 'tdesign-react';
import { pushSettingsToast } from './settings-toast.tsx';

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

export function TenantInfoSection({ client, tenantId, role, locale, payload, error: loadError, loading, onRetry }: {
  client: WeKnoraClient;
  tenantId: number;
  role: string;
  locale: Locale;
  payload: unknown;
  /** 壳层读取失败原文（TenantInfo.vue error 态：t-alert theme=error + 重试）。 */
  error?: string | null;
  /** 壳层首载中（TenantInfo.vue loading 态）。 */
  loading?: boolean;
  onRetry?: () => void;
}) {
  const t = (key: string) => formatMessage(locale, key);
  const info = payload !== null && typeof payload === 'object' && !Array.isArray(payload) ? payload as Record<string, unknown> : {};
  // Vue canEditTenant = hasRole('owner')（TenantInfo.vue:273，按当前空间
  // membership 判定）。React 侧 SettingsRole 把 system-admin 折叠为高于
  // owner 的档位（router.tsx settingsRoute），故两者都可编辑。
  const canEditTenant = role === 'owner' || role === 'system-admin';
  const [editingName, setEditingName] = useState(false);
  const [nameDraft, setNameDraft] = useState('');
  const [editingDescription, setEditingDescription] = useState(false);
  const [descriptionDraft, setDescriptionDraft] = useState('');
  const [saving, setSaving] = useState(false);
  const [savingDescription, setSavingDescription] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const currentName = text(info.name);
  const currentDescription = text(info.description);

  function startEditName() { setError(null); setNotice(null); setNameDraft(currentName); setEditingName(true); }
  function startEditDescription() { setError(null); setNotice(null); setDescriptionDraft(currentDescription); setEditingDescription(true); }
  function cancelEdits() { setEditingName(false); setEditingDescription(false); setError(null); }

  async function saveName(reload: () => void) {
    if (saving || !nameDraft.trim()) return;
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
    if (savingDescription) return;
    setSavingDescription(true); setError(null); setNotice(null);
    try {
      const patch = tenantPatch(currentName || ' ', descriptionDraft);
      await client.settings.tenant.update(tenantId, patch);
      setEditingDescription(false);
      reload();
    } catch (reason) {
      setError(reason instanceof Error && reason.message ? reason.message : t('tenant.messages.fetchFailed'));
    } finally { setSavingDescription(false); }
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
  const reload = () => { window.location.reload(); };

  // TenantInfo.vue 逐节点复刻：section-header + loading/error-inline 自持态 +
  // tenant-info-body（settings-group 行 + 危险区 aside）。样式走
  // settings.td.css §5（TenantInfo.vue scoped 块平移）。
  return (
    <div className="tenant-info" data-testid="tenant-info-section">
      <div className="section-header">
        <h2>{t('tenant.title')}</h2>
        <p className="section-description">{t('tenant.sectionDescription')}</p>
      </div>

      {loading ? (
        <div className="loading-inline">
          <Loading size="small" />
          <span>{t('tenant.loadingInfo')}</span>
        </div>
      ) : loadError ? (
        <div className="error-inline">
          <Alert
            theme="error"
            message={loadError}
            operation={<TButton size="small" onClick={() => onRetry?.()}>{t('tenant.retry')}</TButton>}
          />
        </div>
      ) : (
        <div className="tenant-info-body">
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
                    <TInput
                      placeholder={t('tenant.details.editNamePlaceholder')}
                      maxlength={64}
                      disabled={saving}
                      autofocus
                      className="inline-edit-input"
                      value={nameDraft}
                      onChange={(value) => setNameDraft(String(value ?? ''))}
                      onEnter={() => { void saveName(reload); }}
                      onKeydown={(_value, { e }) => {
                        if (e.key === 'Escape') cancelEdits();
                      }}
                    />
                    <TButton theme="primary" size="small" loading={saving} disabled={!nameDraft.trim()} onClick={() => { void saveName(reload); }}>
                      {t('tenant.details.editNameConfirm')}
                    </TButton>
                    <TButton theme="default" variant="outline" size="small" disabled={saving} onClick={cancelEdits}>
                      {t('tenant.details.editNameCancel')}
                    </TButton>
                  </div>
                ) : (
                  <>
                    <span className="info-value">{currentName || '-'}</span>
                    {canEditTenant ? (
                      <TButton
                        theme="default"
                        variant="text"
                        shape="square"
                        size="small"
                        className="edit-btn"
                        title={t('tenant.details.editName')}
                        aria-label={t('tenant.details.editName')}
                        icon={<TIcon name="edit" />}
                        onClick={startEditName}
                      />
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
                      placeholder={t('tenant.details.editDescriptionPlaceholder')}
                      maxlength={512}
                      autosize={{ minRows: 2, maxRows: 6 }}
                      disabled={savingDescription}
                      autofocus
                      className="inline-edit-textarea"
                      value={descriptionDraft}
                      onChange={(value) => setDescriptionDraft(String(value ?? ''))}
                      count={({ count, maxLength }: { count: number; maxLength?: number }) => <span className="t-textarea__limit">{`${count}/${maxLength}`}</span>}
                      onKeydown={(_value, { e }) => {
                        if (e.key === 'Escape') cancelEdits();
                        if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); void saveDescription(reload); }
                      }}
                    />
                    <div className="inline-edit-actions">
                      <TButton theme="primary" size="small" loading={savingDescription} disabled={!descriptionDraft.trim()} onClick={() => { void saveDescription(reload); }}>
                        {t('tenant.details.editNameConfirm')}
                      </TButton>
                      <TButton theme="default" variant="outline" size="small" disabled={savingDescription} onClick={cancelEdits}>
                        {t('tenant.details.editNameCancel')}
                      </TButton>
                    </div>
                  </div>
                ) : (
                  <>
                    <span className={'info-value description-value' + (currentDescription ? '' : ' is-empty')}>{currentDescription || t('tenant.details.descriptionEmptyPlaceholder')}</span>
                    {canEditTenant ? (
                      <TButton
                        theme="default"
                        variant="text"
                        shape="square"
                        size="small"
                        className="edit-btn"
                        title={t('tenant.details.editDescription')}
                        aria-label={t('tenant.details.editDescription')}
                        icon={<TIcon name="edit" />}
                        onClick={startEditDescription}
                      />
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
              <div className="setting-control">
                <Tag theme={status.tone as 'default'} variant="light" size="small">{t(status.key)}</Tag>
              </div>
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
                  <div className="setting-control">
                    <div className="usage-control">
                      <span className="usage-text">{usage}%</span>
                      <Progress percentage={usage} label={false} size="small" status={usage > 80 ? 'warning' : 'success'} style={{ flex: 1 }} />
                    </div>
                  </div>
                </div>
              </>
            ) : null}
          </div>

          {/* TenantInfo.vue:202-215 deleteDangerZone（owner 且为当前空间时渲染；
              leaveDangerZone 需 owner 数 >1，单 owner 空间不渲染——与 Vue
              evaluateLeaveGate 同判）。删除确认弹窗保持 TenantDeleteZone 实现。 */}
          {canEditTenant && Number(info.id) === tenantId ? (
            <TenantDeleteZone client={client} tenantId={tenantId} tenantName={currentName || String(tenantId)} onDeleted={() => { window.location.assign('/login'); }} />
          ) : null}
        </div>
      )}
      {error ? <div className="error-inline"><Alert theme="error" message={error} /></div> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
    </div>
  );
}

/** Vue newPasswordRules（frontend/src/utils/passwordPolicy.ts）：长度 8-32 +
 * 复杂度按 complex 开关，附加 extraRules（sameAsCurrent 等页面规则）。 */
function newPasswordRules(t: (key: string) => string, complexEnabled: boolean, extraRules: Array<{ validator: (val: string) => boolean; message: string; type: 'error' }>): Array<Record<string, unknown>> {
  const base: Array<Record<string, unknown>> = [
    { required: true, message: t('auth.passwordRequired'), type: 'error' },
    { min: 8, message: t('auth.passwordMinLength'), type: 'error' },
    { max: 32, message: t('auth.passwordMaxLength'), type: 'error' },
  ];
  if (complexEnabled) {
    base.push(
      { pattern: /[a-z]/, message: t('auth.passwordMustContainLowercaseLetter'), type: 'error' },
      { pattern: /[A-Z]/, message: t('auth.passwordMustContainUppercaseLetter'), type: 'error' },
      { pattern: /\d/, message: t('auth.passwordMustContainNumber'), type: 'error' },
      { pattern: new RegExp('[' + PASSWORD_SPECIAL_CHARS.replace(/[-[\]{}()*+?.,\\^$|#\s]/g, '\\$&') + ']'), message: t('auth.passwordMustContainSpecialChar'), type: 'error' },
    );
  } else {
    base.push(
      { pattern: /[a-zA-Z]/, message: t('auth.passwordMustContainLetter'), type: 'error' },
      { pattern: /\d/, message: t('auth.passwordMustContainNumber'), type: 'error' },
    );
  }
  return [...base, ...extraRules];
}

// UserProfileSection —— T12a TDesign 同构迁移// UserProfileSection —— T12a TDesign 同构迁移：逐节点复刻
// frontend/src/views/settings/UserProfile.vue（section-header + loading-inline
// / error-inline t-alert + settings-group 行 + 密码行 popup 编辑）。样式走
// settings.td.css §3（UserProfile.vue scoped 块平移）。

// UserProfile.vue passwordRules 的 newPasswordRules 表（frontend/src/utils/
// passwordPolicy.ts）——tdesign Form rules 直接产出同一批 auth.* 文案。
const PASSWORD_SPECIAL_CHARS = '!@#$%^&*()_+-=[]{}|;:,.<>?';


export function UserProfileSection({ client, locale, payload, error: loadError, loading, onRetry }: {
  client: WeKnoraClient;
  locale: Locale;
  payload: unknown;
  /** 壳层读取失败原文（UserProfile.vue error 态：t-alert theme=error + 重试）。 */
  error?: string | null;
  /** 壳层首载中（UserProfile.vue loading 态：t-loading size=small）。 */
  loading?: boolean;
  onRetry?: () => void;
}) {
  const t = (key: string) => formatMessage(locale, key);
  const info = payload !== null && typeof payload === 'object' && !Array.isArray(payload) ? payload as Record<string, unknown> : {};
  const [complexPasswordEnabled, setComplexPasswordEnabled] = useState(false);
  // Vue UserProfile: the change-password form lives in a click popup off the
  // masked row's edit button (t-popup destroy-on-close).
  const [passwordPopupOpen, setPasswordPopupOpen] = useState(false);
  const [form, setForm] = useState({ oldPassword: '', newPassword: '', confirmPassword: '' });
  const [submitting, setSubmitting] = useState(false);
  // 评审 fix：t-form :rules 逐字段行内校验（UserProfile.vue:109-148 +
  // passwordRules :228-248——newPasswordRules 表 + sameAsCurrent/
  // confirmPassword 校验，错误文案走 t-form-item 行内 tips 而非 toast）。
  const passwordFormRef = useRef<FormInstanceFunctions | null>(null);

  useEffect(() => {
    // Vue 在 popup 打开时才拉 getAuthConfig 的 complex_password_enabled。
    if (!passwordPopupOpen) return;
    let active = true;
    void client.auth.registrationConfig()
      .then((config) => { if (active) setComplexPasswordEnabled(config.complexPasswordEnabled === true); })
      .catch(() => { if (active) setComplexPasswordEnabled(false); });
    return () => { active = false; };
  }, [client, passwordPopupOpen]);

  // Vue passwordRules（UserProfile.vue:228-248）：oldPassword 必填；
  // newPassword = newPasswordRules(t, complex, [sameAsCurrent])；confirmPassword
  // 必填 + 一致（trigger blur）。
  const passwordRules = {
    oldPassword: [{ required: true, message: t('userProfile.changePassword.currentRequired'), type: 'error' as const }],
    newPassword: newPasswordRules(t, complexPasswordEnabled, [
      {
        validator: (val: string) => val !== form.oldPassword,
        message: t('userProfile.changePassword.sameAsCurrent'),
        type: 'error' as const,
      },
    ]),
    confirmPassword: [
      { required: true, message: t('auth.confirmPasswordRequired'), type: 'error' as const },
      {
        validator: (val: string) => val === form.newPassword,
        message: t('auth.passwordMismatch'),
        type: 'error' as const,
        trigger: 'blur' as const,
      },
    ],
  };

  function resetPasswordForm() {
    setForm({ oldPassword: '', newPassword: '', confirmPassword: '' });
    passwordFormRef.current?.clearValidate();
  }

  async function submitPasswordChange() {
    if (submitting) return;
    // Vue submitPasswordChange（UserProfile.vue:297-299）：validate() 不通过
    // 即返回，错误以行内 tips 呈现；通过才发 changePassword。
    const result = await passwordFormRef.current?.validate();
    if (result !== true) return;
    setSubmitting(true);
    try {
      const patch = profilePasswordPatch(form.oldPassword, form.newPassword, form.confirmPassword, { complexPasswordEnabled });
      await client.settings.profile.changePassword(patch);
      setPasswordPopupOpen(false);
      resetPasswordForm();
      pushSettingsToast(t('userProfile.changePassword.success'), 'success');
    } catch (reason) {
      pushSettingsToast(reason instanceof Error && reason.message ? reason.message : t('userProfile.changePassword.failed'), 'error');
    } finally { setSubmitting(false); }
  }

  const passwordInput = (field: 'oldPassword' | 'newPassword' | 'confirmPassword', autocomplete: string, enterSubmit = false) => (
    <TInput
      type="password"
      autocomplete={autocomplete}
      disabled={submitting}
      placeholder={t('userProfile.changePassword.' + (field === 'oldPassword' ? 'currentPlaceholder' : field === 'newPassword' ? 'newPlaceholder' : 'confirmPlaceholder'))}
      value={form[field]}
      onChange={(value) => setForm((current) => ({ ...current, [field]: String(value ?? '') }))}
      {...(enterSubmit ? { onEnter: () => { void submitPasswordChange(); } } : {})}
    />
  );

  return (
    <div className="user-profile" data-testid="user-profile-section">
      <div className="section-header">
        <h2>{t('userProfile.title')}</h2>
        <p className="section-description">{t('userProfile.description')}</p>
      </div>

      {loading ? (
        <div className="loading-inline">
          <Loading size="small" />
          <span>{t('tenant.loadingInfo')}</span>
        </div>
      ) : loadError ? (
        <div className="error-inline">
          <Alert
            theme="error"
            message={loadError}
            operation={<TButton size="small" onClick={() => onRetry?.()}>{t('tenant.retry')}</TButton>}
          />
        </div>
      ) : (
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
            <div className="setting-control">
              <span className="info-value password-mask" aria-hidden="true">••••••••</span>
              <Popup
                trigger="click"
                // Vue placement="bottom-end"：tdesign-react 1.18.3 的
                // PopupPlacement 无 -end 粒度（库间差异，弹层开启态几何才有
                // 影响，稳态扫描不可见），bottom-right 同为右缘对齐。
                placement="bottom-right"
                destroyOnClose
                visible={passwordPopupOpen}
                onVisibleChange={(visible) => { if (!submitting) { setPasswordPopupOpen(visible); if (!visible) resetPasswordForm(); } }}
                overlayClassName="user-profile-password-popup-overlay"
                content={(
                  <div className="password-popup-inner" onClick={(event) => event.stopPropagation()}>
                    <div className="password-popup-title">{t('userProfile.changePassword.label')}</div>
                    <p className="password-popup-hint">{t('userProfile.changePassword.description')}</p>
                    <Form
                      ref={passwordFormRef}
                      initialData={form}
                      rules={passwordRules}
                      labelAlign="top"
                      className="password-popup-form"
                      onSubmit={(ctx) => { ctx.e?.preventDefault(); void submitPasswordChange(); }}
                    >
                      <Form.FormItem label={t('userProfile.changePassword.currentLabel')} name="oldPassword">
                        {passwordInput('oldPassword', 'current-password')}
                      </Form.FormItem>
                      <Form.FormItem label={t('userProfile.changePassword.newLabel')} name="newPassword">
                        {passwordInput('newPassword', 'new-password')}
                      </Form.FormItem>
                      <Form.FormItem label={t('userProfile.changePassword.confirmLabel')} name="confirmPassword">
                        {passwordInput('confirmPassword', 'new-password', true)}
                      </Form.FormItem>
                    </Form>
                    <div className="password-popup-footer">
                      <TButton variant="outline" disabled={submitting} onClick={() => { setPasswordPopupOpen(false); resetPasswordForm(); }}>
                        {t('common.cancel')}
                      </TButton>
                      <TButton theme="primary" loading={submitting} onClick={() => { void submitPasswordChange(); }}>
                        {t('userProfile.changePassword.submit')}
                      </TButton>
                    </div>
                  </div>
                )}
              >
                <TButton
                  theme="default"
                  variant="text"
                  shape="square"
                  size="small"
                  className="edit-btn"
                  title={t('userProfile.changePassword.label')}
                  aria-label={t('userProfile.changePassword.label')}
                  icon={<TIcon name="edit" />}
                />
              </Popup>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
