import test from 'node:test';
import assert from 'node:assert/strict';
import { catalogPublishedLabel, catalogRiskLabel, catalogRiskTone, connectionLabel, connectionStateLabel, connectionStateTone, installationStateLabel, revokeFailureKind } from './connection-state.ts';
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

// --- R439 A3: Vue ConnectionsView/AppsView state-badge contract -----------

// Vue ConnectionsView stateLabel: active/revoked map to apps.common labels,
// anything else stays visible as 状态：{state} (never silently coerced).
test('connection state maps to the Vue state vocabulary',()=>{
 assert.equal(connectionStateLabel('active'),'活跃');
 assert.equal(connectionStateLabel('revoked'),'已断开');
 assert.equal(connectionStateLabel('pending'),'状态：pending');
});
test('connection state tone mirrors Vue stateTheme',()=>{
 assert.equal(connectionStateTone('active'),'success');
 assert.equal(connectionStateTone('revoked'),'error');
 assert.equal(connectionStateTone('pending'),'neutral');
});

// Vue AppsView installationStateLabel: active/disabled vocabulary.
test('installation state maps to the Vue installation vocabulary',()=>{
 assert.equal(installationStateLabel('active'),'活跃');
 assert.equal(installationStateLabel('disabled'),'已停用');
 assert.equal(installationStateLabel('upgrading'),'状态：upgrading');
 assert.equal(installationStateLabel(undefined),'—');
});

// Vue ConnectionsView revoke catch: 409 or VERSION_CONFLICT is a CAS conflict
// surfaced as a re-read prompt, not a generic failure.
test('revoke version conflict is recognized from status or code',()=>{
 assert.equal(revokeFailureKind({ status: 409 }),'conflict');
 assert.equal(revokeFailureKind({ code: 'VERSION_CONFLICT' }),'conflict');
 assert.equal(revokeFailureKind({ status: 500 }),'generic');
 assert.equal(revokeFailureKind(new Error('boom')),'generic');
 assert.equal(revokeFailureKind(null),'generic');
});

// Vue AppsView catalog badges: risk label vocabulary with raw fallback,
// risk theme mapping, and the published/unpublished tag.
test('catalog risk labels follow the Vue apps.risk vocabulary',()=>{
 assert.equal(catalogRiskLabel('read'),'只读');
 assert.equal(catalogRiskLabel('write'),'写入');
 assert.equal(catalogRiskLabel('send'),'发送');
 assert.equal(catalogRiskLabel('delete'),'删除');
 assert.equal(catalogRiskLabel('exotic'),'exotic');
});
test('catalog risk tone mirrors Vue riskTheme',()=>{
 assert.equal(catalogRiskTone('read'),'success');
 assert.equal(catalogRiskTone('write'),'warning');
 assert.equal(catalogRiskTone('send'),'error');
 assert.equal(catalogRiskTone('delete'),'error');
 assert.equal(catalogRiskTone('exotic'),'neutral');
});
test('catalog published badge follows the Vue published/unpublished tags',()=>{
 assert.equal(catalogPublishedLabel(true),'已发布');
 assert.equal(catalogPublishedLabel(false),'未发布');
});
