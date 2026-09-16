import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('./DataSourcesPage.tsx', import.meta.url), 'utf8');

test('keeps datasource editing in a body-level 640px Sheet with a real isolated form', () => {
  assert.match(page, /import \{ createPortal \} from 'react-dom';/);
  assert.match(page, /<Sheet open title=\{editorTitle\}/);
  assert.match(page, /width="640px"/);
  assert.match(page, /createPortal\(editorSurface, document\.body\)/);
  assert.match(page, /<form className="wk-wiki-editor grid gap-3" onSubmit=\{\(event\) => void save\(event\)\}/);
  assert.match(page, /<Button type="submit" loading=\{saving\}>/);
  assert.doesNotMatch(page, /<Card className="mt-4">.*dataSource\.createTitle/s, 'editor must not regress to an outer content card');
});

test('matches Vue datasource permissions and running-sync controls', () => {
  assert.match(page, /sources\.length === 0 && !canManage/);
  assert.match(page, /canManage \? <><Button type="button" onClick=\{\(\) => openEdit\(source\)\}/);
  assert.match(page, /onClick=\{\(\) => void run\(source, 'sync'\)\}/);
  assert.match(page, /disabled=\{action !== null \|\| isSyncRunning\(source\)\}/);
  assert.match(page, /source\.status === 'active'/);
  assert.doesNotMatch(page, /onClick=\{\(\) => showResources\(source\)\}/);
});

test('keeps permission gates on both opening and saving paths', () => {
  assert.match(page, /function openCreate\(\) \{ if \(accessDenied \|\| !canManage\) return;/);
  assert.match(page, /function openEdit\(source: DataSource\) \{ if \(!canManage\) return;/);
  assert.match(page, /async function save\(event\?: FormEvent<HTMLFormElement>\) \{[\s\S]*?if \(!canManage\) return;/);
  assert.match(page, /async function remove\(source: DataSource\) \{ if \(!canManage\) return;/);
});

test('preserves credential replacement and deletion semantics on edit', () => {
  assert.match(page, /if \(editing && !form\.credentialsText\.trim\(\)\) input\.config = \{ \.\.\.config, credentials: undefined \};/);
  assert.match(page, /if \(form\.credentialsText\.trim\(\)\) \{[\s\S]*?if \(editing\) await dataSources\.putCredentials\(saved\.id, credentials\);/);
});

// Vue DataSourceEditorDialog.handleSubmit: the create branch calls
// triggerSync(dataSourceId) right after creating the row and toasts
// datasource.createAndSyncSuccess; a failed trigger degrades to a
// MessagePlugin.warning with createButSyncFailed. The edit branch never
// triggers a sync and warns with updateSuccessSyncHint instead.
test('triggers the first sync right after creating a data source (Vue handleSubmit create branch)', () => {
  assert.match(page, /await dataSources\.sync\(saved\.id\);/);
  assert.match(page, /tone: 'success', text: t\('dataSource\.createAndSyncSuccess'\)/);
  assert.match(page, /tone: 'warning', text: syncError instanceof Error \? syncError\.message : t\('dataSource\.createButSyncFailed'\)/);
});

test('keeps the Vue post-save tone: editing warns that no auto sync runs', () => {
  assert.match(page, /setMessage\(\{ tone: 'warning', text: t\('dataSource\.updateSuccessSyncHint'\) \}\);/);
  assert.doesNotMatch(page, /tone: 'success', text: t\('dataSource\.updateSuccessSyncHint'\)/);
});

// Vue nextStep(): before the connection test, required credential fields are
// validated per field with `${label} ${datasource.isRequired}` warning that
// blocks the submit — not a single generic saveFailed message.
test('validates required credential fields per field before the connection test', () => {
  assert.match(page, /firstMissingRequiredCredential\(form\.type, form\.credentialsText\)/);
  assert.match(page, /tone: 'warning', text: `\$\{t\(missingCredential\)\} \$\{t\('dataSource\.isRequired'\)\}`/);
});

// Vue renders the create type step title via t('datasource.step.selectType');
// the React port must not hardcode Chinese copy that breaks other locales.
test('localizes the create type step title instead of hardcoding Chinese', () => {
  assert.match(page, /editing === null && createStep === 'type' \? t\('dataSource\.step\.selectType'\)/);
  assert.doesNotMatch(page, /选择类型/);
  assert.doesNotMatch(page, /选择要同步的外部数据源类型/);
});
