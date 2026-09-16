import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { SkillCatalogInstallResult, WeKnoraClient } from '@weknora/api-client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`, import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const {
  agentShareInput,
  AgentOperations,
  canStopSkillInstallation,
  ModelDebugPanel,
  modelDebugRequest,
  SkillOperations,
  skillInstallFeedback,
} = await import('./ConfigurationOperations.tsx');

const client = {} as WeKnoraClient;

test('agent sharing renders only the server-supported viewer permission', () => {
  const html = renderToStaticMarkup(<AgentOperations
    client={client}
    agents={[{ id: 'agent-1', name: 'Research' }]}
    disabledIds={[]}
  />);

  assert.match(html, /Viewer/);
  assert.match(html, /read-only/i);
  assert.doesNotMatch(html, /Editor|Admin/);
});

test('agent sharing always submits viewer permission', () => {
  assert.deepEqual(agentShareInput(' org-1 '), {
    organization_id: 'org-1',
    permission: 'viewer',
  });
});

test('only installing skill rows expose the stop action', () => {
  assert.equal(canStopSkillInstallation('installing'), true);
  assert.equal(canStopSkillInstallation('removing'), false);
  assert.equal(canStopSkillInstallation('removed'), false);

  const removing = renderToStaticMarkup(<SkillOperations client={client} initialCatalog={[{
    id: 'catalog-1', name: 'PDF', installations: [{
      skillId: 'skill-1', sandboxConfigId: 'cfg-1', enabled: false, status: 'removing',
    }],
  }]} />);
  const installing = renderToStaticMarkup(<SkillOperations client={client} initialCatalog={[{
    id: 'catalog-1', name: 'PDF', installations: [{
      skillId: 'skill-1', sandboxConfigId: 'cfg-1', enabled: false, status: 'installing',
    }],
  }]} />);

  assert.doesNotMatch(removing, />Stop</);
  assert.match(installing, />Stop</);
});

test('partial catalog installs report accepted installs and each rejected configuration', () => {
  const result: SkillCatalogInstallResult = {
    installs: { 'cfg-1': 'skill-1' },
    errors: { 'cfg-2': 'sandbox config not found', 'cfg-3': 'sandbox unavailable' },
  };

  assert.deepEqual(skillInstallFeedback(result), {
    tone: 'warning',
    message: 'Install accepted for 1 sandbox configuration(s); 2 rejected: cfg-2: sandbox config not found; cfg-3: sandbox unavailable.',
  });
});

test('model debug requires and forwards provider-specific browser files for VLLM and ASR', () => {
  const image = new Blob(['image'], { type: 'image/png' });
  const audio = new Blob(['audio'], { type: 'audio/wav' });

  assert.deepEqual(modelDebugRequest('VLLM', 'describe this', '', image), {
    input: 'describe this', documents: undefined, options: { thinking: false }, file: image,
  });
  assert.deepEqual(modelDebugRequest('ASR', '', '', audio), {
    input: '', documents: undefined, options: { thinking: false }, file: audio,
  });
  assert.throws(() => modelDebugRequest('VLLM', '', '', undefined), /image file is required/i);
  assert.throws(() => modelDebugRequest('ASR', '', '', undefined), /audio file is required/i);

  const vlm = renderToStaticMarkup(<ModelDebugPanel client={client} models={[{ id: 'vlm-1', name: 'Vision', type: 'VLLM' }]} />);
  const asr = renderToStaticMarkup(<ModelDebugPanel client={client} models={[{ id: 'asr-1', name: 'Speech', type: 'ASR' }]} />);
  assert.match(vlm, /type="file"[^>]*accept="image\/\*"/);
  assert.match(asr, /type="file"[^>]*accept="audio\/\*"/);
});
