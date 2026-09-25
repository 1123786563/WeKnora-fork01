import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
// S6：Status 无 TDesign 对应（playbook §1 附行），走 shared/wk-legacy。
import { WkStatus as Status } from '../shared/wk-legacy.tsx';
// T12a：可见面（section-header + hint popup + 空态）直译 EnvVarSettings.vue
// 的 t-popup / t-icon / t-button / t-input / t-select；编辑器分支同组件换
// tdesign（结构保留 React 侧表单，见 task-12a 报告偏离项）。
import { Icon as TIcon } from 'tdesign-icons-react';
import { Button, Input, Popup, Select } from 'tdesign-react';
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

  // T12a：EnvVarSettings.vue :3-25 逐节点复刻——section-header__titlewrap
  // （h2 + hint-trigger t-icon help-circle + t-popup hover）+ description。
  const sectionHeader = <div className="section-header">
    <div className="section-header__titlewrap">
      <h2>{t('envVarSettings.title')}</h2>
      <Popup
        // Vue placement="bottom-start"；react 1.18.3 PopupPlacement 无 -start
        // 粒度（库间差异，hover 弹层几何才有影响，稳态扫描不可见）。
        placement="bottom-left"
        trigger="hover"
        overlayInnerStyle={{ maxWidth: '380px' }}
        content={(
          <div className="hint-popover hint-popover--env" data-testid="envvar-help-popover">
            <div className="hint-popover__block">
              <p className="hint-popover__title">{t('envVarSettings.introPersonalTitle')}</p>
              <p className="hint-popover__text">{t('envVarSettings.introPersonalBody')}</p>
            </div>
            <div className="hint-popover__block">
              <p className="hint-popover__title">{t('envVarSettings.introRuntimeTitle')}</p>
              <p className="hint-popover__text">{t('envVarSettings.introRuntimeBody')}</p>
            </div>
          </div>
        )}
      >
        <button type="button" className="hint-trigger" aria-label={t('envVarSettings.helpAria')}>
          <TIcon name="help-circle" size="16px" />
        </button>
      </Popup>
    </div>
    <p className="section-description">{t('envVarSettings.description')}</p>
  </div>;

  if (sandboxes !== null && sandboxes.length === 0) {
    // Vue EnvVarSettings.vue .env-empty: gray fill, no border, no card.
    return <div className="env-settings" data-testid="envvar-panel">
      {sectionHeader}
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      <div className="env-empty">
        <p className="env-empty__title">{t('envVarSettings.noConfigTitle')}</p>
        <p className="env-empty__desc">{t('envVarSettings.noConfigDescription')}</p>
      </div>
    </div>;
  }

  return <div className="env-settings" data-testid="envvar-panel">
    {sectionHeader}
    {error ? <Status tone="error">{error}</Status> : null}
    {notice ? <Status tone="success">{notice}</Status> : null}
    <form className="wk-settings-editor" onSubmit={setVariable}>
      <label>{t('envVarSettings.sandboxPick')}
        <Select value={scope} onChange={(next) => setScope(String(next) as EnvVarScope)}>
          <Select.Option value="skill" label={t('envVarSettings.skillTitle')}>{t('envVarSettings.skillTitle')}</Select.Option>
          <Select.Option value="sandbox" label={t('envVarSettings.sandboxTitle')}>{t('envVarSettings.sandboxTitle')}</Select.Option>
        </Select>
      </label>
      <label>{scope === 'skill' ? t('envVarSettings.skillOnSandbox', { name: 'ID' }) : t('envVarSettings.sandboxPick')}
        {scope === 'sandbox'
          ? <Select value={scopeId} onChange={(next) => setScopeId(String(next))}>
              {(sandboxes ?? []).map((option) => <Select.Option key={option.id} value={option.id} label={option.name || option.id}>{option.name || option.id}</Select.Option>)}
            </Select>
          : <Input value={scopeId} onChange={(next) => setScopeId(String(next ?? ''))} />}
      </label>
      <label>{t('envVarSettings.namePlaceholder')}<Input value={name} onChange={(next) => setName(String(next ?? ''))} /></label>
      <label>{t('envVarSettings.valuePlaceholder')}<Input type="password" autocomplete="new-password" value={value} onChange={(next) => setValue(String(next ?? ''))} /></label>
      <Button type="submit" loading={busy}>{t('envVarSettings.save')}</Button>
    </form>
    {rows(initialPayload).length === 0
      ? <p className="wk-settings-read-note">{t('envVarSettings.sandboxEmpty')}</p>
      : <ul className="wk-list">{rows(initialPayload).map((row) => (
        <li key={row.scope + ':' + row.scopeId + ':' + row.name} className="wk-list-row">
          <strong>{row.name}</strong> · {row.scope} {row.scopeId}
          <Button type="button" disabled={busy} onClick={() => removeVariable(row)}>{t('envVarSettings.delete')}</Button>
        </li>
      ))}</ul>}
  </div>;
}
