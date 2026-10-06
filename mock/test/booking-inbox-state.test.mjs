import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";

const compiled = await readFile(new URL("../../internal/mockserver/static/booking-inbox-state.js", import.meta.url), "utf8");
const { inboxActions } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);
const now = Date.parse("2030-01-01T12:00:00Z");
const reservation = {host_id:"host",renter_id:"renter",state:"pendiente_de_pago",start_at:"2030-01-03T12:00:00Z",pay_expires_at:"2030-01-01T12:15:00Z",host_expires_at:null};

test("renter can pay or cancel a pending reservation; host and unrelated account cannot", () => {
  assert.deepEqual(inboxActions("renter", reservation, now), {isRenter:true,isHost:false,canPay:true,canCancel:true,canDecide:false,awaitsHostDecision:false});
  assert.equal(inboxActions("host", reservation, now).canPay, false);
  assert.equal(inboxActions("third-party", reservation, now).canCancel, false);
});

test("only the host can decide a paid reservation before its deadline", () => {
  const paid = {...reservation,state:"pagada",host_expires_at:"2030-01-02T12:00:00Z"};
  assert.equal(inboxActions("host", paid, now).canDecide, true);
  assert.equal(inboxActions("renter", paid, now).canDecide, false);
  assert.equal(inboxActions("third-party", paid, now).awaitsHostDecision, false);
  assert.equal(inboxActions("host", paid, Date.parse(paid.host_expires_at)).canDecide, false);
});

test("renter may request cancellation of paid or approved bookings only before start", () => {
  for (const state of ["pagada", "aprobada_host"]) {
    const item = {...reservation,state};
    assert.equal(inboxActions("renter", item, now).canCancel, true);
    assert.equal(inboxActions("host", item, now).canCancel, false);
    assert.equal(inboxActions("renter", item, Date.parse(item.start_at)).canCancel, false);
  }
});

test("terminal and expired states expose no actions", () => {
  for (const state of ["cancelada_por_pago","rechazada_arrendador","vencida_pago","vencida_host","cancelada_arrendatario"]) {
    const item = {...reservation,state};
    const actions = inboxActions("renter", item, now);
    assert.equal(actions.canPay, false, `${state} must not allow payment`);
    assert.equal(actions.canCancel, false, `${state} must not allow cancellation`);
    assert.equal(actions.canDecide, false, `${state} must not allow a decision`);
  }
  assert.equal(inboxActions("renter", {...reservation,pay_expires_at:"2030-01-01T12:00:00Z"}, now).canPay, false);
});
