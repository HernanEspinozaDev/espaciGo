import test from 'node:test';
import assert from 'node:assert/strict';
import { adminLocalContextMatches, captureAdminLocalContext } from '../../internal/mockserver/static/admin-local-state.js';

test('admin response is discarded after criteria or session changes',()=>{
  const context=captureAdminLocalContext('a','token-1',1,7);
  assert.equal(adminLocalContextMatches(context,{account:'a',token:'token-1',generation:1,revision:7,isAdmin:true}),true);
  assert.equal(adminLocalContextMatches(context,{account:'a',token:'token-1',generation:1,revision:8,isAdmin:true}),false);
  assert.equal(adminLocalContextMatches(context,{account:'a',token:'token-2',generation:2,revision:7,isAdmin:true}),false);
  assert.equal(adminLocalContextMatches(context,{account:'b',token:'token-3',generation:3,revision:7,isAdmin:true}),false);
  assert.equal(adminLocalContextMatches(context,{account:'a',token:'token-1',generation:1,revision:7,isAdmin:false}),false);
});
