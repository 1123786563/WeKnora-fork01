import test from 'node:test';
import assert from 'node:assert/strict';
import { JSDOM } from 'jsdom';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../../../../..');
const require = createRequire(resolve(root, 'apps/web/package.json'));
// esbuild is resolved through the web package's declared dependencies (pnpm
// strict layout: it is not a dependency of @weknora/mobile, so a bare import
// only resolves via the tsx CLI's NODE_PATH side effect and fails under
// node --import tsx). Same host package as the react requires below.
const { build } = require('esbuild') as typeof import('esbuild');
const { act } = require('react');
const { createRoot } = require('react-dom/client');
async function mount(entry: string, runtime: any) {
  const dom = new JSDOM('<div id="root"></div>', { url: 'http://localhost' });
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true, __onboardingRuntime: runtime });
  const result = await build({ entryPoints: [resolve(root, entry)], bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic', external: ['react', 'react/jsx-runtime'], plugins: [{ name: 'native-host-fixtures', setup(b) {
    b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
    b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents: path === 'expo-router' ? `import React from 'react'; export const Redirect=({href})=><div data-redirect={href}/>; export const Stack=()=> <div data-protected="true"/>; export const useRouter=()=>({replace:()=>{}});` : path.endsWith('runtime.tsx') ? `export const useMobileRuntime=()=>globalThis.__onboardingRuntime;` : `import React from 'react'; export const View=({children,testID})=><div data-testid={testID}>{children}</div>; export const SafeAreaView=View, ScrollView=View; export const Text=({children,accessibilityRole})=><span role={accessibilityRole}>{children}</span>; export const ActivityIndicator=()=> <span>loading</span>; export const Pressable=({children,onPress,disabled,testID})=><button data-testid={testID} onClick={onPress} disabled={disabled}>{children}</button>; export const TextInput=({value,onChangeText,onBlur,testID})=><input data-testid={testID} value={value} onChange={e=>onChangeText(e.target.value)} onBlur={onBlur}/>; export const Modal=({visible,children})=>visible?<div role="dialog">{children}</div>:null; export const StyleSheet={create:x=>x};` }));
  }}] });
  const module = { exports: {} as any }; new Function('require', 'module', 'exports', result.outputFiles[0].text)(require, module, module.exports);
  const Component = module.exports.default ?? module.exports.OnboardingScreen;
  const host = document.getElementById('root')!; const renderer = createRoot(host);
  await act(async () => renderer.render(require('react').createElement(Component)));
  return { host, async click(id: string) { const button = host.querySelector(`[data-testid="${id}"]`) as HTMLButtonElement; assert.ok(button, `missing ${id}`); await act(async () => button.click()); }, async close() { await act(async () => renderer.unmount()); dom.window.close(); } };
}
const base = () => ({ hydrating: false, credential: { kind: 'bearer', accessToken: 'test' }, tenantId: null, canCreateTenant: false, refreshWorkspaces: async () => {}, logout: async () => {}, client: { identity: { tenants: { invitations: { pendingCount: async () => ({pendingCount: 0}), listMine: async () => ({items: []}), accept: async () => ({}), decline: async () => {} } } } } });
test('protected route never mounts children without a tenant', async () => { const page = await mount('apps/mobile/app/(app)/_layout.tsx', base()); try { assert.equal(page.host.querySelector('[data-protected]'), null); assert.equal(page.host.querySelector('[data-redirect]')?.getAttribute('data-redirect'), '../onboarding'); } finally { await page.close(); } });
test('onboarding invite-only policy exposes invitations and logout, not create', async () => { const runtime=base(); let logout=0; runtime.logout=async()=>{logout++}; const page=await mount('apps/mobile/app/onboarding.tsx', runtime); try { assert.equal(page.host.querySelector('[data-testid="create-open"]'),null); await page.click('invitations-open'); assert.ok(page.host.textContent?.includes('没有待处理')); await page.click('logout'); assert.equal(logout,1); } finally { await page.close(); } });
test('onboarding invitation can be accepted and removed from the list', async () => { const runtime=base(); let accepted=0; runtime.client.identity.tenants.invitations.listMine=async () => ({ items: [{ id: 7, tenant_id: 9, tenant_name: 'Docs', invitee_user_id: 'u1', role: 'viewer', status: 'pending', expires_at: '', created_at: '' }] as any }); runtime.client.identity.tenants.invitations.accept=async () => { accepted++; return {}; }; const page=await mount('apps/mobile/app/onboarding.tsx', runtime); try { await page.click('invitations-open'); await page.click('accept-7'); assert.equal(accepted,1); assert.ok(page.host.textContent?.includes('没有待处理')); } finally { await page.close(); } });
test('onboarding keeps invitation load errors separate from the empty state', async () => { const runtime=base(); runtime.client.identity.tenants.invitations.listMine=async () => { throw new Error('invitations unavailable'); }; const page=await mount('apps/mobile/app/onboarding.tsx', runtime); try { await page.click('invitations-open'); await new Promise((resolve) => setTimeout(resolve, 0)); assert.ok(page.host.textContent?.includes('invitations unavailable')); assert.equal(page.host.textContent?.includes('没有待处理'), false); } finally { await page.close(); } });
test('onboarding shows the pending invitation count beside the invitation action', async () => { const runtime=base(); runtime.client.identity.tenants.invitations.pendingCount=async () => ({ pendingCount: 3 }); const page=await mount('apps/mobile/app/onboarding.tsx', runtime); try { assert.ok(page.host.textContent?.includes('邀请 (3)')); } finally { await page.close(); } });
