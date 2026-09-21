import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Input, Select, Status } from '@weknora/ui';
import { MAX_ENV_VALUE_BYTES, isValidEnvValueLength } from '../configuration/management.ts';
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
  // R484 G4 D3 — the Vue section header carries a help-circle hint trigger
  // (EnvVarSettings.vue lines 6-22) opening a hover popup with the two intro
  // blocks; React keeps the same hover semantics via mouseover/mouseleave.
  const [helpOpen, setHelpOpen] = useState(false);

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
    // Vue rejectBadValue guard (EnvVarSettings.vue:485-495): reject an empty or
    // oversized value before any API call. The native `required` attribute only
    // guards interactive submissions; programmatic paths must be stopped here.
    if (!value) { setNotice(null); setError(t('envVarSettings.valueRequired')); return; }
    if (!isValidEnvValueLength(value)) { setNotice(null); setError(t('envVarSettings.valueTooLong', { max: MAX_ENV_VALUE_BYTES })); return; }
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

  // Section header ported from EnvVarSettings.vue lines 3-25: the panel owns
  // its h2 + help popup + description (the settings shell heading is skipped
  // for envvars) so the hint entry matches the Vue baseline in every state,
  // including the noConfig notice below.
  const sectionHeader = <div className="section-header">
    <div className="relative w-full">
      <div className="flex items-center gap-[6px]">
        <h2 className="m-0 text-[20px] font-semibold leading-[normal] text-[#27364d]">{t('envVarSettings.title')}</h2>
        <span className="relative inline-flex" onMouseLeave={() => setHelpOpen(false)}>
          <button
            type="button"
            className="inline-flex cursor-help items-center justify-center border-0 bg-transparent p-[2px] text-[#8a8a8a] hover:text-accent focus-visible:text-accent focus-visible:outline-none"
            aria-label={t('envVarSettings.helpAria')}
            aria-expanded={helpOpen}
            onMouseOver={() => setHelpOpen(true)}
            onFocus={() => setHelpOpen(true)}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <circle cx="12" cy="12" r="10" />
              <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
              <path d="M12 17h.01" />
            </svg>
          </button>
          {helpOpen ? (
            <div
              data-testid="envvar-help-popover"
              role="tooltip"
              className="absolute left-[calc(100%+6px)] top-0 z-30 grid w-[340px] max-w-[380px] gap-3 rounded-[8px] border border-[#e7e7e7] bg-white p-3 text-left shadow-[0_8px_24px_rgba(23,32,51,.14)]"
            >
              <div>
                <p className="m-0 text-[13px] font-semibold text-[#27364d]">{t('envVarSettings.introPersonalTitle')}</p>
                <p className="m-0 mt-1 text-[12px] leading-[1.55] text-[#66758b]">{t('envVarSettings.introPersonalBody')}</p>
              </div>
              <div>
                <p className="m-0 text-[13px] font-semibold text-[#27364d]">{t('envVarSettings.introRuntimeTitle')}</p>
                <p className="m-0 mt-1 text-[12px] leading-[1.55] text-[#66758b]">{t('envVarSettings.introRuntimeBody')}</p>
              </div>
            </div>
          ) : null}
        </span>
      </div>
      <p className="section-description m-0">{t('envVarSettings.description')}</p>
    </div>
  </div>;

  if (sandboxes !== null && sandboxes.length === 0) {
    // Vue EnvVarSettings.vue .env-empty: gray fill, no border, no card.
    return <div className="env-settings" data-testid="envvar-panel">
      {sectionHeader}
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      <div className="rounded-[10px] bg-[#f3f3f3] px-6 py-6 text-center">
        <p className="m-0 mb-[4px] text-[14px] font-medium leading-[normal] text-[rgba(0,0,0,0.6)]">{t('envVarSettings.noConfigTitle')}</p>
        <p className="m-0 text-[13px] leading-[normal] text-[rgba(0,0,0,0.4)]">{t('envVarSettings.noConfigDescription')}</p>
      </div>
    </div>;
  }

  return <Card className="env-settings" data-testid="envvar-panel">
    {sectionHeader}
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
