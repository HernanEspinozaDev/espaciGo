import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL, fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "public-weekly-"),
);
const source = await readFile(
  path.join(root, "src/features/public-catalog/domain/weekly-schedule.ts"),
  "utf8",
);
const output = ts.transpileModule(source, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;
const compiled = path.join(scratch, "weekly.mjs");
await writeFile(compiled, output);
test.after(async () => rm(scratch, { recursive: true, force: true }));

test("maps generated public weekly DTO with ISO weekdays and exact period times", async () => {
  const { publicWeeklyScheduleSlots } = await import(
    pathToFileURL(compiled).href
  );
  assert.deepEqual(
    publicWeeklyScheduleSlots({
      enabled: true,
      time_zone: "America/Santiago",
      days: [
        {
          weekday: 2,
          periods: [
            { open: "09:30", close: "12:45" },
            { open: "14:15", close: "16:45" },
          ],
        },
        { weekday: 7, periods: [{ open: "00:00", close: "24:00" }] },
      ],
    }),
    [
      { day_of_week: 2, start_time: "09:30", end_time: "12:45" },
      { day_of_week: 2, start_time: "14:15", end_time: "16:45" },
      { day_of_week: 7, start_time: "00:00", end_time: "24:00" },
    ],
  );
});
