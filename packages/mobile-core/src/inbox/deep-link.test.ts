import test from 'node:test';
import assert from 'node:assert/strict';
import { parseNotificationDeepLink } from './deep-link.ts';

const VALID = 'weknora://tasks/detail?taskId=t-1&runId=r-1';

test('accepts the one canonical task-detail deep link and decodes ids', () => {
  assert.deepEqual(parseNotificationDeepLink(VALID), { kind: 'task-detail', taskId: 't-1', runId: 'r-1' });
  // scheme 大小写由 URL 归一（WHATWG URL 把 protocol 小写化）
  assert.deepEqual(parseNotificationDeepLink('WEKNORA://tasks/detail?taskId=t-1&runId=r-1'), { kind: 'task-detail', taskId: 't-1', runId: 'r-1' });
  // 百分号编码的 id 解码后保留原值（含空格与斜杠）
  assert.deepEqual(parseNotificationDeepLink('weknora://tasks/detail?taskId=a%20b&runId=r%2F1'), { kind: 'task-detail', taskId: 'a b', runId: 'r/1' });
});

test('rejects every malformed, foreign, or injected deep link', () => {
  const rejected = [
    '', '   ', 'not-a-url',
    VALID.replace('weknora:', 'https:'),          // https 伪装
    VALID.replace('weknora:', 'javascript:'),      // 协议处理器注入
    VALID.replace('weknora:', 'WEKNORA://tasks'),  // 双重前缀：path 变 //tasks/detail，不再是 /detail
    'weknora://evil/detail?taskId=t-1&runId=r-1',  // 未知 host
    'weknora://oidc?code=x&state=y',               // 非通知路由 host
    'weknora:///detail?taskId=t-1&runId=r-1',       // 空 host
    'weknora://tasks/other?taskId=t-1&runId=r-1',   // 未知 path
    'weknora://tasks/detail',                       // 缺全部参数
    'weknora://tasks/detail?taskId=t-1',            // 缺 runId
    'weknora://tasks/detail?taskId=%20&runId=r-1',  // 空白 taskId
    'weknora://tasks/detail?taskId=t-1&runId=',     // 空 runId
    `${VALID}#fragment`,                            // fragment 注入
    'weknora://tasks/detail?taskId=a%0Db&runId=r-1', // CRLF 注入（解码后含 \r）
    `weknora://tasks/detail?taskId=${'x'.repeat(129)}&runId=r-1`, // 超长 id
    `weknora://tasks/detail?taskId=t-1&runId=r-1&${'p=v&'.repeat(200)}x=1`, // 总长超 512（对齐 deep_link 列宽）
  ];
  for (const value of rejected) {
    assert.equal(parseNotificationDeepLink(value), undefined, `must reject: ${value.slice(0, 60)}`);
  }
});
