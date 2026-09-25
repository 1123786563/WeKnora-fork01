// assembly 与 office-assembly 共用的测试脚手架（最终审查修复 F5：两文件 ~40 行近逐字重复
// 收敛到单一事实源）。承载：@tarojs/taro 平台替身 hook、me/capabilities 身份权威、
// 按 method+pathname 路由的假后端、登录态前置（login → me → capabilities）与 until 轮询。
// 场景级数据（runRow/overviewRows/execDto 等）不属于脚手架，留在各自测试文件。
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

/** 安装 @tarojs/taro 平台替身并加载真实 src 模块；必须在任何 src 导入之前 await 调用。 */
export async function loadAssemblyHarness({ withOffice = false } = {}) {
  const stubURL = pathToFileURL(new URL('./taro-stub.mjs', import.meta.url).pathname).href;
  registerHooks({
    resolve(specifier, context, nextResolve) {
      if (specifier === '@tarojs/taro') return { url: stubURL, shortCircuit: true };
      return nextResolve(specifier, context);
    },
  });
  globalThis.__API_ORIGIN__ = 'https://api.example.test';
  const stubModule = await import('./taro-stub.mjs');
  const runtime = await import('../../src/services/runtime.ts');
  const office = withOffice ? await import('../../src/services/mobile-office.ts') : undefined;
  const harness = createAssemblyHarness(stubModule.stub, runtime);
  return { stub: stubModule.stub, runtime, office, harness, ...harness };
}

export function createAssemblyHarness(stub, runtime) {
  // activateTenant 的身份来自切换后的 me()（mobile-runtime.ts：switch-tenant → persist → authenticate），
  // 因此 me() 必须是有状态替身：activeTenant 随 switch-tenant 路由翻转。
  let activeTenant = 1;
  const me = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: activeTenant, name: activeTenant === 1 ? 'Space' : 'Space 2' }, memberships: [
    { tenant_id: 1, tenant_name: 'Space', role: 'owner' },
    { tenant_id: 2, tenant_name: 'Space 2', role: '成员' },
  ] } });
  // 与真实 wire 一致：GET /system/capabilities 返回标准 code/msg/data 信封（deployment_capabilities.go:151-155，
  // createMobileRuntimeRemote.deploymentCapabilities 按 root.code===0 解包）——非 auth 域的 {success,data} 信封。
  const capabilities = () => ({ code: 0, msg: 'success', data: { protocol_minimum: 1, protocol_maximum: 5 } });
  const settle = ms => new Promise(resolve => setTimeout(resolve, ms ?? 30));
  async function until(predicate, ms = 1500) { const end = Date.now() + ms; while (Date.now() < end) { if (predicate()) return true; await settle(10); } return predicate(); }

  /** 安装按 method+pathname 路由的假后端；route 未命中时挂起（SSE/分块）或 fail（等测试手动响应）。 */
  function backend(routes) {
    stub.use(call => {
      // downloadFile/uploadFile 等 native 调用不携带 method 字段：按微信语义补全（GET/POST）。
      const method = call.options.method ?? (call.kind === 'uploadFile' ? 'POST' : 'GET');
      const path = decodeURIComponent(new URL(call.options.url).pathname);
      const exact = routes[`${method} ${path}`];
      let fn = exact;
      if (fn === undefined) {
        // 以 '/' 结尾的键按前缀匹配，承载 :run_id / :request_id 路径参数。
        const prefix = Object.keys(routes).filter(k => k.endsWith('/') && `${method} ${path}`.startsWith(k)).sort((a, b) => b.length - a.length)[0];
        if (prefix) fn = routes[prefix];
      }
      if (fn === undefined && call.options.enableChunked) {
        // SSE 路由未命中时挂起等待测试驱动（真实 SSE 是长连接）。
        return;
      }
      if (fn === undefined) { call.options.fail({ errMsg: `no backend route for ${method} ${path}` }); return; }
      fn(call);
    });
  }
  /** 登录态前置路由：login → me → capabilities（Runtime authenticate 的真实三步）。 */
  function authRoutes(extra = {}) {
    return {
      'POST /api/v1/auth/login': call => stub.succeed(call, { data: { success: true, data: { token: 't1', refresh_token: 'r1' } } }),
      'GET /api/v1/auth/me': call => stub.succeed(call, { data: me() }),
      'GET /api/v1/system/capabilities': call => stub.succeed(call, { data: capabilities() }),
      ...extra,
    };
  }
  async function freshLogin(extraRoutes = {}) {
    // 注意：stub.reset() 会连 storage 一起清空，而 MobileRuntime 每次授权请求都从凭据仓现读
    // （sendWithCredential → credentialStore.read）——登录后绝不能再 reset，只按需重装路由 handler。
    stub.reset();
    backend(authRoutes(extraRoutes));
    await runtime.auth.login('u@example.test', 'pw');
  }
  const authorization = call => call.options.header.Authorization ?? call.options.header.authorization;
  const authKeys = () => [...stub.state.storage.keys()].filter(k => k.startsWith('wk:auth:'));
  return {
    me, capabilities, settle, until, backend, authRoutes, freshLogin, authorization, authKeys,
    setActiveTenant(value) { activeTenant = value; },
    get activeTenant() { return activeTenant; },
  };
}
