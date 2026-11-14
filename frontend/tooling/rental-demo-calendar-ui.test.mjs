import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import { createRequire } from "node:module";
import path from "node:path";
import test from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { createElement } from "react";
import ts from "typescript";

const require = createRequire(import.meta.url);
const root = fileURLToPath(new URL("../", import.meta.url));
const tempDir = await mkdtemp(path.join(process.env.TMPDIR ?? os.tmpdir(), "rental-demo-cal-ui-"));

test.after(async () => {
  await rm(tempDir, { recursive: true, force: true });
});

async function compile(relPath, outputName, replacements = []) {
  let source = await readFile(path.join(root, relPath), "utf8");
  for (const [from, to] of replacements) {
    source = source.replaceAll(from, to);
  }
  const transpiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
      jsx: ts.JsxEmit.ReactJSX,
    },
  }).outputText
    .replaceAll(
      '"react/jsx-runtime"',
      JSON.stringify(pathToFileURL(require.resolve("react/jsx-runtime")).href),
    )
    .replaceAll(
      '"react"',
      JSON.stringify(pathToFileURL(require.resolve("react")).href),
    );
  const outPath = path.join(tempDir, outputName);
  await writeFile(outPath, transpiled);
  return import(pathToFileURL(outPath).href);
}

await compile(
  "src/features/rental-demo/domain/calendar.ts",
  "calendar.mjs",
);

const { MonthCalendar } = await compile(
  "src/features/rental-demo/ui/month-calendar.tsx",
  "month-calendar.mjs",
  [["../domain/calendar", "./calendar.mjs"]],
);

test("MonthCalendar renders accessible grid, header navigation and availability legend", () => {
  const days = [
    { date: "2030-01-14", status: "available", intervals: [{ start_at: "2030-01-14T10:00:00Z", end_at: "2030-01-14T11:00:00Z" }] },
    { date: "2030-01-15", status: "available", intervals: [{ start_at: "2030-01-15T10:00:00Z", end_at: "2030-01-15T11:00:00Z" }] },
    { date: "2030-01-16", status: "partial", intervals: [{ start_at: "2030-01-16T14:00:00Z", end_at: "2030-01-16T15:00:00Z" }] },
    { date: "2030-01-17", status: "unavailable", intervals: [] },
    { date: "2030-01-18", status: "outside_horizon", intervals: [] },
  ];

  const html = renderToStaticMarkup(
    createElement(MonthCalendar, {
      month: "2030-01",
      days,
      selectedDate: "2030-01-15",
      onSelect: () => {},
      onMonthChange: () => {},
      today: "2030-01-15",
      disabled: false,
    }),
  );

  // Check semantic grid roles
  assert.match(html, /role="grid"/);
  assert.match(html, /role="columnheader"/);
  assert.match(html, /role="gridcell"/);

  // Check navigation controls
  assert.match(html, /aria-label="Mes anterior"/);
  assert.match(html, /aria-label="Mes siguiente"/);
  assert.match(html, />Hoy<\/button>/);

  // Check selected button attributes
  assert.match(html, /aria-pressed="true"/);
  assert.match(html, /Seleccionado/);

  // Check outside horizon status is disabled
  assert.match(html, /disabled=""[^>]*><span>18<\/span>/);

  // Check visual classes by status
  assert.match(html, /border-emerald-600 bg-emerald-50/);
  assert.match(html, /border-amber-600 bg-amber-50/);
  assert.match(html, /border-muted bg-muted\/30/);

  // Check padding days outside the month have aria-hidden
  assert.match(html, /<span aria-hidden="true" class="block min-h-11"/);

  // Check legend elements
  assert.match(html, /aria-label="Leyenda de disponibilidad"/);
  assert.match(html, /<li>Disponible<\/li>/);
  assert.match(html, /<li>Parcialmente disponible<\/li>/);
  assert.match(html, /<li>No disponible<\/li>/);
  assert.doesNotMatch(html, /<li>Fuera del horizonte de consulta<\/li>/);
});

test("MonthCalendar disables boundary buttons at extreme years 0001 and 9999", () => {
  const htmlMin = renderToStaticMarkup(
    createElement(MonthCalendar, {
      month: "0001-01",
      days: [],
      selectedDate: null,
      onSelect: () => {},
      onMonthChange: () => {},
      today: "2030-01-15",
      disabled: false,
    }),
  );
  assert.match(htmlMin, /aria-label="Mes anterior" disabled=""/);
  assert.doesNotMatch(htmlMin, /aria-label="Mes siguiente" disabled=""/);

  const htmlMax = renderToStaticMarkup(
    createElement(MonthCalendar, {
      month: "9999-12",
      days: [],
      selectedDate: null,
      onSelect: () => {},
      onMonthChange: () => {},
      today: "2030-01-15",
      disabled: false,
    }),
  );
  assert.doesNotMatch(htmlMax, /aria-label="Mes anterior" disabled=""/);
  assert.match(htmlMax, /aria-label="Mes siguiente" disabled=""/);
});

test("MonthCalendar disables all navigation and day buttons when disabled prop is true", () => {
  const html = renderToStaticMarkup(
    createElement(MonthCalendar, {
      month: "2030-01",
      days: [],
      selectedDate: null,
      onSelect: () => {},
      onMonthChange: () => {},
      today: "2030-01-15",
      disabled: true,
    }),
  );

  // Both nav buttons must be disabled
  assert.match(html, /aria-label="Mes anterior" disabled=""/);
  assert.match(html, /aria-label="Mes siguiente" disabled=""/);
  assert.match(html, /disabled=""[^>]*>Hoy<\/button>/);
});
