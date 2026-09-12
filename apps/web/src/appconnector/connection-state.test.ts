import test from 'node:test';
import assert from 'node:assert/strict';
import { connectionLabel } from './connection-state.ts';
test('installation does not imply shared credentials',()=>{
 assert.equal(connectionLabel({id:'c',kind:'personal',state:'active',owner_id:'u'}),'个人连接');
});
test('space kind is labelled as a shared space connection',()=>{
 assert.equal(connectionLabel({id:'c2',kind:'space',state:'active',owner_id:'u'}),'空间连接');
});
test('revoked connections are labelled revoked regardless of kind',()=>{
 assert.equal(connectionLabel({id:'c3',kind:'personal',state:'revoked',owner_id:'u'}),'连接已撤销');
 assert.equal(connectionLabel({id:'c4',kind:'space',state:'revoked',owner_id:null}),'连接已撤销');
});
