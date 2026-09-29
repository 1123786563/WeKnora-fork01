// ChatComposer 对平台壳层全局拖拽事件 weknora:chat-file-drop 的承接
//（Vue Input-field.vue:81/110-115/1821/1869 — handleChatFileDrop →
// handleDroppedFiles 把拖入文件投入输入框附件）。React 侧附件为统一管线
//（R472-A2）：拖入文件逐一经 onAttachmentSelect 进入，校验由宿主承担。
// Harness 跟随 apps/web/src/chat/chat-composer-keyboard.test.tsx。
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/creatChat' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, HTMLInputElement: dom.window.HTMLInputElement, CustomEvent: dom.window.CustomEvent, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
const { createRoot } = await import('react-dom/client');
const { ChatComposer } = await import('@weknora/views');

let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
});

function dispatchChatFileDrop(files: File[] | undefined): void {
  act(() => {
    window.dispatchEvent(new dom.window.CustomEvent('weknora:chat-file-drop', files === undefined ? {} : { detail: { files } }));
  });
}

const aFile = (name: string) => new dom.window.File([`-- ${name}`], name, { type: 'text/markdown' });

async function mountComposer(onAttachmentSelect?: (file: File) => void) {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => {
    root?.render(
      <ChatComposer
        draft=""
        onDraftChange={() => undefined}
        onSubmit={() => undefined}
        {...(onAttachmentSelect ? { onAttachmentSelect } : {})}
      />,
    );
  });
  return container;
}

test('weknora:chat-file-drop feeds each dropped file into onAttachmentSelect', async () => {
  const received: File[] = [];
  await mountComposer((file) => { received.push(file); });

  const first = aFile('notes-1.md');
  const second = aFile('notes-2.md');
  dispatchChatFileDrop([first, second]);

  assert.deepEqual(received, [first, second], 'dropped files enter the attachment pipeline in order');
});

test('an empty or missing detail file list is a no-op (Vue handleChatFileDrop guard)', async () => {
  const received: File[] = [];
  await mountComposer((file) => { received.push(file); });

  dispatchChatFileDrop([]);
  dispatchChatFileDrop(undefined);
  assert.deepEqual(received, []);
});

test('unmount removes the window listener', async () => {
  const received: File[] = [];
  await mountComposer((file) => { received.push(file); });
  await act(async () => root?.unmount());
  root = undefined;

  dispatchChatFileDrop([aFile('late.md')]);
  assert.deepEqual(received, [], 'no attachment is added after the composer unmounts');
});
