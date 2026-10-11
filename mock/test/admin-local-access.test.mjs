import test from 'node:test';
import assert from 'node:assert/strict';
import { applyAdminLocalSessionSnapshot, captureAdminLocalContext, captureAdminLocalSession, handleAdminLocalForbidden, isAdminLocalForbiddenError } from '../../internal/mockserver/static/admin-local-state.js';

function sessionState(){return {account:'admin-a',token:'token-a',generation:4,roles:['administrador','arrendatario'],restrictedMode:false};}

test('admin request recognizes the shared 403 error message',()=>{
  assert.equal(isAdminLocalForbiddenError(new Error('Acceso restringido: denegado (HTTP 403).')),true);
  assert.equal(isAdminLocalForbiddenError(new Error('Denegado (HTTP 403, forbidden)')),true);
  assert.equal(isAdminLocalForbiddenError(new Error('No autenticado (HTTP 401, unauthenticated)')),false);
});

test('a current 403 clears the view and refreshes roles even after filters change',async()=>{
  const request=captureAdminLocalContext('admin-a','token-a',4,7),context=captureAdminLocalSession(request),current=sessionState();
  let revision=7,clears=0,refreshedToken='';
  revision++;
  const result=await handleAdminLocalForbidden(context,()=>current,()=>{clears++;revision++;},async token=>{refreshedToken=token;return{roles:['administrador','arrendatario'],restrictedMode:false};},snapshot=>{current.roles=snapshot.roles;current.restrictedMode=snapshot.restrictedMode;});
  // Revision represents filters/results and deliberately does not participate in authorization validity.
  assert.equal(result,'refreshed');assert.equal(clears,1);assert.equal(refreshedToken,'token-a');assert.equal(current.token,'token-a');assert.deepEqual(current.roles,['administrador','arrendatario']);
});

test('a session refresh that removes administrator role or confirms restricted mode clears the view',async t=>{
  for(const snapshot of [{roles:['arrendatario'],restrictedMode:false},{roles:['administrador','arrendatario'],restrictedMode:true}]){
    await t.test(JSON.stringify(snapshot),async()=>{
      const context=captureAdminLocalSession(captureAdminLocalContext('admin-a','token-a',4,7)),current=sessionState();let clears=0;
      const result=await handleAdminLocalForbidden(context,()=>current,()=>{clears++;},async()=>snapshot,value=>{current.roles=value.roles;current.restrictedMode=value.restrictedMode;});
      assert.equal(result,'refreshed');assert.equal(clears,2);assert.equal(current.token,'token-a');assert.deepEqual(current.roles,snapshot.roles);assert.equal(current.restrictedMode,snapshot.restrictedMode);
    });
  }
});

test('a delayed 403 from an old session cannot clear or refresh a new session',async()=>{
  const context=captureAdminLocalSession(captureAdminLocalContext('admin-a','old-token',4,7));let current=sessionState();let clears=0,applies=0,refreshes=0;
  current={account:'admin-a',token:'new-token',generation:6,roles:['administrador'],restrictedMode:false};
  const stale=await handleAdminLocalForbidden(context,()=>current,()=>{clears++;},async()=>{refreshes++;return{roles:['arrendatario'],restrictedMode:true};},snapshot=>{applies++;current.roles=snapshot.roles;current.restrictedMode=snapshot.restrictedMode;});
  assert.equal(stale,'stale');assert.equal(clears,0);assert.equal(refreshes,0);assert.equal(applies,0);assert.equal(current.token,'new-token');
});

test('a delayed session refresh after a current 403 cannot apply to a relogged session',async()=>{
  const context=captureAdminLocalSession(captureAdminLocalContext('admin-a','old-token',4,7));let current={...sessionState(),token:'old-token'};let resolveRefresh,clears=0,applies=0,refreshes=0;
  const delayedRefresh=new Promise(resolve=>{resolveRefresh=resolve;});
  const pending=handleAdminLocalForbidden(context,()=>current,()=>{clears++;},async()=>{refreshes++;return delayedRefresh;},snapshot=>{applies++;current.roles=snapshot.roles;current.restrictedMode=snapshot.restrictedMode;});
  current={account:'admin-a',token:'new-token',generation:6,roles:['administrador'],restrictedMode:false};
  resolveRefresh({roles:['arrendatario'],restrictedMode:true});
  assert.equal(await pending,'stale');assert.equal(clears,1);assert.equal(refreshes,1);assert.equal(applies,1);assert.equal(current.token,'new-token');assert.deepEqual(current.roles,['administrador']);assert.equal(current.restrictedMode,false);
});

test('manual session query clears administrative view when the API removes the administrator role, preserving the session',async()=>{
  const current=sessionState(),context=captureAdminLocalSession(captureAdminLocalContext(current.account,current.token,current.generation,12));
  let viewRevision=12,clears=0,sessionApplies=0;
  const applied=applyAdminLocalSessionSnapshot(context,()=>current,{roles:['arrendatario'],restrictedMode:false},snapshot=>{sessionApplies++;current.roles=snapshot.roles;current.restrictedMode=snapshot.restrictedMode;},()=>{clears++;viewRevision++;});
  assert.equal(applied,true);assert.equal(sessionApplies,1);assert.equal(clears,1);assert.equal(viewRevision,13);
  assert.equal(current.account,'admin-a');assert.equal(current.token,'token-a');assert.deepEqual(current.roles,['arrendatario']);assert.equal(current.restrictedMode,false);
});

test('manual session query clears administrative view when the API confirms restricted mode, preserving allowed session',async()=>{
  const current=sessionState(),context=captureAdminLocalSession(captureAdminLocalContext(current.account,current.token,current.generation,2));
  let clears=0;
  const applied=applyAdminLocalSessionSnapshot(context,()=>current,{roles:['administrador','arrendatario'],restrictedMode:true},snapshot=>{current.roles=snapshot.roles;current.restrictedMode=snapshot.restrictedMode;},()=>{clears++;});
  assert.equal(applied,true);assert.equal(clears,1);assert.equal(current.account,'admin-a');assert.equal(current.token,'token-a');
  assert.deepEqual(current.roles,['administrador','arrendatario']);assert.equal(current.restrictedMode,true);
});

test('manual session response arriving after logout and login cannot alter the new session',async()=>{
  const before=sessionState(),context=captureAdminLocalSession(captureAdminLocalContext(before.account,before.token,before.generation,5));
  let current={...before},resolveResponse,applies=0,clears=0;
  const delayedResponse=new Promise(resolve=>{resolveResponse=resolve;});
  const pending=delayedResponse.then(snapshot=>applyAdminLocalSessionSnapshot(context,()=>current,snapshot,value=>{applies++;current.roles=value.roles;current.restrictedMode=value.restrictedMode;},()=>{clears++;}));
  // Same account, new token and generation after logout/login.
  current={account:'admin-a',token:'replacement-token',generation:6,roles:['administrador'],restrictedMode:false};
  resolveResponse({roles:['arrendatario'],restrictedMode:true});
  assert.equal(await pending,false);assert.equal(applies,0);assert.equal(clears,0);
  assert.equal(current.token,'replacement-token');assert.equal(current.generation,6);assert.deepEqual(current.roles,['administrador']);assert.equal(current.restrictedMode,false);
});
