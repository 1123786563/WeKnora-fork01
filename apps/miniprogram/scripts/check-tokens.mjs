import {spawnSync} from 'node:child_process';
import {readFileSync} from 'node:fs';
const r=spawnSync(process.execPath,['scripts/build-tokens.mjs','--check'],{stdio:'inherit'});if(r.status!==0)process.exit(r.status??1);
const declared=new Set([...readFileSync('src/styles/tokens.mini.scss','utf8').matchAll(/\$(wk-[\w-]+):/g)].map(m=>m[1]));
const used=new Set([...readFileSync('src/app.scss','utf8').matchAll(/\$(wk-[\w-]+)/g)].map(m=>m[1]));
for(const name of used)if(!declared.has(name))throw new Error(`Unknown SCSS token: ${name}`);
if(/#[\da-f]{3,8}\b/i.test(readFileSync('src/app.scss','utf8')))throw new Error('Use checked-in tokens, not hard-coded product colors');
console.log(`${used.size} SCSS token references are valid`);
