import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "weekly-slots-"),
);
let validationPath;
let helpersPath;
async function compile(sourcePath, target, replacements = []) {
  let source = await readFile(path.join(root, sourcePath), "utf8");
  for (const [from, to] of replacements) source = source.replaceAll(from, to);
  const output = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText;
  const dest = path.join(scratch, target);
  await writeFile(dest, output);
  return dest;
}
const typesPath = await compile(
  "src/features/host-listings/domain/types.ts",
  "types.mjs",
);
validationPath = await compile(
  "src/features/host-listings/domain/validation.ts",
  "validation.mjs",
  [["./types", pathToFileURL(typesPath).href]],
);
helpersPath = await compile(
  "src/features/host-listings/domain/weekly-schedule-slots.ts",
  "helpers.mjs",
  [
    ["./types", pathToFileURL(typesPath).href],
    ["./validation", pathToFileURL(validationPath).href],
  ],
);
test.after(async () => rm(scratch, { recursive: true, force: true }));

test("propagates optional external schedule errors to both time controls", async () => {
  const source = await readFile(
    path.join(root, "src/features/host-listings/ui/weekly-schedule-slots.tsx"),
    "utf8",
  );
  assert.match(source, /invalid\?: boolean;/);
  assert.match(source, /errorId\?: string;/);
  assert.equal(
    (source.match(/aria-invalid=\{invalid \|\| undefined\}/g) ?? []).length,
    2,
  );
  assert.equal(
    (source.match(/aria-describedby=\{invalid \? errorId : undefined\}/g) ?? [])
      .length,
    2,
  );
});

test("hour toggles merge adjacent cells and preserve fractional intervals", async () => {
  const { toggleWeeklyHour } = await import(pathToFileURL(helpersPath).href);
  const result = toggleWeeklyHour(
    [{ day_of_week: 1, start_time: "09:30", end_time: "10:30" }],
    1,
    10,
    true,
  );
  assert.deepEqual(result, {
    slots: [{ day_of_week: 1, start_time: "09:30", end_time: "11:00" }],
    error: null,
  });
});
test("hour coverage distinguishes exact partial unions without rounding or mutation", async () => {
  const { weeklyHourCoverage } = await import(pathToFileURL(helpersPath).href);
  const slots = [
    { day_of_week: 1, start_time: "09:15", end_time: "09:30" },
    { day_of_week: 1, start_time: "09:40", end_time: "10:00" },
    { day_of_week: 1, start_time: "17:00", end_time: "17:30" },
    { day_of_week: 2, start_time: "17:00", end_time: "18:00" },
  ];
  const snapshot = structuredClone(slots);
  assert.equal(weeklyHourCoverage(slots, 1, 9), "partial");
  assert.equal(
    weeklyHourCoverage(
      [{ day_of_week: 1, start_time: "09:15", end_time: "10:00" }],
      1,
      9,
    ),
    "partial",
  );
  assert.equal(
    weeklyHourCoverage(
      [{ day_of_week: 1, start_time: "23:30", end_time: "24:00" }],
      1,
      23,
    ),
    "partial",
  );
  assert.equal(weeklyHourCoverage(slots, 1, 17), "partial");
  assert.equal(
    weeklyHourCoverage(
      [
        { day_of_week: 1, start_time: "17:00", end_time: "17:20" },
        { day_of_week: 1, start_time: "17:20", end_time: "18:00" },
      ],
      1,
      17,
    ),
    "full",
  );
  assert.equal(weeklyHourCoverage([], 1, 17), "none");
  assert.equal(weeklyHourCoverage(slots, 1, 16), "none");
  assert.deepEqual(slots, snapshot);
});

