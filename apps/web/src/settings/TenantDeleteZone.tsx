import { useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { formatMessage } from '@weknora/i18n';
// T12a：TenantInfo.vue:202-237 deleteDangerZone 直译——.leave-space-panel
// delete-space-panel 结构 + t-button theme=danger + t-dialog（cancelBtn
// variant=base 补齐台账 #1）。
import { Button, Dialog, Input } from 'tdesign-react';
import { useAppLocale } from '../i18n.ts';

/** Danger zone for permanent workspace deletion. Owner+ only (backend RBAC:
 *  routes_auth_tenant.go DELETE /:id). Requires typing the workspace name to
 *  confirm, matching the Vue TenantInfo.vue deleteDangerZone confirm dialog
 *  (lines 202-237): localized title/description/hint/button with the shared
 *  tenant.deleteDangerZone.* keys. */
export function TenantDeleteZone({ client, tenantId, tenantName, onDeleted }: {
  client: WeKnoraClient;
  tenantId: number;
  tenantName: string;
  onDeleted: () => void;
}) {
  const locale = useAppLocale();
  const t = (key: string, values?: Record<string, string | number>) => formatMessage(locale, key, values);
  const [confirmText, setConfirmText] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  // TenantInfo.vue enables its destructive confirmation once the trimmed
  // input equals the tenant name, so pasted whitespace must not strand the
  // React owner in a disabled dialog.
  const armed = confirmText.trim() === tenantName && tenantName.length > 0;

  function closeDialog() {
    if (busy) return;
    setOpen(false);
    setConfirmText('');
    setError(null);
  }

  async function handleDelete() {
    if (!armed || busy) return;
    setBusy(true);
    setError(null);
    try {
      await client.identity.tenants.admin.deleteTenant(tenantId);
      onDeleted();
    } catch (reason) {
      setError(reason instanceof Error && reason.message ? reason.message : t('tenant.messages.fetchFailed'));
      setBusy(false);
    }
  }

  return (
    <>
      <aside data-testid="tenant-delete-zone" className="leave-space-panel delete-space-panel" aria-label={t('tenant.deleteDangerZone.title')}>
        <div className="leave-space-panel-inner">
          <div className="leave-space-panel-text">
            <div className="leave-space-panel-title">{t('tenant.deleteDangerZone.title')}</div>
            <p className="leave-space-panel-desc">{t('tenant.deleteDangerZone.desc')}</p>
          </div>
          <div className="leave-space-panel-action">
            <Button theme="danger" size="medium" onClick={() => { setError(null); setConfirmText(''); setOpen(true); }}>
              {t('tenant.deleteDangerZone.button')}
            </Button>
          </div>
        </div>
        {error ? <p role="alert" className="error-inline" style={{ color: 'var(--td-error-color)', fontSize: 13, margin: '8px 0 0' }}>{error}</p> : null}
      </aside>
      <Dialog
        visible={open}
        header={t('tenant.deleteDangerZone.confirmTitle')}
        confirmBtn={{
          content: t('tenant.deleteDangerZone.confirm'),
          theme: 'danger',
          disabled: !armed || busy,
          loading: busy,
        }}
        cancelBtn={{ content: t('common.cancel'), variant: 'base' }}
        closeBtn={!busy}
        closeOnOverlayClick={!busy}
        onClose={closeDialog}
        onCancel={closeDialog}
        onConfirm={() => { void handleDelete(); }}
      >
        <div className="delete-tenant-confirm">
          <p className="delete-tenant-confirm-body">
            {t('tenant.deleteDangerZone.confirmBody', { name: tenantName })}
          </p>
          <p className="delete-tenant-confirm-hint">
            {t('tenant.deleteDangerZone.confirmHint', { name: tenantName })}
          </p>
          <Input
            placeholder={tenantName}
            disabled={busy}
            clearable
            value={confirmText}
            onChange={(value) => setConfirmText(String(value ?? ''))}
          />
        </div>
      </Dialog>
    </>
  );
}
