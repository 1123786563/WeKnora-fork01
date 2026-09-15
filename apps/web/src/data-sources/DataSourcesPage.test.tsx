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
