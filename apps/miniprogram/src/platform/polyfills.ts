import 'core-js/actual/url';
import 'core-js/actual/url-search-params';
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
