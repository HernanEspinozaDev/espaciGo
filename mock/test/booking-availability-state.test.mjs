import test from "node:test";
import assert from "node:assert/strict";
import { BookingAvailabilityState } from "../../internal/mockserver/static/booking-availability-state.js";

const first = { spaceID: "space-a", date: "2030-01-02", duration: "2", accountID: "renter-a", sessionToken: "session-a" };
const option = { startAt: "2030-01-02T13:00:00Z", endAt: "2030-01-02T15:00:00Z", startLocal: "2030-01-02T10:00", endLocal: "2030-01-02T12:00" };

test("accepts only the currently selected fixture/date/duration/session response", () => {
  const state = new BookingAvailabilityState();
  const token = state.beginRequest(first);
  assert.equal(state.accepts(token, first), true);
  assert.equal(state.accepts(token, { ...first, spaceID: "space-b" }), false);
  assert.equal(state.accepts(token, { ...first, date: "2030-01-03" }), false);
  assert.equal(state.accepts(token, { ...first, duration: "4" }), false);
  assert.equal(state.accepts(token, { ...first, sessionToken: "session-b" }), false);
});

test("a changed selection invalidates late replies and prior interval", () => {
  const state = new BookingAvailabilityState();
  const token = state.beginRequest(first);
  state.invalidate();
  assert.equal(state.accepts(token, first), false);
  assert.equal(state.selectedFor(first, option.startLocal, option.endLocal), null);
});

test("only a returned interval matching the selected context may fill quote", () => {
  const state = new BookingAvailabilityState();
  const token = state.beginRequest(first);
  assert.equal(state.select(token, first, option), true);
  assert.deepEqual(state.selectedFor(first, option.startLocal, option.endLocal), option);
  assert.equal(state.selectedFor(first, "2030-01-02T10:30", option.endLocal), null);
  assert.equal(state.selectedFor({ ...first, date: "2030-01-03" }, option.startLocal, option.endLocal), null);
});
