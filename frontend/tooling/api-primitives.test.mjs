import assert from "node:assert/strict";
import test from "node:test";
import { clpAmount, addClp, formatClp } from "../src/lib/api/clp.ts";
import { authErrorMessage, normalizeApiError } from "../src/lib/api/errors.ts";
import { serializeExactJson } from "../src/lib/api/exact-json.ts";
import { createMutationIntentCore } from "../src/lib/api/mutation-intent-core.mjs";
import { OpaqueCursorPager } from "../src/lib/api/opaque-cursor.ts";
import {
  assertIanaTimeZone,
  formatInTimeZone,
  parseRfc3339,
} from "../src/lib/api/time.ts";

function offsetMinutesAt(timestamp, timeZone) {
  const name = new Intl.DateTimeFormat("en-US", {
    timeZone,
    timeZoneName: "shortOffset",
  })
    .formatToParts(new Date(timestamp))
    .find((part) => part.type === "timeZoneName")?.value;
  const match = /^GMT(?:([+-])(\d{1,2})(?::(\d{2}))?)?$/.exec(name ?? "");
  assert.ok(match, "runtime must expose an IANA UTC offset");
  if (!match[1]) return 0;
  const minutes = Number(match[2]) * 60 + Number(match[3] ?? 0);
  return match[1] === "+" ? minutes : -minutes;
}

function transitionsInYear(year, timeZone) {
  const start = Date.UTC(year, 0, 1);
  const end = Date.UTC(year + 1, 0, 1);
  const step = 6 * 60 * 60 * 1000;
  const transitions = [];
  let previous = start;
  let previousOffset = offsetMinutesAt(previous, timeZone);

  for (let current = start + step; current <= end; current += step) {
    const currentOffset = offsetMinutesAt(current, timeZone);
    if (currentOffset !== previousOffset) {
      let low = previous;
      let high = current;
      while (high - low > 60_000) {
        const middle = Math.floor((low + high) / 120_000) * 60_000;
        if (offsetMinutesAt(middle, timeZone) === previousOffset) low = middle;
        else high = middle;
      }
      transitions.push(high);
    }
    previous = current;
    previousOffset = currentOffset;
  }
  return transitions;
}

test("exact JSON preserves explicit fields and rejects lossy values", () => {
  assert.equal(
    serializeExactJson({ name: "synthetic", count: 2 }),
    '{"name":"synthetic","count":2}',
  );
  assert.throws(() => serializeExactJson({ name: undefined }), TypeError);
  assert.throws(() => serializeExactJson({ amount: Number.NaN }), TypeError);
  const cyclic = {};
  cyclic.self = cyclic;
  assert.throws(() => serializeExactJson(cyclic), TypeError);
});

test("API errors expose safe codes, not server messages or credentials", () => {
  const privateMessage = "private-details-not-for-ui";
  const payload = {
    error: {
      code: "invalid_credentials",
      message: privateMessage,
      request_id: "req.phase3.synthetic",
      details: [],
    },
  };
  const response = new Response(JSON.stringify(payload), {
    status: 401,
    headers: { "content-type": "application/json" },
  });
  const error = normalizeApiError(response, payload);

  assert.equal(error.code, "invalid_credentials");
  assert.equal(error.requestId, "req.phase3.synthetic");
  assert.equal(
    authErrorMessage(error),
    "El correo o la contraseña no son válidos.",
  );
  assert.equal(error.message.includes(privateMessage), false);
  assert.equal(authErrorMessage(error).includes(privateMessage), false);

  const unauthenticatedPayload = {
    error: { ...payload.error, code: "unauthenticated" },
  };
  const unauthenticated = normalizeApiError(
    new Response(JSON.stringify(unauthenticatedPayload), {
      status: 401,
      headers: { "content-type": "application/json" },
    }),
    unauthenticatedPayload,
  );
  assert.equal(unauthenticated.code, "unauthenticated");
  assert.equal(
    authErrorMessage(unauthenticated),
    "El correo o la contraseña no son válidos.",
  );
});

test("a mutation retry retains one idempotency key and immutable exact payload", () => {
  const intent = createMutationIntentCore(
    { name: "synthetic", options: { enabled: true } },
    serializeExactJson,
  );
  const firstAttempt = {
    key: intent.idempotencyKey,
    body: intent.serializedBody,
  };
  const retry = {
    key: intent.idempotencyKey,
    body: intent.serializedBody,
  };

  assert.match(
    intent.idempotencyKey,
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
  );
  assert.deepEqual(retry, firstAttempt);
  assert.equal(
    intent.serializedBody,
    '{"name":"synthetic","options":{"enabled":true}}',
  );
  assert.ok(Object.isFrozen(intent.body));
  assert.ok(Object.isFrozen(intent.body.options));
  assert.throws(() => {
    intent.body.options.enabled = false;
  }, TypeError);
});

test("CLP values remain exact beyond JavaScript's safe integer range", () => {
  const amount = clpAmount("9007199254740993");
  assert.equal(formatClp(amount), "$9.007.199.254.740.993");
  assert.equal(addClp(amount, clpAmount("7")), "9007199254741000");
  assert.throws(() => clpAmount(9_007_199_254_740_993), RangeError);
});

test("opaque cursors are retained only for canonically identical filters", () => {
  const pager = new OpaqueCursorPager();
  pager.accept({ category: "studio", sort: "price" }, "opaque-cursor-a");
  assert.equal(
    pager.current({ sort: "price", category: "studio" }),
    "opaque-cursor-a",
  );
  assert.equal(pager.current({ sort: "price", category: "office" }), null);
  pager.accept({ category: "office", sort: "price" }, "opaque-cursor-b");
  assert.equal(
    pager.current({ category: "office", sort: "price" }),
    "opaque-cursor-b",
  );
});

test("RFC 3339 timestamps format in America/Santiago across its 2026 DST transitions", () => {
  const timeZone = assertIanaTimeZone("America/Santiago");
  assert.equal(
    parseRfc3339("2026-04-05T02:30:00Z").toISOString(),
    "2026-04-05T02:30:00.000Z",
  );
  const transitions = transitionsInYear(2026, timeZone);
  assert.ok(transitions.length >= 1, "Chile transition data must be available");
  for (const transition of transitions) {
    const before = new Date(transition - 60_000).toISOString();
    const after = new Date(transition).toISOString();
    assert.notEqual(
      formatInTimeZone(before, timeZone),
      formatInTimeZone(after, timeZone),
    );
  }
  assert.throws(() => assertIanaTimeZone("Mars/Olympus"), RangeError);
});
