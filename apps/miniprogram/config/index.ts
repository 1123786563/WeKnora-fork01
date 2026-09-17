import { defineConfig } from '@tarojs/cli';
import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
const origin=(process.env.WEKNORA_API_ORIGIN??'').replace(/\/+$/,'');
if(origin && !/^https:\/\/[^/?#]+$/.test(origin))throw new Error('WEKNORA_API_ORIGIN must be a fixed HTTPS origin without /api/v1');
const appid=process.env.WEKNORA_WEAPP_APPID??'touristappid';
// Public build metadata; credentials belong exclusively on the Go server.
writeFileSync(resolve(process.cwd(),'project.config.json'),JSON.stringify({miniprogramRoot:'dist/',projectname:'weknora-miniprogram',appid,compileType:'miniprogram',setting:{es6:true,minified:true,urlCheck:true}},null,2));
export default defineConfig({
  projectName:'weknora-miniprogram',date:'2026-09-17',designWidth:375,deviceRatio:{375:2},
  sourceRoot:'src',outputRoot:'dist',framework:'react',compiler:'webpack5',
  plugins:['@tarojs/plugin-platform-weapp'],
  defineConstants:{__API_ORIGIN__:JSON.stringify(origin)},
  mini:{postcss:{pxtransform:{enable:true,config:{}},url:{enable:true,config:{limit:1024}}}},
});
