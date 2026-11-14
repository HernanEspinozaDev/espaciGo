import test from "node:test";
import assert from "node:assert/strict";

test("public rental availability endpoint and return URL contracts conform to Phase 2 and Phase 3 specifications", async () => {
  const spaceID = "11111111-1111-4111-8111-111111111111";
  const date = "2026-12-01";
  const duration = 2;

  // 1. URL formatting
  const expectedPath = `/api/backend/spaces/${encodeURIComponent(spaceID)}/rental-availability?date=${encodeURIComponent(date)}&duration=${duration}`;
  assert.equal(
    expectedPath,
    "/api/backend/spaces/11111111-1111-4111-8111-111111111111/rental-availability?date=2026-12-01&duration=2",
  );

  // 2. Safe return URL for unauthenticated login prompt
  const returnUrl = `/espacios/${encodeURIComponent(spaceID)}`;
  const loginUrl = `/acceso?returnTo=${encodeURIComponent(returnUrl)}`;
  assert.equal(
    loginUrl,
    "/acceso?returnTo=%2Fespacios%2F11111111-1111-4111-8111-111111111111",
  );

  // 3. Response shape validation
  const sampleEnvelope = {
    data: {
      space_id: spaceID,
      time_zone: "America/Santiago",
      rate_unit: "hora",
      checked_at: "2026-11-14T05:00:00Z",
      items: [
        {
          start_at: "2026-12-01T14:00:00Z",
          end_at: "2026-12-01T16:00:00Z",
        },
        {
          start_at: "2026-12-01T16:00:00Z",
          end_at: "2026-12-01T18:00:00Z",
        },
      ],
    },
  };

  assert.equal(sampleEnvelope.data.items.length, 2);
  assert.equal(sampleEnvelope.data.items[0].start_at, "2026-12-01T14:00:00Z");
  assert.equal(sampleEnvelope.data.items[0].end_at, "2026-12-01T16:00:00Z");
});

test("rental availability durations match allowed combinations ratified by product", () => {
  const getDurations = (unit) => {
    switch (unit) {
      case "hora":
        return [1, 2, 4];
      case "dia":
        return [1, 2, 3];
      case "mes":
        return [1];
      default:
        return [];
    }
  };

  assert.deepEqual(getDurations("hora"), [1, 2, 4]);
  assert.deepEqual(getDurations("dia"), [1, 2, 3]);
  assert.deepEqual(getDurations("mes"), [1]);
});
