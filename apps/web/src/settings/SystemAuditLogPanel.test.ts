import assert from 'node:assert/strict';
import test from 'node:test';

import { auditColumnLabels } from './SystemAuditLogPanel.helpers.ts';

test('uses the Vue audit column translations for each supported locale', () => {
  assert.deepEqual(auditColumnLabels('en-US'), ['Time', 'Actor', 'Action', 'Target', 'Outcome']);
  assert.deepEqual(auditColumnLabels('zh-CN'), ['时间', '操作者', '操作', '目标', '结果']);
});
