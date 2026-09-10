import { useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import type { WebScopeRuntime } from '../platform/scope-runtime.ts';

export interface WorkspaceOnboardingPageProps {
  client: WeKnoraClient;
  scopeRuntime: WebScopeRuntime;
  onLogout: () => Promise<void> | void;
}

/** The no-tenant boundary. Creation/invitation mutations stay in the next identity slice. */
export function WorkspaceOnboardingPage({ client, scopeRuntime, onLogout }: WorkspaceOnboardingPageProps) {
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');

  async function refreshPolicy() {
    setLoading(true);
    setMessage('');
    try {
      const next = await client.auth.me();
      scopeRuntime.hydrate(next);
      if (!scopeRuntime.requiresWorkspace()) window.location.assign('/platform/knowledge-bases');
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Unable to refresh workspace access.');
    } finally {
      setLoading(false);
    }
  }

  return <main className="wk-page"><Card>
    <h1>Choose a workspace</h1>
    <p className="wk-muted">Your account is signed in, but it does not have an active workspace yet.</p>
    {scopeRuntime.can('can_create_tenant') ? <Status>Create a workspace or accept an invitation to continue.</Status> : <Status>Ask a workspace administrator for an invitation.</Status>}
    {message ? <Status tone="error">{message}</Status> : null}
    <div className="wk-actions">
      <Button type="button" disabled={loading} onClick={() => void refreshPolicy()}>{loading ? 'Refreshing…' : 'Refresh access'}</Button>
      <Button type="button" onClick={() => void onLogout()}>Sign out</Button>
    </div>
  </Card></main>;
}
