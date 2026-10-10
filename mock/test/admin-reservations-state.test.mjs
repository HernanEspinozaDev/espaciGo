import test from 'node:test';
import assert from 'node:assert/strict';
import { AdminReservationsState } from '../../internal/mockserver/static/admin-reservations-state.js';

test('admin reservation response is discarded after logout and re-login, including same account',()=>{
  const state=new AdminReservationsState();
  const old=state.begin('admin','token-1',4);
  assert.equal(state.current(old,'admin','token-1',4),true);
  state.clear();
  assert.equal(state.current(old,'admin','token-1',5),false);
  const fresh=state.begin('admin','token-2',6);
  assert.equal(state.current(old,'admin','token-2',6),false);
  assert.equal(state.current(fresh,'admin','token-2',6),true);
});

test('only current successful request may restore pagination controls',()=>{
  const state=new AdminReservationsState();
  const request=state.begin('admin','token',1);
  state.nextCursor='next';
  assert.equal(state.finish(request,'admin','token',1),true);
  assert.equal(state.loading,false);
  state.clear();
  assert.equal(state.nextCursor,'');
  assert.equal(state.finish(request,'admin','token',1),false);
});

test('filter or page-size changes invalidate cursors, results request, and detail request',()=>{
  const state=new AdminReservationsState();
  assert.equal(state.updateCriteria('state=pagada&page_size=5'),true);
  const list=state.begin('admin','token',1);
  const detail=state.begin('admin','token',1);
  state.cursor='opaque-current';state.nextCursor='opaque-next';
  assert.equal(state.updateCriteria('state=aprobada_host&page_size=5'),true);
  assert.equal(state.current(list,'admin','token',1),false);
  assert.equal(state.current(detail,'admin','token',1),false);
  assert.equal(state.cursor,'');assert.equal(state.nextCursor,'');
  assert.equal(state.updateCriteria('state=aprobada_host&page_size=10'),true);
  assert.equal(state.nextCursor,'');
  const fresh=state.begin('admin','token',1);
  assert.equal(state.current(fresh,'admin','token',1),true);
});
