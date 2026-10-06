import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";

const compiled = await readFile(new URL("../../internal/mockserver/static/booking-quote-state.js", import.meta.url), "utf8");
const { BookingQuoteState } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);

test("a quote for space A cannot be requested after selecting space B", () => {
  const state = new BookingQuoteState();
  const selectionA = state.beginSelection("space-A");
  const quoteA = state.beginQuote("space-A");
  assert.equal(quoteA, selectionA);
  assert.equal(state.acceptQuote(quoteA, "space-A", "quote-A"), true);

  state.beginSelection("space-B");
  assert.equal(state.canRequest("quote-A", "space-B"), false);
  assert.equal(state.canRequest("quote-A", "space-A"), false);
});

test("a late quote response is discarded after the selected space changes", () => {
  const state = new BookingQuoteState();
  state.beginSelection("space-A");
  const pendingQuote = state.beginQuote("space-A");
  assert.notEqual(pendingQuote, null);

  state.beginSelection("space-B");
  assert.equal(state.acceptQuote(pendingQuote, "space-A", "late-quote-A"), false);
  assert.equal(state.canRequest("late-quote-A", "space-B"), false);
});

test("starting another search invalidates the prepared quote", () => {
  const state = new BookingQuoteState();
  const selection = state.beginSelection("space-A");
  assert.equal(state.acceptQuote(selection, "space-A", "quote-A"), true);
  state.beginSearch();
  assert.equal(state.canRequest("quote-A", null), false);
});
