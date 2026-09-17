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

// R438 A3: Vue DataSourceSettings.vue renders the list as a responsive card
// grid (ds-grid, repeat(auto-fill,minmax(320px,1fr))) — not plain rows.
test('renders sources as a Vue-style card grid with a connector icon badge', () => {
  assert.match(page, /wk-data-source-grid grid grid-cols-\[repeat\(auto-fill,minmax\(320px,1fr\)\)\] gap-3/);
  assert.match(page, /wk-data-source-badge flex h-9 w-9 flex-none items-center justify-center rounded-\[9px\]/);
  assert.match(page, /bg-\[rgba\(7,192,95,0\.12\)\] text-\[15px\] font-semibold tracking-\[0\.02em\] text-\[#07c05f\]/);
  assert.match(page, /rounded-\[10px\] border border-line-soft bg-white px-4 py-\[14px\]/);
});

// Vue ds-card__status dot: active→success, paused→warning, error→error.
test('card subtitle carries the Vue status dot color semantics', () => {
  assert.match(page, /active' \? 'text-success-text' : status === 'paused' \? 'text-warning-text' : status === 'error' \? 'text-danger'/);
  assert.match(page, /h-1\.5 w-1.5 flex-none rounded-full bg-current/);
});

// Vue ds-card__detail: humanized schedule · relative last sync (full time on
// hover) · colored sync result · tabular metric pills; error box below.
test('card detail humanizes cron, relative time, sync result tone and metric pills', () => {
  assert.match(page, /import \{ humanizeCron, relativeTime, syncResultPills \} from '\.\/card\.ts';/);
  assert.match(page, /\{humanizeCron\(source\.sync_schedule, t\)\}/);
  assert.match(page, /\{relativeTime\(source\.last_sync_at, t\)\}/);
  assert.match(page, /title=\{fullTime \|\| undefined\}/);
  assert.match(page, /success' \? 'text-success-text' : status === 'failed' \? 'text-danger' : status === 'running' \? 'text-primary' : status === 'partial' \? 'text-warning-text'/);
  assert.match(page, /wk-data-source-metric font-mono text-\[11px\] tabular-nums/);
  assert.match(page, /source\.error_message \? <div className="mt-2 flex items-start gap-1\.5 rounded-md bg-danger\/10 px-2\.5 py-2 text-xs leading-snug text-danger">/);
});

// R437 pending adjudication: the extra test-connection button stays; action
// clicks must not bubble into the card-level openEdit.
test('keeps the test-connection button and stops action click bubbling in the card header', () => {
  assert.match(page, /\{t\('dataSource\.testConnection'\)\}/);
  assert.match(page, /onClick=\{\(event\) => event\.stopPropagation\(\)\}/);
});

// Vue appends the dashed add card to the grid for managers, even when sources exist.
test('appends the dashed add card to the grid for managers alongside sources', () => {
  const grid = page.slice(page.indexOf('wk-data-source-grid'));
  assert.match(grid, /\{canManage \? <button type="button" className="wk-data-source-create/);
});

// Vue DataSourceEditorDialog edit mode renders the configured-credential faux
// row (credential-faux-input): "configured ✓" with Replace (update) + Remove
// actions; Remove swaps the row into an inline confirm prompt
// (confirmRemovePrompt + cancel/confirmRemove) instead of a modal; the
// unconfigured degenerate row shows a Configure action that reveals the inputs;
// replace mode renders a Cancel action that discards anything typed.
test('renders the Vue credential status rows with replace/remove and inline confirm', () => {
  assert.match(page, /const kind = credentialStepKind\(\{ isEdit: Boolean\(editing\), credentialsConfigured: credentialStep\.credentialsConfigured, replaceMode: credentialStep\.replaceMode \}\)/);
  assert.match(page, /kind === 'configured' \? \(credentialStep\.pendingRemove \? /);
  assert.match(page, /t\('dataSource\.credential\.confirmRemovePrompt'\)/);
  assert.match(page, /t\('dataSource\.credential\.confirmRemove'\)/);
  assert.match(page, /t\('dataSource\.credential\.configured'\)/);
  assert.match(page, /t\('dataSource\.credential\.update'\)/);
  assert.match(page, /t\('dataSource\.credential\.remove'\)/);
  assert.match(page, /t\('dataSource\.credential\.unconfigured'\)/);
  assert.match(page, /t\('dataSource\.credential\.configure'\)/);
  assert.doesNotMatch(page, /window\.confirm\(`?\$\{t\('dataSource\.credential/);
});

// Vue confirmRemoveCredentials calls DELETE /credentials on the data source
// and, on success, resets to the unconfigured state with a removedToast. The
// typed api-client may not ship removeCredentials yet (file-frozen for this
// lane), so the page guards for it and surfaces removeFailed otherwise.
test('remove confirmation calls the credentials subresource and resets state', () => {
  assert.match(page, /async function confirmRemoveCredentials\(\)/);
  assert.match(page, /typeof api\.removeCredentials !== 'function'/);
  assert.match(page, /credentialStepReducer\(current, 'remove-confirmed'\)/);
  assert.match(page, /text: t\('dataSource\.credential\.removedToast'\)/);
  assert.match(page, /text: error instanceof Error \? error\.message : t\('dataSource\.credential\.removeFailed'\)/);
});

// Vue commitCredentialsIfNeeded collapses replace mode back to the configured
// row after a successful PUT /credentials.
test('a committed replacement collapses back to the configured row', () => {
  assert.match(page, /credentialStepReducer\(current, 'replace-committed'\)/);
});

// Vue replaces credentialsRequiredForValidation's "typed replacement" input
// with the explicit replace-mode flag, and cancel-replace clears the draft.
test('replace mode drives the validation exemption and cancel discards the draft', () => {
  assert.match(page, /replacementTyped: credentialStep\.replaceMode/);
  assert.match(page, /credentialStepReducer\(current, 'cancel-replace'\)/);
});

// Vue connectorDefs renders each credential field with its hint line
// (form-desc) and falls empty placeholders back to credential.inputPlaceholder.
test('credential fields render Vue hints and the input placeholder fallback', () => {
  assert.match(page, /placeholder=\{field\.placeholder \|\| t\('dataSource\.credential\.inputPlaceholder'\)\}/);
  assert.match(page, /\{field\.hint \? <small className="text-muted">\{t\(field\.hint\)\}<\/small> : null\}/);
});
