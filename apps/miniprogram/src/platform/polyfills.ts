// 目录形式 'core-js/actual/url' 只有 bundler 能解析；Node ESM（tests 直接 import 源码）
// 会 Directory import 失败，必须带 index.js 才能在两种环境同时解析。
import 'core-js/actual/url/index.js';
import 'core-js/actual/url-search-params/index.js';
// 小程序逻辑层没有原生 AbortController，而 abort-controller 包的 browser 构建依赖
// self 全局（逻辑层不存在）在模块求值时即崩溃，不可用。这里内联最小实现，只需覆盖
// 项目实际用法：signal.aborted / addEventListener('abort', cb, {once}) / abort()。
class MiniAbortSignal {
  aborted=false;
  listeners:Array<()=>void>=[];
  addEventListener(_type:'abort',cb:()=>void,_options?:{once?:boolean}):void{
    if(this.aborted){cb();return}
    this.listeners.push(cb);
  }
  removeEventListener(_type:'abort',cb:()=>void):void{
    const i=this.listeners.indexOf(cb);
    if(i!==-1)this.listeners.splice(i,1);
  }
}
class MiniAbortController {
  signal=new MiniAbortSignal();
  abort():void{
    if(this.signal.aborted)return;
    this.signal.aborted=true;
    const pending=this.signal.listeners;
    this.signal.listeners=[];
    for(const cb of pending)cb();
  }
}
if(typeof globalThis.AbortController==='undefined'){
  globalThis.AbortController=MiniAbortController as unknown as typeof AbortController;
}
