import test from 'node:test';
import assert from 'node:assert/strict';
// 导入失败必须带出真实原因（语法/依赖错误），不允许吞掉后报“模块不存在”误导定位。
const mod=await import('../src/core/auth.ts');
const tenant=(id)=>({user:{id:'u1',username:'Lin'},tenant:{id,name:`Space ${id}`},memberships:[]});
function fixture(overrides={}){
 const map=new Map();const storage={read:k=>map.get(k),write:(k,v)=>map.set(k,v),remove:k=>map.delete(k)};
 let refreshes=0,token='t1';
 const api={login:async()=>({token,refreshToken:'r1',...tenant(1)}),me:async()=>tenant(1),
 refresh:async()=>{refreshes++;await new Promise(r=>setTimeout(r,5));return {access_token:'t2',refresh_token:'r2'}},
 switchTenant:async id=>({token:'t3',refreshToken:'r3',...tenant(id)}),logout:async()=>{},...overrides};
 const auth=new (mod.AuthCoordinator)('https://api.example.test',storage,api);
 return {auth,map,storage,get refreshes(){return refreshes}};
}
test('login exposes identity without exposing credentials to UI',async()=>{const {auth}=fixture();await auth.login('x@example.test','not-stored');assert.equal(auth.snapshot().phase,'ready');assert.equal(auth.snapshot().tenantId,'1');assert.equal(JSON.stringify(auth.snapshot()).includes('t1'),false);assert.equal(auth.credential().accessToken,'t1')});
test('concurrent refresh is single-flight and persists rotated credentials',async()=>{const f=fixture();await f.auth.login('x','pw');const scope=f.auth.scope.capture();await Promise.all([f.auth.refresh(scope),f.auth.refresh(scope)]);assert.equal(f.refreshes,1);assert.equal(f.auth.credential().accessToken,'t2')});
test('failed tenant switch retains original identity and unfreezes reads',async()=>{const {auth}=fixture({switchTenant:async()=>{throw new Error('forbidden')}});await auth.login('x','pw');await assert.rejects(auth.switchTenant(2));assert.equal(auth.snapshot().tenantId,'1');assert.equal(auth.snapshot().phase,'ready')});
test('refresh finishing after logout cannot resurrect a login',async()=>{let resolve;const wait=new Promise(r=>resolve=r);const {auth}=fixture({refresh:async()=>{await wait;return {access_token:'late',refresh_token:'late-r'}}});await auth.login('x','pw');const p=auth.refresh(auth.scope.capture());await auth.logout();resolve();await assert.rejects(p,/scope/i);assert.equal(auth.credential().kind,'anonymous');assert.equal(auth.snapshot().phase,'anonymous')});
test('switch aborts old requests and preserves user while replacing tenant',async()=>{const {auth}=fixture();await auth.login('x','pw');const controller=auth.scope.controller();const stamp=auth.scope.capture();await auth.switchTenant(2);assert.equal(controller.signal.aborted,true);assert.equal(auth.scope.isCurrent(stamp),false);assert.equal(auth.snapshot().tenantId,'2')});
test('login response completing after logout cannot restore identity',async()=>{let release;const gate=new Promise(r=>release=r);const {auth}=fixture({login:async()=>{await gate;return {token:'late',refreshToken:'late-r',...tenant(1)}}});const work=auth.login('x','pw');await auth.logout();release();await assert.rejects(work,/stale/i);assert.equal(auth.snapshot().phase,'anonymous');assert.equal(auth.credential().kind,'anonymous')});
test('tenant switch response completing after logout cannot restore identity',async()=>{let release;const gate=new Promise(r=>release=r);const {auth}=fixture({switchTenant:async()=>{await gate;return {token:'late',refreshToken:'late-r',...tenant(2)}}});await auth.login('x','pw');const work=auth.switchTenant(2);await auth.logout();release();await assert.rejects(work,/stale/i);assert.equal(auth.snapshot().phase,'anonymous');assert.equal(auth.credential().kind,'anonymous')});
test('bootstrap me response completing after logout is discarded',async()=>{let release,blocking=false;const gate=new Promise(r=>release=r);const {auth}=fixture({me:async()=>{if(blocking)await gate;return tenant(1)}});await auth.login('x','pw');blocking=true;const work=auth.bootstrap();await auth.logout();release();await work;assert.equal(auth.snapshot().phase,'anonymous');assert.equal(auth.credential().kind,'anonymous')});
test('definitive refresh rejection on the hot path clears the zombie session',async()=>{
 // 热路径 refresh 被服务端明确拒绝（refresh token 失效返回 401）后，
 // 会话不能停留在伪 ready：必须清除凭证并回到 anonymous，否则每次操作都无限 401。
 const unauthorized=Object.assign(new Error('refresh token expired'),{status:401});
 const {auth,map}=fixture({refresh:async()=>{await new Promise(r=>setTimeout(r,5));throw unauthorized}});
 await auth.login('x','pw');
 const key=[...map.keys()].find(k=>k.startsWith('wk:auth:'));
 assert.ok(key,'stored credential key exists');
 await assert.rejects(auth.refresh(auth.scope.capture()));
 assert.equal(auth.snapshot().phase,'anonymous','session must leave ready after definitive 401');
 assert.equal(auth.credential().kind,'anonymous','credentials must be cleared');
 assert.equal(map.has(key),false,'stored credential must be removed');
});
test('transient refresh failure keeps the session for later retry',async()=>{
 // 网络类失败不是凭证失效：不能把用户登出（会误伤在线用户），下一次仍可重试。
 const network=Object.assign(new Error('network down'),{code:'NETWORK_ERROR'});
 const {auth}=fixture({refresh:async()=>{throw network}});
 await auth.login('x','pw');
 await assert.rejects(auth.refresh(auth.scope.capture()));
 assert.equal(auth.snapshot().phase,'ready','network failure must not clear the session');
 assert.equal(auth.credential().accessToken,'t1');
});
test('D7: host-case variant of the same origin migrates into the canonical key on construct',async()=>{
 // URL host 大小写不敏感，但存储 key 是字符串：用不同大小写 origin 构建的两个包
 // 不能互相看见登录态。构造时必须把同 host 变体的旧 key 迁移到规范（小写）key，
 // 并删除变体 key，避免凭证残留本机。
 const map=new Map();
 map.set('wk:auth:https://WeKnora-App.ORB.local',{kind:'bearer',accessToken:'legacy',refreshToken:'legacy-r'});
 const storage={read:k=>map.get(k),write:(k,v)=>map.set(k,v),remove:k=>map.delete(k),keys:()=>[...map.keys()]};
 const api={me:async()=>tenant(1),refresh:async()=>({access_token:'t2',refresh_token:'r2'})};
 const auth=new mod.AuthCoordinator('https://WeKnora-App.ORB.local',storage,api);
 await auth.bootstrap();
 assert.equal(auth.snapshot().phase,'ready','migrated credential must restore the session');
 assert.equal(auth.credential().accessToken,'legacy');
 assert.deepEqual([...map.keys()],['wk:auth:https://weknora-app.orb.local'],'variant key must be removed after migration');
});
test('D7: logout clears the canonical key only, other-origin keys stay untouched',async()=>{
 const map=new Map();
 map.set('wk:auth:https://Up-WeKnora-app.orb.local',{kind:'bearer',accessToken:'other-box',refreshToken:'other-r'});
 const storage={read:k=>map.get(k),write:(k,v)=>map.set(k,v),remove:k=>map.delete(k),keys:()=>[...map.keys()]};
 const api={login:async()=>({token:'t1',refreshToken:'r1',...tenant(1)}),me:async()=>tenant(1),refresh:async()=>({access_token:'t2',refresh_token:'r2'}),logout:async()=>{}};
 const auth=new mod.AuthCoordinator('https://WeKnora-App.ORB.local',storage,api);
 await auth.login('x','pw');
 await auth.logout();
 assert.deepEqual([...map.keys()],['wk:auth:https://Up-WeKnora-app.orb.local'],'only same-host variants are normalized; different containers must be untouched');
});
test('D7: non-bearer garbage in a variant key is dropped, not migrated',async()=>{
 const map=new Map();
 map.set('wk:auth:https://WeKnora-App.ORB.local',{kind:'anonymous'});
 const storage={read:k=>map.get(k),write:(k,v)=>map.set(k,v),remove:k=>map.delete(k),keys:()=>[...map.keys()]};
 const api={me:async()=>tenant(1)};
 new mod.AuthCoordinator('https://weknora-app.orb.local',storage,api);
 assert.equal(map.size,0,'garbage variant key must be removed without writing it into the canonical key');
});