test("hour coverage handles disjoint pieces and the 24:00 boundary", async () => {
  const coveragePath = await compile(
    "src/features/host-listings/domain/weekly-schedule-slots.ts",
    "coverage-boundary.mjs",
    [
      ["./types", pathToFileURL(typesPath).href],
      ["./validation", pathToFileURL(validationPath).href],
    ],
  );
  const { weeklyHourCoverage } = await import(pathToFileURL(coveragePath).href);
  assert.equal(
    weeklyHourCoverage(
      [
        { day_of_week: 1, start_time: "23:00", end_time: "23:30" },
        { day_of_week: 1, start_time: "23:30", end_time: "24:00" },
      ],
      1,
      23,
    ),
    "full",
  );
  assert.equal(
    weeklyHourCoverage(
      [
        { day_of_week: 1, start_time: "09:00", end_time: "09:15" },
        { day_of_week: 1, start_time: "09:45", end_time: "10:00" },
      ],
      1,
      9,
    ),
    "partial",
  );
});

test("rejects a third disjoint daily interval without losing prior slots", async () => {
  const { toggleWeeklyHour } = await import(pathToFileURL(helpersPath).href);
  const slots = [
    { day_of_week: 1, start_time: "08:00", end_time: "09:00" },
    { day_of_week: 1, start_time: "11:00", end_time: "12:00" },
  ];
  const result = toggleWeeklyHour(slots, 1, 15, true);
  assert.equal(
    result.error,
    "Cada día admite como máximo dos tramos horarios.",
  );
  assert.deepEqual(result.slots, slots);
});
test("hour removal splits an interval without rounding its existing fractional edge", async () => {
  const { toggleWeeklyHour } = await import(pathToFileURL(helpersPath).href);
  const result = toggleWeeklyHour(
    [{ day_of_week: 2, start_time: "09:30", end_time: "12:30" }],
    2,
    10,
    false,
  );
  assert.deepEqual(result.slots, [
    { day_of_week: 2, start_time: "09:30", end_time: "10:00" },
    { day_of_week: 2, start_time: "11:00", end_time: "12:30" },
  ]);
});
test("preset replaces only the selected day and supports end of day", async () => {
  const { applyWeeklyPreset } = await import(pathToFileURL(helpersPath).href);
  const before = [{ day_of_week: 2, start_time: "09:30", end_time: "10:30" }];
  assert.deepEqual(applyWeeklyPreset(before, 1, "24h"), {
    slots: [
      { ...before[0] },
      { day_of_week: 1, start_time: "00:00", end_time: "24:00" },
    ],
    error: null,
  });
});

test("preserves other days and rejects invalid day without throwing", async () => {
  const { toggleWeeklyHour } = await import(pathToFileURL(helpersPath).href);
  const other = {
    day_of_week: 0,
    start_time: "09:30",
    end_time: "12:30",
    extra: "keep",
  };
  const slots = [
    other,
    { day_of_week: 1, start_time: "08:00", end_time: "09:00" },
  ];
  const invalid = toggleWeeklyHour(slots, 8, 10, true);
  assert.ok(invalid.error);
  assert.deepEqual(invalid.slots, slots);
  const updated = toggleWeeklyHour(slots, 1, 9, true);
  assert.equal(updated.error, null);
  assert.deepEqual(updated.slots[0], other);
});

test("rejects invalid 24-hour edits without changing source slots", async () => {
  const { toggleWeeklyHour } = await import(pathToFileURL(helpersPath).href);
  const slots = [
    { day_of_week: 1, start_time: "08:00", end_time: "09:00" },
    { day_of_week: 1, start_time: "11:00", end_time: "12:00" },
  ];
  const result = toggleWeeklyHour(slots, 1, 15, true);
  assert.ok(result.error);
  assert.deepEqual(result.slots, slots);
});

test("morning is a preset, not a tomorrow-day preset", async () => {
  const { applyWeeklyPreset } = await import(pathToFileURL(helpersPath).href);
  assert.equal(
    applyWeeklyPreset([], 1, "morning").slots[0].start_time,
    "09:00",
  );
  assert.throws(() => applyWeeklyPreset([], 1, "tomorrow"));
});
