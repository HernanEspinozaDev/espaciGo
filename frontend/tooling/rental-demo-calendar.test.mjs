import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(path.join(process.env.TMPDIR ?? os.tmpdir(), "rental-demo-calendar-"));
const source = await readFile(path.join(root, "src/features/rental-demo/domain/calendar.ts"), "utf8");
const output = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText;
const modulePath = path.join(scratch, "calendar.mjs");
await writeFile(modulePath, output);
const calendar = await import(pathToFileURL(modulePath).href);
test.after(async () => rm(scratch, { recursive: true, force: true }));

test("monthDays returns Monday-first padding and civil dates", () => {
  const days = calendar.monthDays("2026-02");
  assert.equal(days.length, 35);
  assert.equal(days[0].date, "2026-01-26");
  assert.equal(days[0].weekday, 0);
  assert.equal(days[6].weekday, 6);
  assert.equal(days[6].date, "2026-02-01");
  assert.equal(days[34].date, "2026-03-01");
});

test("monthDays pads month grid to complete weeks and handles leap years", () => {
  const days = calendar.monthDays("2024-02");
  assert.equal(days.length, 35);
  assert.equal(days.filter((d) => d.inMonth).length, 29);
  assert.equal(days.find((d) => d.date === "2024-02-29").weekday, 3);
});

test("shiftMonth handles year boundaries and rejects out-of-range movement", () => {
  assert.equal(calendar.shiftMonth("2026-12", 1), "2027-01");
  assert.equal(calendar.shiftMonth("2026-01", -1), "2025-12");
  assert.equal(calendar.shiftMonth("0001-01", -1), null);
  assert.equal(calendar.shiftMonth("9999-12", 1), null);
});

test("localDateLabel formats civil date in Spanish without host timezone drift", () => {
  assert.equal(calendar.localDateLabel("2026-03-01"), "domingo, 1 de marzo de 2026");
  assert.equal(calendar.localDateLabel("2024-02-29"), "jueves, 29 de febrero de 2024");
  assert.equal(calendar.localDateLabel("2026-12-31"), "jueves, 31 de diciembre de 2026");
  assert.equal(calendar.localDateLabel("2026-01-01"), "jueves, 1 de enero de 2026");
  assert.equal(calendar.shiftMonth("2026-06", 0), "2026-06");
  assert.equal(calendar.shiftMonth("2026-06", 12), "2027-06");
});

test("calendar validation rejects invalid month and date shapes", () => {
  assert.throws(() => calendar.monthDays("invalid-month"), RangeError);
  assert.throws(() => calendar.monthDays("2026-13"), RangeError);
  assert.throws(() => calendar.monthDays("2026-00"), RangeError);
  assert.throws(() => calendar.monthDays("0000-01"), RangeError);
  assert.throws(() => calendar.localDateLabel("2026-02-29"), RangeError); // 2026 not leap
  assert.throws(() => calendar.localDateLabel("2026-02-30"), RangeError);
  assert.throws(() => calendar.localDateLabel("2026-00-15"), RangeError);
  assert.throws(() => calendar.localDateLabel("2026-13-10"), RangeError);
  assert.throws(() => calendar.localDateLabel("2026-05-00"), RangeError);
  assert.throws(() => calendar.localDateLabel("2026-05-32"), RangeError);
  assert.throws(() => calendar.localDateLabel("2026-04-31"), RangeError); // April has 30 days
  assert.throws(() => calendar.localDateLabel("2026-06-31"), RangeError); // June has 30 days
  assert.throws(() => calendar.localDateLabel("2026-09-31"), RangeError); // Sept has 30 days
  assert.throws(() => calendar.localDateLabel("2026-11-31"), RangeError); // Nov has 30 days
  assert.throws(() => calendar.localDateLabel("not-a-date"), RangeError);
  assert.throws(() => calendar.shiftMonth("2026-01", 1.5), RangeError);
  assert.throws(() => calendar.shiftMonth("2026-01", NaN), RangeError);
  assert.throws(() => calendar.shiftMonth("2026-01", Infinity), RangeError);
});

test("grid always aligns perfectly to full weeks across 28, 29, 30 and 31 day months", () => {
  for (const month of ["2025-02", "2024-02", "2026-04", "2026-05"]) {
    const days = calendar.monthDays(month);
    assert.equal(days.length % 7, 0, `Month ${month} grid must be multiple of 7`);
    assert.equal(days[0].weekday, 0, 'Grid must start on Monday');
    assert.equal(days[days.length - 1].weekday, 6, 'Grid must end on Sunday');
  }
});

test("shiftMonth clamps at extreme year boundaries (0001 to 9999)", () => {
  assert.equal(calendar.shiftMonth("0001-01", -1), null);
  assert.equal(calendar.shiftMonth("9999-12", 1), null);
  assert.equal(calendar.shiftMonth("2026-12", 1), "2027-01");
  assert.equal(calendar.shiftMonth("2026-01", -1), "2025-12");
});

test("monthDays correctly handles Gregorian century leap rules (2000 vs 1900)", () => {
  const feb2000 = calendar.monthDays("2000-02");
  const daysInMonth2000 = feb2000.filter((d) => d.inMonth);
  assert.equal(daysInMonth2000.length, 29, 'Year 2000 is a century leap year (400-year rule)');
  assert.equal(daysInMonth2000[28].date, "2000-02-29");

  const feb1900 = calendar.monthDays("1900-02");
  const daysInMonth1900 = feb1900.filter((d) => d.inMonth);
  assert.equal(daysInMonth1900.length, 28, 'Year 1900 is not a leap year (100-year rule)');
  assert.equal(daysInMonth1900[27].date, "1900-02-28");
});

test("monthDays padding days are strictly contiguous with month days", () => {
  for (const month of ["2026-05", "2026-06", "2026-12"]) {
    const days = calendar.monthDays(month);
    for (let i = 0; i < days.length - 1; i++) {
      const current = new Date(days[i].date + "T00:00:00Z");
      const next = new Date(days[i + 1].date + "T00:00:00Z");
      const diffDays = Math.round((next - current) / (1000 * 60 * 60 * 24));
      assert.equal(diffDays, 1, `Days must be exactly 1 civil day apart: ${days[i].date} -> ${days[i + 1].date}`);
      assert.equal(days[i].weekday, i % 7);
    }
  }
});
