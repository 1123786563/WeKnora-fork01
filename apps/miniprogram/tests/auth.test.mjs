import test from 'node:test';
import assert from 'node:assert/strict';
let mod; try { mod=await import('../src/core/auth.ts'); } catch {}
const requireModule=()=>{assert.ok(mod,'AuthCoordinator must exist');return mod};
const tenant=(id)=>({user:{id:'u1',username:'Lin'},tenant:{id,name:`Space ${id}`},memberships:[]});
function fixture(overrides={}){
 const map=new Map();const storage={read:k=>map.get(k),write:(k,v)=>map.set(k,v),remove:k=>map.delete(k)};
 let refreshes=0,token='t1';
 const api={login:async()=>({token,refreshToken:'r1',...tenant(1)}),me:async()=>tenant(1),
 refresh:async()=>{refreshes++;await new Promise(r=>setTimeout(r,5));return {access_token:'t2',refresh_token:'r2'}},
 switchTenant:async id=>({token:'t3',refreshToken:'r3',...tenant(id)}),logout:async()=>{},...overrides};
 const auth=new (requireModule().AuthCoordinator)('https://api.example.test',storage,api);
 return {auth,storage,get refreshes(){return refreshes}};
}
test('login exposes identity without exposing credentials to UI',async()=>{const {auth}=fixture();await auth.login('x@example.test','not-stored');assert.equal(auth.snapshot().phase,'ready');assert.equal(auth.snapshot().tenantId,'1');assert.equal(JSON.stringify(auth.snapshot()).includes('t1'),false);assert.equal(auth.credential().accessToken,'t1')});
test('concurrent refresh is single-flight and persists rotated credentials',async()=>{const f=fixture();await f.auth.login('x','pw');const scope=f.auth.scope.capture();await Promise.all([f.auth.refresh(scope),f.auth.refresh(scope)]);assert.equal(f.refreshes,1);assert.equal(f.auth.credential().accessToken,'t2')});
test('failed tenant switch retains original identity and unfreezes reads',async()=>{const {auth}=fixture({switchTenant:async()=>{throw new Error('forbidden')}});await auth.login('x','pw');await assert.rejects(auth.switchTenant(2));assert.equal(auth.snapshot().tenantId,'1');assert.equal(auth.snapshot().phase,'ready')});
test('refresh finishing after logout cannot resurrect a login',async()=>{let resolve;const wait=new Promise(r=>resolve=r);const {auth}=fixture({refresh:async()=>{await wait;return {access_token:'late',refresh_token:'late-r'}}});await auth.login('x','pw');const p=auth.refresh(auth.scope.capture());await auth.logout();resolve();await assert.rejects(p,/scope/i);assert.equal(auth.credential().kind,'anonymous');assert.equal(auth.snapshot().phase,'anonymous')});
test('switch aborts old requests and preserves user while replacing tenant',async()=>{const {auth}=fixture();await auth.login('x','pw');const controller=auth.scope.controller();const stamp=auth.scope.capture();await auth.switchTenant(2);assert.equal(controller.signal.aborted,true);assert.equal(auth.scope.isCurrent(stamp),false);assert.equal(auth.snapshot().tenantId,'2')});
test('login response completing after logout cannot restore identity',async()=>{let release;const gate=new Promise(r=>release=r);const {auth}=fixture({login:async()=>{await gate;return {token:'late',refreshToken:'late-r',...tenant(1)}}});const work=auth.login('x','pw');await auth.logout();release();await assert.rejects(work,/stale/i);assert.equal(auth.snapshot().phase,'anonymous');assert.equal(auth.credential().kind,'anonymous')});
test('tenant switch response completing after logout cannot restore identity',async()=>{let release;const gate=new Promise(r=>release=r);const {auth}=fixture({switchTenant:async()=>{await gate;return {token:'late',refreshToken:'late-r',...tenant(2)}}});await auth.login('x','pw');const work=auth.switchTenant(2);await auth.logout();release();await assert.rejects(work,/stale/i);assert.equal(auth.snapshot().phase,'anonymous');assert.equal(auth.credential().kind,'anonymous')});
test('bootstrap me response completing after logout is discarded',async()=>{let release,blocking=false;const gate=new Promise(r=>release=r);const {auth}=fixture({me:async()=>{if(blocking)await gate;return tenant(1)}});await auth.login('x','pw');blocking=true;const work=auth.bootstrap();await auth.logout();release();await work;assert.equal(auth.snapshot().phase,'anonymous');assert.equal(auth.credential().kind,'anonymous')});
