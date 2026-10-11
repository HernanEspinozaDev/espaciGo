import test from "node:test";
import assert from "node:assert/strict";
import { reviewableReservationState } from "../../internal/mockserver/static/review-eligibility.js";

test("reviews are available to participants after finalization or confirmed settlement close", () => {
  assert.equal(reviewableReservationState("finalizada"), true);
  assert.equal(reviewableReservationState("cerrada"), true);
  for (const state of ["en_disputa", "en_curso", "cancelada_arrendatario", undefined]) {
    assert.equal(reviewableReservationState(state), false, `${state} must not permit a new review`);
  }
});
