import { useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { envVarRemove, envVarSet, type EnvVarScope } from './surface.ts';

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

interface EnvVarRow { scope: EnvVarScope; scopeId: string; name: string; value: string }

function rows(payload: unknown): EnvVarRow[] {
  if (!Array.isArray(payload)) return [];
  return payload
    .filter((item): item is Record<string, unknown> => item !== null && typeof item === 'object' && !Array.isArray(item))
    .map((row) => {
      const skillId = typeof row.skill_id === 'string' ? row.skill_id : '';
      const configId = typeof row.sandbox_config_id === 'string' ? row.sandbox_config_id : '';
      return {
        scope: (skillId ? 'skill' : 'sandbox') as EnvVarScope,
        scopeId: skillId || configId,
        name: typeof row.name === 'string' ? row.name : '',
        value: typeof row.value === 'string' ? row.value : '',
      };
    })
    .filter((row) => row.scopeId && row.name);
}

export function EnvVarSettingsPanel({ client, initialPayload, onMutated }: { client: WeKnoraClient; initialPayload: unknown; onMutated?: () => void }) {
  const [scope, setScope] = useState<EnvVarScope>('skill');
  const [scopeId, setScopeId] = useState('');
  const [name, setName] = useState('');
  const [value, setValue] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  async function run(action: () => Promise<unknown>, success: string) {
    setBusy(true); setError(null); setNotice(null);
    try { await action(); setNotice(success); onMutated?.(); }
    catch (reason) { setError(errorText(reason, 'The environment-variable operation failed; the stored value was kept.')); }
    finally { setBusy(false); }
  }

  function setVariable(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    let mutation;
    try { mutation = envVarSet(scope, scopeId, name, value); }
    catch (reason) { setError(errorText(reason, 'Invalid environment-variable input.')); return; }
    const set = scope === 'skill'
      ? () => client.settings.envVars.skill.set(mutation.body.skill_id as string, mutation.name, mutation.value)
      : () => client.settings.envVars.sandbox.set(mutation.body.sandbox_config_id as string, mutation.name, mutation.value);
    void run(set, 'Environment variable saved for ' + scope + ' ' + mutation.body[scope === 'skill' ? 'skill_id' : 'sandbox_config_id'] + '.');
  }

  function removeVariable(row: EnvVarRow) {
    let body;
    try { body = envVarRemove(row.scope, row.scopeId, row.name); }
    catch { return; }
    const remove = row.scope === 'skill'
      ? () => client.settings.envVars.skill.remove((body as { skill_id: string }).skill_id, row.name)
      : () => client.settings.envVars.sandbox.remove((body as { sandbox_config_id: string }).sandbox_config_id, row.name);
    void run(remove, 'Environment variable ' + row.name + ' removed.');
  }

  return <Card data-testid="envvar-panel">
    {error ? <Status tone="error">{error}</Status> : null}
    {notice ? <Status tone="success">{notice}</Status> : null}
    <form className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px]" onSubmit={setVariable}>
      <label>Scope
        <select value={scope} onChange={(event) => setScope(event.target.value as EnvVarScope)}>
          <option value="skill">Skill (skill_id)</option>
          <option value="sandbox">Sandbox config (sandbox_config_id)</option>
        </select>
      </label>
      <label>{scope === 'skill' ? 'Skill ID' : 'Sandbox config ID'}<input required value={scopeId} onChange={(event) => setScopeId(event.target.value)} /></label>
      <label>Variable name<input required value={name} onChange={(event) => setName(event.target.value)} /></label>
      <label>Value<input required value={value} onChange={(event) => setValue(event.target.value)} /></label>
      <Button type="submit" loading={busy}>Set variable</Button>
    </form>
    {rows(initialPayload).length === 0
      ? <p className="wk-settings-read-note text-muted-strong text-[.9rem]">No personal environment variables are stored yet.</p>
      : <ul className="wk-list m-0 list-none p-0">{rows(initialPayload).map((row) => (
        <li key={row.scope + ':' + row.scopeId + ':' + row.name} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]">
          <strong>{row.name}</strong> · {row.scope} {row.scopeId} = {row.value}
          <Button type="button" disabled={busy} onClick={() => removeVariable(row)}>Remove</Button>
        </li>
      ))}</ul>}
  </Card>;
}
