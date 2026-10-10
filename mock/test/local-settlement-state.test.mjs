import assert from "node:assert/strict";
import test from "node:test";
import { LocalSettlementState } from "../../internal/mockserver/static/local-settlement-state.js";

test("uncertain timeout keeps the same settlement key and fake result",()=>{
  const state=new LocalSettlementState();let generated=0;
  const first=state.begin("reservation-a","sin_respuesta",()=>`key-${++generated}`);
  assert.equal(first.idempotencyKey,"key-1");
  assert.equal(state.begin("reservation-a","exito",()=>`bad-${++generated}`),null);
  state.finish("reservation-a",false);
  const retry=state.begin("reservation-a","exito",()=>`bad-${++generated}`);
  assert.deepEqual(retry,first);assert.equal(generated,1);
});

test("only one request may run and a confirmed result clears its attempt",()=>{
  const state=new LocalSettlementState();
  const first=state.begin("reservation-a","exito",()=>"same");
  assert.equal(state.begin("reservation-a","exito",()=>"other"),null);
  state.finish("reservation-a",true);
  assert.equal(state.get("reservation-a"),null);
  assert.equal(first.idempotencyKey,"same");
});

test("late response is rejected after selection or session changes, including same-account relogin",()=>{
  const state=new LocalSettlementState();
  const context=state.capture("reservation-a","account-a","token-a",4);
  const same={reservationID:"reservation-a",accountID:"account-a",token:"token-a",generation:4};
  assert.equal(state.accepts(context,same),true);
  assert.equal(state.accepts(context,{...same,reservationID:"reservation-b"}),false);
  assert.equal(state.accepts(context,{...same,token:"token-b",generation:5}),false);
  state.clear();
  assert.equal(state.accepts(context,same),false);
});
