import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, HTMLInputElement: dom.window.HTMLInputElement, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
const { createRoot } = await import('react-dom/client');
const { ChatComposer, resolveChatCopy } = await import('@weknora/views');

let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
});

/*
 * R472-A2 / R470 reverse gap: Vue Input-field.vue (~2717-2734) renders a
 * dedicated image upload button between the web-search toggle and the
 * paperclip, visible only when the selected agent config has
 * `image_upload_enabled === true` (`isImageUploadEnabledByAgent`, ~444-448;
 * quick-answer / builtin default config hides it). Its hidden input accepts
 * `image/jpeg,image/png,image/gif,image/webp` (multiple, ~2596) and picked
 * files travel through the same temporary-attachment transport as paperclip
 * files on the authenticated web client (chat/index.vue ~1139-1172 —
 * uploadTemporaryAttachment → attachment_ids). React therefore aligns by
 * gating a dedicated button on the agent config and routing its files into
 * the existing onAttachmentSelect pipeline.
 */

const IMAGE_ACCEPT = 'image/jpeg,image/png,image/gif,image/webp';

function renderComposer(overrides: Partial<React.ComponentProps<typeof ChatComposer>> = {}): HTMLElement {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  void act(() => root?.render(<ChatComposer copy={resolveChatCopy('zh-CN')} draft="" onDraftChange={() => undefined} onSubmit={() => undefined} {...overrides} />));
  return container;
}

test('image upload button renders only for an agent with image_upload_enabled', () => {
  const container = renderComposer({
    agents: [
      { id: 'agent-vlm', name: '视觉智能体', config: { image_upload_enabled: true } },
      { id: 'agent-text', name: '文本智能体', config: {} },
    ],
    selectedAgentId: 'agent-vlm',
    onAttachmentSelect: () => undefined,
  });
  const imageButton = container.querySelector<HTMLButtonElement>('button[aria-label="上传图片"]');
  assert.ok(imageButton, 'agent with image_upload_enabled must surface the dedicated image button');
  assert.ok(imageButton.getAttribute('title'), 'tooltip mirrors the Vue image-upload tooltip slot');
  const imageInput = container.querySelector<HTMLInputElement>(`input[accept="${IMAGE_ACCEPT}"]`);
  assert.ok(imageInput, 'the dedicated button must own an image-only accept input (Vue ~2596)');
  // Vue control order: image button sits before the paperclip attachment button.
  const attachmentButton = container.querySelector<HTMLButtonElement>('button[aria-label="上传附件"]');
  assert.ok(attachmentButton);
  assert.ok(imageButton.compareDocumentPosition(attachmentButton) & dom.window.Node.DOCUMENT_POSITION_FOLLOWING, 'image button precedes the attachment button (Vue ~2717-2754)');
});

test('image upload button stays hidden for agents without the flag and for quick-answer', () => {
  for (const overrides of [
    { agents: [{ id: 'agent-text', name: '文本智能体', config: {} }], selectedAgentId: 'agent-text' },
    { agents: [{ id: 'builtin-quick', name: '快速问答', config: { agent_mode: 'quick-answer' } }], selectedAgentId: 'builtin-quick' },
  ] as const) {
    const container = renderComposer({ ...overrides, onAttachmentSelect: () => undefined });
    assert.equal(container.querySelector('button[aria-label="上传图片"]'), null, `no image button for ${JSON.stringify(overrides.agents[0].config)}`);
    assert.equal(container.querySelector(`input[accept="${IMAGE_ACCEPT}"]`), null);
  }
});

test('clicking the image button opens the image-only file input', () => {
  const clicks: HTMLInputElement[] = [];
  const originalClick = HTMLInputElement.prototype.click;
  HTMLInputElement.prototype.click = function (this: HTMLInputElement) { clicks.push(this); return originalClick.call(this); };
  try {
    const container = renderComposer({
      agents: [{ id: 'agent-vlm', name: '视觉智能体', config: { image_upload_enabled: true } }],
      selectedAgentId: 'agent-vlm',
      onAttachmentSelect: () => undefined,
    });
    void act(() => container.querySelector<HTMLButtonElement>('button[aria-label="上传图片"]')?.click());
    const imageInput = container.querySelector<HTMLInputElement>(`input[accept="${IMAGE_ACCEPT}"]`);
    assert.equal(clicks[clicks.length - 1], imageInput, 'the click must land on the image input, not the attachment input');
  } finally {
    HTMLInputElement.prototype.click = originalClick;
  }
});

test('images picked through the dedicated input reuse the attachment pipeline', async () => {
  const selected: string[] = [];
  const container = renderComposer({
    agents: [{ id: 'agent-vlm', name: '视觉智能体', config: { image_upload_enabled: true } }],
    selectedAgentId: 'agent-vlm',
    onAttachmentSelect: (file) => { selected.push(file.name); },
  });
  const imageInput = container.querySelector<HTMLInputElement>(`input[accept="${IMAGE_ACCEPT}"]`);
  assert.ok(imageInput);
  const file = new dom.window.File(['bytes'], 'cat.png', { type: 'image/png' });
  Object.defineProperty(imageInput, 'files', { configurable: true, value: [file] });
  await act(async () => imageInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })));
  assert.deepEqual(selected, ['cat.png'], 'dedicated image picks must flow into onAttachmentSelect (shared attachment_ids transport)');
  assert.equal(imageInput.value === '' || imageInput.value, true, 'input resets like the Vue handleImageSelect');
});

test('the image button carries a count badge and active state from image attachments (Vue image-count)', () => {
  const container = renderComposer({
    agents: [{ id: 'agent-vlm', name: '视觉智能体', config: { image_upload_enabled: true } }],
    selectedAgentId: 'agent-vlm',
    onAttachmentSelect: () => undefined,
    attachments: [
      { id: 'a1', name: 'cat.png', status: 'ready' },
      { id: 'a2', name: 'notes.pdf', status: 'ready' },
    ],
  });
  const imageButton = container.querySelector<HTMLButtonElement>('button[aria-label="上传图片"]');
  assert.ok(imageButton);
  assert.equal(imageButton.getAttribute('data-image-count'), '1', 'only image attachments count toward the Vue image badge');
  assert.ok(imageButton.querySelector('.wk-chat-image-count'), 'badge node rendered');
  assert.equal(imageButton.getAttribute('data-active'), 'true');
});

test('resolveChatCopy exposes the image upload tooltip in every locale', () => {
  const expected = {
    'zh-CN': '上传图片',
    'en-US': 'Upload image',
    'ja-JP': '画像をアップロード',
    'ko-KR': '이미지 업로드',
    'ru-RU': 'Загрузить изображение',
  } as const;
  for (const [locale, label] of Object.entries(expected)) {
    assert.equal(resolveChatCopy(locale as keyof typeof expected).uploadImage, label);
  }
});
