import assert from "node:assert/strict";
import test from "node:test";
import { BookingPaymentState, BookingRequestState, executePaymentAttempt } from "../../internal/mockserver/static/booking-payment-state.js";

test("timeout retries reuse both idempotency key and outcome, and block a concurrent send", () => {
  const payments = new BookingPaymentState();
  let generated = 0;
  const first = payments.begin("reservation-a", "sin_respuesta", () => `key-${++generated}`);
  assert.deepEqual(first, {reservationID:"reservation-a",idempotencyKey:"key-1",outcome:"sin_respuesta"});
  assert.equal(payments.begin("reservation-a", "exito", () => `key-${++generated}`), null);

  payments.finish("reservation-a", "uncertain");
  const retry = payments.begin("reservation-a", "exito", () => `key-${++generated}`);
  assert.deepEqual(retry, first, "a timeout keeps the original key and request body");
  assert.equal(retry.idempotencyKey, "key-1");
  assert.equal(retry.outcome, "sin_respuesta");
  assert.equal(generated, 1);
});

test("separate reservations receive separate attempts; a completed payment clears its local retry state", () => {
  const payments = new BookingPaymentState();
  const a = payments.begin("reservation-a", "exito", () => "key-a");
  payments.finish("reservation-a", "completed");
  const b = payments.begin("reservation-b", "rechazo", () => "key-b");
  assert.equal(a.idempotencyKey, "key-a");
  assert.equal(b.idempotencyKey, "key-b");
  assert.equal(payments.get("reservation-a"), null);
  assert.equal(payments.get("reservation-b").outcome, "rechazo");
});

test("responses are discarded after changing reservation, account, or session", () => {
  const state = new BookingRequestState();
  state.select("reservation-a");
  const first = state.capture("account-a", "session-a");
  assert.equal(state.accepts(first, "account-a", "session-a"), true);

  state.select("reservation-b");
  assert.equal(state.accepts(first, "account-a", "session-a"), false);
  const second = state.capture("account-a", "session-a");
  assert.equal(state.accepts(second, "account-b", "session-b"), false);
  assert.equal(state.accepts(second, "account-a", "session-b"), false);

  state.invalidate();
  assert.equal(state.accepts(second, "account-a", "session-a"), false);
});

test("a timeout followed by reconciliation retries the same fake operation and refreshes with its result", async () => {
  const payments = new BookingPaymentState();
  const requests = [];
  let fakeStarts = 0;
  let persistedState = "pendiente_de_pago";
  const first = await executePaymentAttempt(payments,"reservation-timeout","exito",()=>"stable-key",async attempt=>{
    requests.push(attempt);
    fakeStarts++;
    throw new Error("HTTP 504 simulated_payment_timeout");
  });
  assert.equal(first.status,"uncertain");
  assert.equal(payments.get("reservation-timeout").idempotencyKey,"stable-key");

  // The Backend reconciler applies the already-persisted fake result between attempts.
  persistedState="pagada";
  const retry = await executePaymentAttempt(payments,"reservation-timeout","rechazo",()=>"must-not-be-used",async attempt=>{
    requests.push(attempt);
    assert.equal(attempt.idempotencyKey,"stable-key");
    assert.equal(attempt.outcome,"exito");
    return {state:persistedState};
  });
  assert.equal(retry.status,"completed");
  assert.equal(retry.value.state,"pagada");
  assert.equal(requests.length,2);
  assert.equal(fakeStarts,1,"the retry queries the same operation; it does not start another fake payment");
  assert.equal(payments.get("reservation-timeout"),null);
});

test("concurrent clicks produce one request and the second one is ignored", async () => {
  const payments = new BookingPaymentState();
  let calls=0;
  let release;
  const waiting=new Promise(resolve=>{release=resolve;});
  const first=executePaymentAttempt(payments,"reservation-a","exito",()=>"one-key",async()=>{calls++;await waiting;return "pagada";});
  const second=await executePaymentAttempt(payments,"reservation-a","exito",()=>"second-key",async()=>{calls++;return "incorrect";});
  assert.deepEqual(second,{status:"busy"});
  assert.equal(calls,1);
  release();
  assert.equal((await first).status,"completed");
});
