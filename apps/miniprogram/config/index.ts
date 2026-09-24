import { defineConfig } from '@tarojs/cli';
import { existsSync, realpathSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
const origin=(process.env.WEKNORA_API_ORIGIN??'').replace(/\/+$/,'');
// 生产强制固定 HTTPS origin；本地回环的 http（localhost/127.0.0.1 带端口）仅用于开发
// 联调——模拟器的 Chromium 对 *.orb.local 走 mDNS 解析不可靠，本地联调须用回环地址。
const LOCAL_HTTP_ORIGIN=/^http:\/\/(localhost|127\.0\.0\.1)(:\d+)?$/;
if(origin && !LOCAL_HTTP_ORIGIN.test(origin) && !/^https:\/\/[^/?#]+$/.test(origin))throw new Error('WEKNORA_API_ORIGIN must be a fixed HTTPS origin (or loopback http for local dev) without /api/v1');
const appid=process.env.WEKNORA_WEAPP_APPID??'touristappid';
// Public build metadata; credentials belong exclusively on the Go server.
writeFileSync(resolve(process.cwd(),'project.config.json'),JSON.stringify({miniprogramRoot:'dist/',projectname:'weknora-miniprogram',appid,compileType:'miniprogram',setting:{es6:true,minified:true,urlCheck:true}},null,2));
// TDesign Miniprogram 以“预编译原生组件”方式接入（Spec：小程序使用 TDesign Miniprogram）。
// 不能用 npm 包名引用（usingComponents: 'tdesign-miniprogram/button/button'）：Taro 会把它
// 解析成 node_modules 绝对路径并重写为 /npm/.pnpm/.../node_modules/...——DevTools 拒绝任何
// 含 node_modules 段的组件路径且 dist 从不产出 dist/npm（task-6-live-validation D1）。
// 因此这里把 tdesign-miniprogram 的 miniprogram_dist 原样拷入 dist/npm/tdesign，页面
// usingComponents 直接引用 dist 根的绝对路径 /npm/tdesign/button/button。
// realpath 解开 pnpm 符号链接，避免 copy 依据变化。
const tdesignDist=realpathSync(resolve(process.cwd(),'node_modules/tdesign-miniprogram/miniprogram_dist'));
if(!existsSync(resolve(tdesignDist,'button/button.js')))throw new Error(`tdesign-miniprogram miniprogram_dist not found at ${tdesignDist} — run pnpm install first`);
// Taro 4.2.1 的 IMiniAppConfig 类型声明缺 copy 字段（运行时支持：MiniWebpackPlugin
// getCopyWebpackPlugin 消费 config.copy）。用展开注入，绕开对象字面量的多余属性检查。
const withTdesignCopy={copy:{patterns:[{from:tdesignDist,to:'dist/npm/tdesign'}],options:{}}};
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
    // 见上方 tdesignDist 注释：官方构建必须产出 dist/npm/tdesign（D1 修复的另一半在
    // src/subpackages/execution/artifact/index.config.ts 的 /npm/tdesign/... 引用）。
    ...withTdesignCopy,
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
