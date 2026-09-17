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
  sourceRoot:'src',outputRoot:'dist',framework:'react',
  // prebundle 的 remoteEntry 依赖注入与 app.json 的 lazyCodeLoading:requiredComponents
  // 冲突：页面 chunk 不被加载，全部页面 "has not been registered yet" 白屏。关闭 prebundle。
  compiler:{type:'webpack5',prebundle:{enable:false}},
  plugins:['@tarojs/plugin-platform-weapp'],
  defineConstants:{__API_ORIGIN__:JSON.stringify(origin)},
  // watch 构建本身很快；显式关闭持久化缓存，避免规则调整后坏模块被缓存复活。
  cache:{enable:false},
  mini:{
    postcss:{pxtransform:{enable:true,config:{}},url:{enable:true,config:{limit:1024}}},
    // Workspace 包以 .ts 源码直出（exports 指向 src/*.ts），真实路径在仓库 packages/ 下，
    // 不命中 Taro script rule 默认 include（sourceDir + *taro*）。用单个判定函数接管：
    // 仓库内 packages/ 目录、小程序自身 src/ 目录、以及 node_modules 里路径含 *taro* 的文件。
    // 注意不要用 path.resolve 拼 include 前缀——Taro 转译执行本配置时相对基准不可靠。
    webpackChain(chain){
      chain.module.rule('script').include.clear()
        .add((filename:string)=>{
          return filename.includes('/packages/')
            || filename.includes('\\packages\\')
            || /(^|[\\/])src[\\/]/.test(filename)
            || /(?<=node_modules[\\/]).*taro/.test(filename);
        });
    },
  },
});
