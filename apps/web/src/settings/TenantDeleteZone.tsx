import { useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';

/** Danger zone for permanent workspace deletion. Owner+ only (backend RBAC:
 *  routes_auth_tenant.go DELETE /:id). Requires typing the workspace name to
 *  confirm, matching the Vue GeneralSettings delete confirmation semantics. */
export function TenantDeleteZone({ client, tenantId, tenantName, onDeleted }: {
  client: WeKnoraClient;
  tenantId: number;
  tenantName: string;
  onDeleted: () => void;
}) {
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
      setError(reason instanceof Error ? reason.message : 'Workspace deletion failed.');
      setBusy(false);
    }
  }

  return (
    <div data-testid="tenant-delete-zone" style={{ marginTop: 24, paddingTop: 16, borderTop: '1px solid #fecaca' }}>
      <p style={{ color: '#dc2626', fontWeight: 600, margin: '0 0 4px' }}>Danger zone</p>
      <p style={{ color: '#6b7280', fontSize: 13, margin: '0 0 8px' }}>
        Type the workspace name <strong>{tenantName}</strong> to confirm. This permanently deletes the workspace and all its data.
      </p>
      <input
        aria-label="Confirm workspace name"
        value={confirmText}
        onChange={(event) => setConfirmText(event.target.value)}
        placeholder={tenantName}
        disabled={busy}
        style={{ width: '100%', maxWidth: 320, padding: '8px 12px', border: '1px solid #fecaca', borderRadius: 6, marginBottom: 8 }}
      />
      <div>
        <button
          type="button"
          data-testid="tenant-delete-button"
          disabled={!armed || busy}
          onClick={() => void handleDelete()}
          style={{ padding: '8px 16px', background: armed ? '#dc2626' : '#f3f4f6', color: armed ? '#fff' : '#9ca3af', border: 0, borderRadius: 6, cursor: armed ? 'pointer' : 'not-allowed' }}
        >
          {busy ? 'Deleting…' : 'Delete workspace permanently'}
        </button>
      </div>
      {error ? <p role="alert" style={{ color: '#dc2626', fontSize: 13, marginTop: 8 }}>{error}</p> : null}
    </div>
  );
}
