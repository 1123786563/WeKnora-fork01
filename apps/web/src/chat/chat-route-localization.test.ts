import assert from 'node:assert/strict';
import test from 'node:test';

import { resolveChatSessionSourceOptions } from './session-source-options.ts';

test('chat session source options use the Vue locale labels', () => {
  assert.deepEqual(resolveChatSessionSourceOptions('en-US', true), [
    { value: '', label: 'All Sources' },
    { value: 'web', label: 'Web' },
    { value: 'embed', label: 'Embed' },
    { value: 'api', label: 'API' },
    { value: 'feishu', label: 'Feishu' },
    { value: 'wechat', label: 'WeChat' },
    { value: 'slack', label: 'Slack' },
  ]);
  assert.deepEqual(resolveChatSessionSourceOptions('zh-CN', false), [{ value: 'web', label: '网页端' }]);
});
