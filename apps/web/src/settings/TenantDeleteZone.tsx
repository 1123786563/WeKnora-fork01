import { useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { formatMessage } from '@weknora/i18n';
import { Button, Dialog, Input } from '@weknora/ui';
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

  const [open, setOpen] = useState(false);

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
    <>
      <aside data-testid="tenant-delete-zone" className="mt-3" aria-label={t('tenant.deleteDangerZone.title')}>
        <div className="flex items-center justify-between gap-5 rounded-[10px] border border-line-control bg-[#f3f3f3] px-[18px] py-4 max-[560px]:flex-col max-[560px]:items-stretch">
          <div className="min-w-0 flex-1 max-w-[28rem] pr-2 max-[560px]:max-w-none max-[560px]:pr-0">
            <p className="m-0 mb-1 text-[15px] font-medium leading-[1.4] text-ink">{t('tenant.deleteDangerZone.title')}</p>
            <p className="m-0 text-[13px] leading-[1.55] text-muted-strong">{t('tenant.deleteDangerZone.desc')}</p>
          </div>
          <div className="shrink-0 max-[560px]:flex max-[560px]:justify-end">
            <Button type="button" variant="danger" className="border border-danger text-danger" onClick={() => { setError(null); setOpen(true); }}>
              {t('tenant.deleteDangerZone.button')}
            </Button>
          </div>
        </div>
        {error ? <p role="alert" className="m-0 mt-2 text-[13px] text-danger">{error}</p> : null}
      </aside>
      <Dialog open={open} title={t('tenant.deleteDangerZone.confirmTitle')} onClose={closeDialog} closeLabel={t('common.close')}>
        <p className="m-0 mb-2 leading-[1.6] text-ink">{t('tenant.deleteDangerZone.confirmBody', { name: tenantName })}</p>
        <p className="m-0 mb-3 leading-[1.5] text-muted-strong">{t('tenant.deleteDangerZone.confirmHint', { name: tenantName })}</p>
        <Input
          className="w-full box-border border border-[#cbd5e1] rounded-control bg-white text-ink [font:inherit] px-[.65rem] py-[.55rem]"
          aria-label={t('tenant.deleteDangerZone.confirmTitle')}
          value={confirmText}
          onChange={(event) => setConfirmText(event.target.value)}
          placeholder={tenantName}
          disabled={busy}
        />
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="danger" data-testid="tenant-delete-button" disabled={!armed || busy} loading={busy} onClick={() => void handleDelete()}>
            {t('tenant.deleteDangerZone.confirm')}
          </Button>
          <Button type="button" disabled={busy} onClick={closeDialog}>{t('common.cancel')}</Button>
        </div>
      </Dialog>
    </>
  );
}
