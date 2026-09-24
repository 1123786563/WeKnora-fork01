import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (h: {resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown}) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit:true, url:'data:text/javascript,export default {}' } : nextResolve(specifier,context) });
const React = await import('react');
const { renderToStaticMarkup } = await import('../../../../apps/web/node_modules/react-dom/server.js');
const { CraftSources } = await import('./sources.tsx');

test('Craft sources disclose empty and truncated retrieval, and show immutable observation time', () => {
 const empty = renderToStaticMarkup(React.createElement(CraftSources,{locale:'en',sources:[],truncated:true,openingCitation:null,onOpenSource:()=>{}}));
 assert.match(empty,/This run has no knowledge material/);
 assert.match(empty,/Retrieval exceeded the material caps/);
 const source = {citationId:'kc_a',ref:'craftkb:\/\/kb\/a\/knowledge\/k\/chunk\/c',title:'A',digest:'123456789abcdef',excerptBytes:4,tenantId:1,acquiredAt:'2026-09-23T01:02:03Z'};
 const shown = renderToStaticMarkup(React.createElement(CraftSources,{locale:'en',sources:[source],truncated:false,openingCitation:null,onOpenSource:()=>{},knowledgeBases:[{id:'a',name:'A'},{id:'b',name:'B'}],selectedKnowledgeBaseIds:['a'],onSelectionChange:()=>{}}));
 assert.match(shown,/2026-09-23T01:02:03Z/);
 assert.match(shown,/Select knowledge bases/);
 assert.equal((shown.match(/type="checkbox"/g) ?? []).length, 2);
 const revoked = renderToStaticMarkup(React.createElement(CraftSources,{locale:'en',sources:[source],truncated:false,openingCitation:null,onOpenSource:()=>{},revokedCitationIds:['kc_a']}));
 assert.doesNotMatch(revoked,/123456789abc/);
 assert.doesNotMatch(revoked,/2026-09-23T01:02:03Z/);
 assert.match(revoked,/disabled=""/);
});
