import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Input, Select, Status } from '@weknora/ui';
import { envVarRemove, envVarSet, type EnvVarScope } from './surface.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

interface EnvVarRow { scope: EnvVarScope; scopeId: string; name: string; value: string }

interface SandboxConfigOption { id: string; name: string }

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

function configOptions(payload: unknown): SandboxConfigOption[] {
  if (!Array.isArray(payload)) return [];
  return payload
    .filter((item): item is Record<string, unknown> => item !== null && typeof item === 'object' && !Array.isArray(item))
    .map((row) => ({
      id: typeof row.id === 'string' || typeof row.id === 'number' ? String(row.id) : '',
      name: typeof row.name === 'string' && row.name.trim() ? row.name.trim() : '',
    }))
    .filter((option) => option.id);
}

export function EnvVarSettingsPanel({ client, initialPayload, onMutated }: { client: WeKnoraClient; initialPayload: unknown; onMutated?: () => void }) {
  const t = settingsT(readInitialLocale());
  const [scope, setScope] = useState<EnvVarScope>('skill');
  const [scopeId, setScopeId] = useState('');
  const [name, setName] = useState('');
  const [value, setValue] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  // Vue EnvVarSettings guards the whole editor on sandbox availability: with
  // no workspace sandbox backend there is nothing to bind values to, so the
  // form is replaced by the noConfig notice.
  const [sandboxes, setSandboxes] = useState<SandboxConfigOption[] | null>(null);

  useEffect(() => {
    let active = true;
    void client.request({ method: 'GET', path: '/api/v1/sandbox-configs' }).then((value: unknown) => {
      if (active) setSandboxes(configOptions(value));
    }).catch(() => { if (active) setSandboxes([]); });
    return () => { active = false; };
  }, [client]);

  async function run(action: () => Promise<unknown>, success: string) {
    setBusy(true); setError(null); setNotice(null);
    try { await action(); setNotice(success); onMutated?.(); }
    catch (reason) { setError(errorText(reason, t('envVarSettings.saveFailed'))); }
    finally { setBusy(false); }
  }

  function setVariable(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    let mutation;
    try { mutation = envVarSet(scope, scopeId, name, value); }
    catch (reason) { setError(errorText(reason, t('envVarSettings.valueRequired'))); return; }
    const set = scope === 'skill'
      ? () => client.settings.envVars.skill.set(mutation.body.skill_id as string, mutation.name, mutation.value)
      : () => client.settings.envVars.sandbox.set(mutation.body.sandbox_config_id as string, mutation.name, mutation.value);
    void run(set, t('envVarSettings.saveSuccess'));
  }

  function removeVariable(row: EnvVarRow) {
    let body;
    try { body = envVarRemove(row.scope, row.scopeId, row.name); }
    catch { return; }
    const remove = row.scope === 'skill'
      ? () => client.settings.envVars.skill.remove((body as { skill_id: string }).skill_id, row.name)
      : () => client.settings.envVars.sandbox.remove((body as { sandbox_config_id: string }).sandbox_config_id, row.name);
    void run(remove, t('envVarSettings.deleteSuccess'));
  }

  if (sandboxes !== null && sandboxes.length === 0) {
    return <Card data-testid="envvar-panel">
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      <div className="py-6 text-center">
        <p className="m-0 text-[15px] font-semibold text-[#27364d]">{t('envVarSettings.noConfigTitle')}</p>
        <p className="m-0 mt-2 text-[13px] text-muted-strong">{t('envVarSettings.noConfigDescription')}</p>
      </div>
    </Card>;
  }

  return <Card data-testid="envvar-panel">
    {error ? <Status tone="error">{error}</Status> : null}
    {notice ? <Status tone="success">{notice}</Status> : null}
    <form className="wk-settings-editor my-4 grid max-w-[620px] gap-[.8rem] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold" onSubmit={setVariable}>
      <label>{t('envVarSettings.sandboxPick')}
        <Select value={scope} onChange={(event) => setScope(event.target.value as EnvVarScope)}>
          <option value="skill">{t('envVarSettings.skillTitle')}</option>
          <option value="sandbox">{t('envVarSettings.sandboxTitle')}</option>
        </Select>
      </label>
      <label>{scope === 'skill' ? t('envVarSettings.skillOnSandbox', { name: 'ID' }) : t('envVarSettings.sandboxPick')}
        {scope === 'sandbox'
          ? <Select required value={scopeId} onChange={(event) => setScopeId(event.target.value)}>
              <option value="" disabled>{t('envVarSettings.sandboxPick')}</option>
              {(sandboxes ?? []).map((option) => <option key={option.id} value={option.id}>{option.name || option.id}</option>)}
            </Select>
          : <Input required value={scopeId} onChange={(event) => setScopeId(event.target.value)} />}
      </label>
      <label>{t('envVarSettings.namePlaceholder')}<Input required value={name} onChange={(event) => setName(event.target.value)} /></label>
      <label>{t('envVarSettings.valuePlaceholder')}<Input type="password" autoComplete="new-password" required value={value} onChange={(event) => setValue(event.target.value)} /></label>
      <Button type="submit" loading={busy}>{t('envVarSettings.save')}</Button>
    </form>
    {rows(initialPayload).length === 0
      ? <p className="wk-settings-read-note text-muted-strong text-[.9rem]">{t('envVarSettings.sandboxEmpty')}</p>
      : <ul className="wk-list m-0 list-none p-0">{rows(initialPayload).map((row) => (
        <li key={row.scope + ':' + row.scopeId + ':' + row.name} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]">
          <strong>{row.name}</strong> · {row.scope} {row.scopeId}
          <Button type="button" disabled={busy} onClick={() => removeVariable(row)}>{t('envVarSettings.delete')}</Button>
        </li>
      ))}</ul>}
  </Card>;
}
