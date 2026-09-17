import test from 'node:test';
import assert from 'node:assert/strict';
import { parseNotificationLink } from './deep-link.ts';

test('deep links cannot authorize commands or inject tokens',()=>{
 assert.deepEqual(parseNotificationLink('weknora://execution?tenant=t&run=r'),{tenantID:'t',runID:'r'});
 for(const suffix of ['&action=approve','&token=x','&server=https://evil.test']) {
 assert.throws(()=>parseNotificationLink('weknora://execution?tenant=t&run=r'+suffix),/INVALID_LINK/);
 }
});

test('deep links reject duplicate keys, alternate authority casing, and unsafe references', () => {
  for (const raw of [
    'weknora://execution?tenant=t&tenant=t&run=r',
    'weknora://EXECUTION?tenant=t&run=r',
    'weknora://execution?tenant=t&run=../private',
    'https://execution?tenant=t&run=r',
    'weknora://execution?tenant=t%0A&run=r',
  ]) assert.throws(() => parseNotificationLink(raw), /INVALID_LINK/);
});
