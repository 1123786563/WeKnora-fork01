import {readFileSync,writeFileSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import {resolve,dirname} from 'node:path';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'../src/styles');
const source=JSON.parse(readFileSync(resolve(root,'tokens.json'),'utf8'));
function value(t,mini=false){if(t.type==='dimension')return `${t.value*(mini?2:1)}${mini?'rpx':'px'}`;if(t.type==='fixedPx')return `${t.value}px`;if(t.type==='duration')return `${t.value}ms`;return String(t.value)}
const outputs={
 'tokens.css':'/* Generated from tokens.json; edit the source, not this export. */\n:root {\n'+source.tokens.map(t=>`  --wk-${t.name}: ${value(t)};\n`).join('')+'}\n',
 'tokens.mini.scss':'// Generated final rpx for a 375 logical-px design. No second px conversion.\n'+source.tokens.map(t=>`$wk-${t.name}: ${value(t,true)};\n`).join(''),
 'tokens.ts':'// Generated; dimension values are logical CSS px, not physical device pixels.\nexport const tokens = '+JSON.stringify(Object.fromEntries(source.tokens.map(t=>[t.name,t.value])),null,2)+' as const;\nexport type TokenName = keyof typeof tokens;\n'
};
for(const [name,content] of Object.entries(outputs)){const path=resolve(root,name);if(process.argv.includes('--check')){if(readFileSync(path,'utf8')!==content)throw new Error(`Token export differs: ${name}`)}else writeFileSync(path,content)}
console.log(`${source.tokens.length} tokens: ${process.argv.includes('--check')?'exports match':'generated'}`);
