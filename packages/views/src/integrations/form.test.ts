import assert from 'node:assert/strict';
import test from 'node:test';
import { buildEmbedUpdatePayload } from './form.ts';

test('embed name edits preserve the server-owned non-secret channel fields', () => {
  assert.deepEqual(buildEmbedUpdatePayload({
    agent_id: 'agent-1', enabled: true, allowed_origins: ['https://shop.example.test'], welcome_message: 'Welcome',
    rate_limit_per_minute: 12, rate_limit_per_day: 100, primary_color: '#123456', page_title: 'Support',
    header_title_mode: 'channel', show_suggested_questions: false, widget_position: 'top-left', allow_web_search: true,
    allow_file_upload: true, default_locale: 'en-US', webhook_url: '',
  }, 'Renamed'), {
    name: 'Renamed', agent_id: 'agent-1', enabled: true, allowed_origins: ['https://shop.example.test'], welcome_message: 'Welcome',
    rate_limit_per_minute: 12, rate_limit_per_day: 100, primary_color: '#123456', page_title: 'Support', header_title_mode: 'channel',
    show_suggested_questions: false, widget_position: 'top-left', allow_web_search: true, allow_file_upload: true, default_locale: 'en-US', webhook_url: '',
  });
});

test('embed updates fail closed when the origin allowlist is unavailable', () => {
  assert.throws(() => buildEmbedUpdatePayload({ name: 'broken', allowed_origins: null }, 'Renamed'), /allowed origin/);
});
