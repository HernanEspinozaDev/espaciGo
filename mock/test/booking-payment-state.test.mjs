import assert from "node:assert/strict";
import test from "node:test";
import { BookingPaymentState, BookingRequestState, executePaymentAttempt, paymentPanelAfterError } from "../../internal/mockserver/static/booking-payment-state.js";

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

test("after a timeout, a refreshed paid state replaces the uncertain message and clears the retry attempt", async () => {
  const payments=new BookingPaymentState();
  const failed=await executePaymentAttempt(payments,"reservation-paid","exito",()=>"paid-key",async()=>{throw new Error("HTTP 504 simulated_payment_timeout");});
  assert.equal(failed.status,"uncertain");
  assert.equal(payments.get("reservation-paid").idempotencyKey,"paid-key");

  const panel=paymentPanelAfterError("pagada",payments.get("reservation-paid")!==null);
  if(panel.clearAttempt)payments.clearCompleted("reservation-paid");
  assert.match(panel.message,/API confirmó el pago fake del arriendo/);
  assert.match(panel.message,/autorización de garantía por separado/);
  assert.equal(panel.buttonLabel,"Enviar pago de ensayo");
  assert.equal(payments.get("reservation-paid"),null);
});

test("after a payment error, a refreshed expired state replaces uncertainty and clears the retry attempt", async () => {
  const payments=new BookingPaymentState();
  const failed=await executePaymentAttempt(payments,"reservation-expired","exito",()=>"expired-key",async()=>{throw new Error("HTTP 503 connection error");});
  assert.equal(failed.status,"uncertain");
  assert.equal(payments.get("reservation-expired").idempotencyKey,"expired-key");

  const panel=paymentPanelAfterError("vencida_pago",payments.get("reservation-expired")!==null);
  if(panel.clearAttempt)payments.clearCompleted("reservation-expired");
  assert.match(panel.message,/La API confirmó que venció el plazo de pago/);
  assert.equal(panel.buttonLabel,"Enviar pago de ensayo");
  assert.equal(payments.get("reservation-expired"),null);
});

test("an updated pending state keeps the same retry key and uncertain action", async () => {
  const payments=new BookingPaymentState();
  await executePaymentAttempt(payments,"reservation-pending","exito",()=>"pending-key",async()=>{throw new Error("HTTP 504 timeout");});
  const panel=paymentPanelAfterError("pendiente_de_pago",payments.get("reservation-pending")!==null);
  assert.equal(panel.clearAttempt,false);
  assert.equal(panel.buttonLabel,"Consultar / reintentar pago (misma clave)");
  assert.match(panel.message,/continúa pendiente/);
  assert.equal(payments.get("reservation-pending").idempotencyKey,"pending-key");
});

test("a failed status refresh reports uncertainty without clearing the existing attempt", () => {
  const payments=new BookingPaymentState();
  payments.begin("reservation-unavailable","exito",()=>"unchanged-key");
  payments.finish("reservation-unavailable","uncertain");
  const panel=paymentPanelAfterError(null,payments.get("reservation-unavailable")!==null);
  assert.match(panel.message,/No se pudo actualizar el estado/);
  assert.equal(panel.buttonLabel,"Consultar / reintentar pago (misma clave)");
  assert.equal(panel.clearAttempt,false);
  assert.equal(payments.get("reservation-unavailable").idempotencyKey,"unchanged-key");
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
