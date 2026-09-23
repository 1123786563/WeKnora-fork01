import { useState } from 'react';
import type { AgentVersion } from '@weknora/contracts';
import type { ReleaseMetadataInput } from '@weknora/api-client';
import type { AgentMarketplaceApi } from './agent-marketplace-api.ts';
import './am-u.css';

interface AgentVersionActionsProps {
  api: AgentMarketplaceApi;
  agentId: string;
  agentName: string;
}

interface MetadataForm {
  semantic_version: string;
  display_name: string;
  summary: string;
  supported_languages: string;
  use_cases: string;
  non_use_cases: string;
  capability_requirements: string;
  data_categories: string;
  external_side_effects: string;
  minimum_weknora_capability: string;
  license_id: string;
  change_notes: string;
}

const splitList = (value: string): string[] => value.split(',').map((item) => item.trim()).filter(Boolean);

export function AgentVersionActions({ api, agentId, agentName }: AgentVersionActionsProps) {
  const [versions, setVersions] = useState<AgentVersion[]>([]);
  const [selectedVersionID, setSelectedVersionID] = useState('');
  const [metadata, setMetadata] = useState<MetadataForm>({
    semantic_version: '1.0.0', display_name: agentName, summary: '', supported_languages: 'en',
    use_cases: '', non_use_cases: '', capability_requirements: '', data_categories: '', external_side_effects: '',
    minimum_weknora_capability: '', license_id: '', change_notes: '',
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');

  const update = (key: keyof MetadataForm, value: string) => setMetadata((current) => ({ ...current, [key]: value }));
  const complete = metadata.semantic_version.trim() !== '' && metadata.display_name.trim() !== '' && metadata.summary.trim() !== ''
    && splitList(metadata.supported_languages).length > 0 && splitList(metadata.use_cases).length > 0
    && metadata.minimum_weknora_capability.trim() !== '' && metadata.license_id.trim() !== '';
  const freeze = async () => {
    if (busy) return;
    setBusy(true);
    setError('');
    setMessage('');
    try {
      const version = await api.versions.freezeVersion(agentId);
      setVersions((current) => [...current, version]);
      setSelectedVersionID(version.id);
      setMessage(`Frozen Agent Version ${version.version_number} (${version.id})`);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not freeze Agent Version');
    } finally {
      setBusy(false);
    }
  };

  const submit = async () => {
    if (busy || selectedVersionID === '') return;
    setBusy(true);
    setError('');
    setMessage('');
    try {
      const input: ReleaseMetadataInput = {
        semantic_version: metadata.semantic_version.trim(),
        display_name: metadata.display_name.trim(),
        summary: metadata.summary.trim(),
        supported_languages: splitList(metadata.supported_languages),
        use_cases: splitList(metadata.use_cases),
        ...(splitList(metadata.non_use_cases).length === 0 ? {} : { non_use_cases: splitList(metadata.non_use_cases) }),
        ...(splitList(metadata.capability_requirements).length === 0 ? {} : { capability_requirements: splitList(metadata.capability_requirements) }),
        ...(splitList(metadata.data_categories).length === 0 ? {} : { data_categories: splitList(metadata.data_categories) }),
        ...(splitList(metadata.external_side_effects).length === 0 ? {} : { external_side_effects: splitList(metadata.external_side_effects) }),
        minimum_weknora_capability: metadata.minimum_weknora_capability.trim(),
        license_id: metadata.license_id.trim(),
        ...(metadata.change_notes.trim() === '' ? {} : { change_notes: metadata.change_notes.trim() }),
      };
      const submission = await api.releases.submit(selectedVersionID, input);
      setMessage(`Submitted Release ${submission.id} for review`);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not submit Release');
    } finally {
      setBusy(false);
    }
  };

  const field = (key: keyof MetadataForm, label: string, required = true) => (
    <label className="wk-ava-1" key={key}>
      <span>{label}{required ? ' *' : ''}</span>
      {key === 'summary' || key === 'change_notes' ? (
        <textarea name={key} required={required} value={metadata[key]} onInput={(event) => update(key, event.currentTarget.value)} rows={key === 'summary' ? 3 : 2} className="wk-ava-2" />
      ) : (
        <input name={key} required={required} value={metadata[key]} onInput={(event) => update(key, event.currentTarget.value)} className="wk-ava-2" />
      )}
    </label>
  );

  return (
    <section aria-label="Author a Tenant Release" className="wk-ava-3" data-agent-release-author>
      <div>
        <h3 className="wk-ava-4">Tenant Release</h3>
        <p className="wk-ava-5">Freeze this saved Agent configuration, then submit its fixed version for Tenant review.</p>
      </div>
      <div className="wk-ava-6">
        <button type="button" data-author-freeze disabled={busy} onClick={() => void freeze()} className="wk-ava-7">Freeze Agent Version</button>
        {versions.length > 0 ? (
          <label className="wk-ava-8">Selected version
            <select data-author-version value={selectedVersionID} onChange={(event) => setSelectedVersionID(event.target.value)} disabled={busy} className="wk-ava-2">
              {versions.map((version) => <option key={version.id} value={version.id}>v{version.version_number} · {version.id} · {version.source_sha256}</option>)}
            </select>
          </label>
        ) : <span className="wk-ava-9">No frozen version selected</span>}
      </div>
      {selectedVersionID !== '' ? <>
        <div className="wk-ava-10 wk-ava-10-md">
          {field('semantic_version', 'Release version')}
          {field('display_name', 'Display name')}
          {field('summary', 'Summary')}
          {field('supported_languages', 'Supported languages (comma separated)')}
          {field('use_cases', 'Use cases (comma separated)')}
          {field('non_use_cases', 'Non-use cases (comma separated)', false)}
          {field('capability_requirements', 'Capability requirements (comma separated)', false)}
          {field('data_categories', 'Data categories (comma separated)', false)}
          {field('external_side_effects', 'External side effects (comma separated)', false)}
          {field('minimum_weknora_capability', 'Minimum WeKnora capability')}
          {field('license_id', 'License ID')}
          {field('change_notes', 'Change notes', false)}
        </div>
        <button type="button" data-author-submit disabled={busy || selectedVersionID === '' || !complete} onClick={() => void submit()} className="wk-ava-11 wk-ava-self-start">Submit Release for review</button>
      </> : null}
      {error ? <p role="alert" className="wk-ava-12">{error}</p> : null}
      {message ? <p role="status" className="wk-ava-13" data-author-status>{message}</p> : null}
    </section>
  );
}
