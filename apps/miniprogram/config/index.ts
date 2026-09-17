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
  // watch 构建本身很快；显式关闭持久化缓存，避免规则调整后坏模块被缓存复活。
  cache:{enable:false},
  mini:{
    postcss:{pxtransform:{enable:true,config:{}},url:{enable:true,config:{limit:1024}}},
    // Workspace 包以 .ts 源码直出（exports 指向 src/*.ts），真实路径在仓库 packages/ 下，
    // 不命中 Taro script rule 默认 include（sourceDir + *taro*）。注意：Taro 转译执行本配置时
    // __dirname 指向临时产物目录，路径必须基于 process.cwd()（即 apps/miniprogram）计算。
    webpackChain(chain){
      const cwd=process.cwd();
      chain.module.rule('script').include.clear().add([
        resolve(cwd,'src'),
        resolve(cwd,'../packages'),
        (filename:string)=>/(?<=node_modules[\\/]).*taro/.test(filename),
      ]);
    },
  },
});
