import { useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { formatMessage } from '@weknora/i18n';
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
  const armed = confirmText === tenantName && tenantName.length > 0;

  async function handleDelete() {
    if (!armed || busy) return;
    setBusy(true);
    setError(null);
    try {
      await client.identity.tenants.admin.deleteTenant(tenantId);
      onDeleted();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t('tenant.messages.fetchFailed'));
      setBusy(false);
    }
  }

  return (
    <div data-testid="tenant-delete-zone" className="wk-delete-zone">
      <div className="wk-delete-zone__text">
        <p className="wk-delete-zone__title">{t('tenant.deleteDangerZone.title')}</p>
        <p className="wk-delete-zone__desc">{t('tenant.deleteDangerZone.desc')}</p>
      </div>
      <div className="wk-delete-zone__confirm">
        <p className="wk-delete-zone__hint">{t('tenant.deleteDangerZone.confirmHint', { name: tenantName })}</p>
        <input
          aria-label={t('tenant.deleteDangerZone.confirmTitle')}
          value={confirmText}
          onChange={(event) => setConfirmText(event.target.value)}
          placeholder={tenantName}
          disabled={busy}
        />
        <button
          type="button"
          data-testid="tenant-delete-button"
          className="wk-delete-zone__button"
          disabled={!armed || busy}
          onClick={() => void handleDelete()}
        >
          {t('tenant.deleteDangerZone.button')}
        </button>
      </div>
      {error ? <p role="alert" className="wk-delete-zone__error">{error}</p> : null}
    </div>
  );
}
