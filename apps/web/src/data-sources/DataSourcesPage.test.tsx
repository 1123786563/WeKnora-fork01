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
  // SP2-a Task 10 rewrote remove() from a one-line window.confirm into the
  // dual-choice panel opener; the canManage gate stays the first statement.
  assert.match(page, /async function remove\(source: DataSource\) \{\s*if \(!canManage\) return;/);
});

test('preserves credential replacement and deletion semantics on edit', () => {
  assert.match(page, /if \(editing && !form\.credentialsText\.trim\(\)\) input\.config = \{ \.\.\.config, credentials: undefined \};/);
  // The commit gate includes the serialized rss header rows (Vue treats them
  // as part of the credential draft).
  assert.match(page, /if \(form\.credentialsText\.trim\(\) \|\| rssAuthHeadersSerialized\) \{[\s\S]*?if \(editing\) await dataSources\.putCredentials\(saved\.id, credentials\);/);
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

// SP2-a Task 6: a running latest sync log (isSyncRunning) swaps the card's
// sync action for a cancel button that POSTs cancelSyncLog on the running
// log's id. The cancel is cooperative (backend answers 202 cancel_requested;
// the sync loop exits at its next checkpoint), so the page performs no
// optimistic update — the existing 3s polling converges the row to canceled.
// The route is Admin-gated (routes_infra.go), matching the canManage prop the
// page already gates every mutation action on.
test('running syncs render a cancel button that requests cooperative cancel without optimistic update', () => {
  assert.match(page, /async function cancelSync\(source: DataSource\)/);
  assert.match(page, /const log = source\.latest_sync_log;[\s\S]*?if \(!log \|\| log\.status !== 'running'\) return;/);
  assert.match(page, /await dataSources\.cancelSyncLog\(source\.id, log\.id\);/);
  assert.match(page, /isSyncRunning\(source\) \? <Button type="button" disabled=\{action !== null\} onClick=\{\(\) => void cancelSync\(source\)\}>\{t\('dataSource\.cancelSync'\)\}<\/Button> : null/);
  // No optimistic flip: cancel success keeps the running pill and leans on the
  // 3s poll; only a toast communicates the request was accepted.
  assert.doesNotMatch(page, /setSources\(\(current\)[\s\S]*?cancel_requested/);
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
// typed api-client now ships removeCredentials, so the page calls it directly
// and surfaces the Vue removeFailed copy when the backend rejects.
test('remove confirmation calls the credentials subresource and resets state', () => {
  assert.match(page, /async function confirmRemoveCredentials\(\)/);
  assert.match(page, /await dataSources\.removeCredentials\(editing\.id\);/);
  assert.doesNotMatch(page, /typeof api\.removeCredentials/, 'feature-detect must go once the typed client ships removeCredentials');
  assert.match(page, /credentialStepReducer\(current, 'remove-confirmed'\)/);
  assert.match(page, /text: t\('dataSource\.credential\.removedToast'\)/);
  assert.match(page, /text: error instanceof Error \? error\.message : t\('dataSource\.credential\.removeFailed'\)/);
});

// Vue commitCredentialsIfNeeded collapses replace mode back to the configured
// row after a successful PUT /credentials.
test('a committed replacement collapses back to the configured row', () => {
  assert.match(page, /credentialStepReducer\(current, 'replace-committed'\)/);
});

// Vue connectorDefs gives rss a single custom_headers credential field: the
// editor renders key-value rows (add / remove) with the authHeaders hint and
// "Key: Value" placeholders, replacing the generic credentials textarea.
test('rss renders the Vue custom header rows editor in place of the generic textarea', () => {
  assert.match(page, /form\.type === 'rss' \? <div className="grid gap-2" data-kind="rss-auth-headers">/);
  assert.match(page, /t\('dataSource\.credential\.headerAdd'\)/);
  assert.match(page, /t\('dataSource\.credential\.headerKeyPlaceholder'\)/);
  assert.match(page, /t\('dataSource\.credential\.headerValuePlaceholder'\)/);
  assert.match(page, /\{t\('dataSource\.field\.authHeadersHint'\)\}/);
  assert.match(page, /aria-label=\{t\('common\.delete'\)\}/);
});

// Vue validateStep1Fields/commitCredentialsIfNeeded treat the serialized rss
// header rows as the credential draft: the connection test and the
// /credentials commit run when rows exist even with an empty credentialsText,
// and cancel-replace discards the rows like Vue cancelReplaceCredentials.
test('rss header rows gate the connection test and commit, cancel discards them', () => {
  assert.match(page, /const rssAuthHeadersSerialized = form\.type === 'rss' \? serializeAuthHeaders\(form\.authHeaders \?\? \[\]\) : '';/);
  assert.match(page, /if \(!form\.credentialsText\.trim\(\) && !rssAuthHeadersSerialized\) \{/);
  assert.match(page, /setForm\(\(current\) => \(\{ \.\.\.current, credentialsText: '', authHeaders: \[\] \}\)\)/);
});

// Vue 资源步为 Drive 连接器渲染 folder_token 输入行（输入 + 加载按钮 +
// shareHint / 必填内联错误），首次加载成功前显示占位块；打开资源步时预填
// 已保存 token，存在时自动加载（Vue nextStep step-2 branch）。
test('drive connectors render the Vue folder_token input row with load and placeholder', () => {
  assert.match(page, /data-kind="drive-folder-input"/);
  assert.match(page, /t\('dataSource\.drive\.folderTokenLabel'\)/);
  assert.match(page, /t\('dataSource\.drive\.folderTokenPlaceholder'\)/);
  assert.match(page, /t\('dataSource\.drive\.load'\)/);
  assert.match(page, /t\('dataSource\.drive\.shareHint'\)/);
  assert.match(page, /t\('dataSource\.drive\.folderTokenRequired'\)/);
  assert.match(page, /data-kind="drive-placeholder"/);
  assert.match(page, /t\('dataSource\.drive\.placeholderTitle'\)/);
  assert.match(page, /t\('dataSource\.drive\.placeholderDesc'\)/);
  assert.match(page, /async function loadDriveRoot\(source: DataSource, explicitToken\?: string\)/);
  assert.match(page, /const token = extractDriveFolderToken\(explicitToken \?\? driveToken\);/);
  assert.match(page, /setForm\(\(current\) => \(\{ \.\.\.current, resourceIds: \[token\] \}\)\)/);
  assert.match(page, /await dataSources\.update\(source\.id, \{ \.\.\.input, knowledge_base_id: knowledgeBaseId \}\);/);
  assert.match(page, /if \(isDriveConnector\(source\.type\)\) \{[\s\S]*?await loadDriveRoot\(source, token\);/);
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

// R452 A2: Vue DataSourceEditorDialog renders the gitlab projects editor as
// structured multi-row list (gitlab-project-list rows: project N + delete,
// project_id / ref inputs, paths textarea, add-project button) instead of the
// generic settingsText textarea; nextStep blocks the submit with
// datasource.gitlab.projectRequired until a row carries a project_id.
test('renders the Vue gitlab projects multi-row editor in the settings fieldset', () => {
  assert.match(page, /data-kind="gitlab-projects"/);
  assert.match(page, /t\('dataSource\.gitlab\.projects'\)/);
  assert.match(page, /t\('dataSource\.gitlab\.projectsHint'\)/);
  assert.match(page, /t\('dataSource\.gitlab\.addProject'\)/);
  assert.match(page, /data-kind="gitlab-project-row"/);
  assert.match(page, /\{t\('dataSource\.gitlab\.project'\)\} \{index \+ 1\}/);
  assert.match(page, /placeholder=\{t\('dataSource\.gitlab\.projectIdPlaceholder'\)\}/);
  assert.match(page, /placeholder=\{t\('dataSource\.gitlab\.refPlaceholder'\)\}/);
  assert.match(page, /placeholder=\{t\('dataSource\.gitlab\.pathsPlaceholder'\)\}/);
  assert.match(page, /updateForm\('gitlabProjects', \(form\.gitlabProjects \?\? \[\]\)\.filter\(\(_, at\) => at !== index\)\)/, 'each row has a remove action');
  assert.match(page, /updateForm\('gitlabProjects', \[\.\.\.\(form\.gitlabProjects \?\? \[\]\), \{ \.\.\.emptyGitLabProject \}\]\)/, 'the add button appends an empty row');
});

test('blocks the gitlab save with the Vue projectRequired warning', () => {
  assert.match(page, /if \(form\.type === 'gitlab' && !\(form\.gitlabProjects \?\? \[\]\)\.some\(\(project\) => project\.project_id\.trim\(\)\)\)/);
  assert.match(page, /tone: 'warning', text: t\('dataSource\.gitlab\.projectRequired'\)/);
});

// Vue openEditor def branch seeds one empty row when creating a gitlab
// connector (addGitLabProject); edits hydrate rows from settings.projects.
test('creating a gitlab connector seeds one empty project row', () => {
  assert.match(page, /type === 'gitlab' && \(current\.gitlabProjects \?\? \[\]\)\.length === 0 \? \[\{ \.\.\.emptyGitLabProject \}\] : current\.gitlabProjects/);
  // edits hydrate through dataSourceFormFrom, whose structured gitlabProjects
  // channel is covered by gitlab-projects.test.ts.
  assert.match(page, /setForm\(dataSourceFormFrom\(source\)\)/);
});

// R453 A1: Vue DataSourceEditorDialog step-1 renders a collapsible prereq
// setup-guide block (ds-setup-guide) above the form for connectors that
// declare required permissions (feishu/lark/feishu_drive/lark_drive/yuque),
// with the Vue t(perTypeKey, fallbackKey) copy chain, a permission-tag
// fallback when no per-type step-2 description exists, and a
// permissionPageUrl console link (target=_blank rel=noopener). It also
// renders a docHint inline alert with an openDoc link for connectors with a
// docUrl. The collapsed state resets whenever the editor (re)opens or the
// user picks a connector type (Vue prereqExpanded.value = false).
test('renders the Vue prereq setup-guide block with the per-type fallback chain', () => {
  assert.match(page, /VUE_CONNECTOR_GUIDES/);
  assert.match(page, /guide && guide\.requiredPermissions\.length > 0/);
  assert.match(page, /aria-expanded=\{prereqExpanded\}/);
  assert.match(page, /prereqCopy\(t, `dataSource\.prereqBarText_\$\{form\.type\}`, 'dataSource\.prereqBarText'\)/);
  assert.match(page, /prereqCopy\(t, `dataSource\.prereqStep1Brief_\$\{form\.type\}`, 'dataSource\.prereqBotBrief'\)/);
  assert.match(page, /prereqCopy\(t, `dataSource\.prereqStep2Brief_\$\{form\.type\}`, 'dataSource\.prereqPermBrief'\)/);
  assert.match(page, /prereqCopy\(t, `dataSource\.prereqStep3Brief_\$\{form\.type\}`, 'dataSource\.prereqMemberBrief'\)/);
  assert.match(page, /prereqCopy\(t, `dataSource\.prereqStep3Desc_\$\{form\.type\}`, 'dataSource\.prereqMemberDesc'\)/);
  assert.match(page, /prereqCopy\(t, `dataSource\.prereqOpenConsole_\$\{form\.type\}`, 'dataSource\.prereqOpenConsole'\)/);
  assert.match(page, /href=\{guide\.permissionPageUrl\} target="_blank" rel="noopener"/);
  assert.match(page, /setPrereqExpanded\(\(\) => false\)/);
});

test('renders the Vue docHint inline alert with an openDoc link', () => {
  assert.match(page, /guide\?\.docUrl/);
  assert.match(page, /t\('dataSource\.docHint'\)/);
  assert.match(page, /href=\{guide\.docUrl\} target="_blank" rel="noopener">\{t\('dataSource\.openDoc'\)\}/);
});

// R452 A1 browser evidence flagged English hardcoded empty-state copy; the
// direct page path was already localized (R442), and the hardcoded strings
// live in the knowledge-settings summary tiles, not here. Guard the page so
// the data-sources surface itself can never regress to hardcoded copy.
test('keeps the empty-state, add-card and loading copy on i18n keys', () => {
  assert.match(page, /t\('dataSource\.empty'\)/);
  assert.match(page, /t\('dataSource\.add'\)/);
  assert.match(page, /t\('common\.loading'\)/);
  assert.doesNotMatch(page, /No data sources/);
  assert.doesNotMatch(page, /Add an external connector/);
  assert.doesNotMatch(page, /Sync status/);
});

// SP2-a Task 10: the card delete action no longer window.confirms — it opens
// a controlled Sheet dual-choice panel. Entering the panel fetches the
// synced-documents count (GET /datasource/:id/documents-count, Task 9) that
// drives the "N synced documents" line, the purge checkbox label and the
// purge warning; a request serial keeps a late count response from landing
// in a panel opened for a different source.
test('delete opens a controlled dual-choice Sheet and drops window.confirm', () => {
  assert.doesNotMatch(page, /window\.confirm/);
  assert.match(page, /const \[deleteSource, setDeleteSource\] = useState<DataSource \| null>\(null\);/);
  assert.match(page, /const \[deletePurge, setDeletePurge\] = useState\(false\);/);
  assert.match(page, /async function remove\(source: DataSource\) \{\s*if \(!canManage\) return;[\s\S]*?setDeleteSource\(source\);[\s\S]*?setDeletePurge\(false\);/);
  assert.match(page, /const request = \+\+deleteCountRequest\.current;/);
  assert.match(page, /await dataSources\.documentsCount\(source\.id\);/);
  assert.match(page, /if \(deleteCountRequest\.current === request\) setDeleteCount\(count\);/);
  assert.match(page, /t\('dataSource\.deletePanelTitle', \{ name: deleteSource\.name \}\)/);
});

// Default state keeps the existing promise copy (documents stay); checking
// the purge checkbox swaps the body to the red irreversible warning and the
// confirm button to the deleteAndPurge label.
test('delete panel keeps the keep copy by default and swaps to the red purge warning once checked', () => {
  assert.match(page, /data-kind=\{deletePurge \? 'delete-purge-warning' : 'delete-keep'\}/);
  assert.match(page, /\{deletePurge \? <span className="font-medium text-danger">\{purgeWarningText\}<\/span> : t\('dataSource\.deletePanelKeep'\)\}/);
  assert.match(page, /<Checkbox checked=\{deletePurge\} onChange=\{\(event\) => setDeletePurge\(event\.target\.checked\)\} \/>/);
  assert.match(page, /purgeLabelText = deleteCount !== null \? t\('dataSource\.deletePanelPurgeLabel', \{ count: deleteCount \}\) : t\('dataSource\.deletePanelPurgeLabelUnknown'\)/);
  assert.match(page, /purgeWarningText = deleteCount !== null \? t\('dataSource\.deletePanelPurgeWarning', \{ count: deleteCount \}\) : t\('dataSource\.deletePanelPurgeWarningUnknown'\)/);
  assert.match(page, /\{deletePurge \? t\('dataSource\.deleteAndPurge'\) : t\('dataSource\.delete'\)\}/);
});

// The count line loads behind the panel: a loading placeholder first, the
// interpolated "synced documents: N" once the count resolves.
test('delete panel renders the documents count with a loading state', () => {
  assert.match(page, /deleteCountLoading \? <p className="m-0 text-muted" data-kind="delete-count-loading">\{t\('common\.loading'\)\}<\/p> : deleteCount !== null \? <p className="m-0 text-muted" data-kind="delete-count">\{t\('dataSource\.deletePanelCount', \{ count: deleteCount \}\)\}<\/p> : null/);
});

// Confirm runs the dual-choice delete: the checkbox rides into
// remove(id, {purgeDocuments}) (DELETE ?purge_documents=true, Task 9) and the
// success toast distinguishes purged from kept documents.
test('confirming the delete panel passes the purge choice to remove and splits the toast', () => {
  assert.match(page, /await dataSources\.remove\(deleteSource\.id, \{ purgeDocuments: purge \}\);/);
  assert.match(page, /text: purge \? t\('dataSource\.deleteSuccessPurged'\) : t\('dataSource\.deleteSuccess'\)/);
});
