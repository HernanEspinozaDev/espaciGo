import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import { createRequire } from "node:module";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { createElement } from "react";
import ts from "typescript";

const require = createRequire(import.meta.url);
const root = new URL("../", import.meta.url);
const temporary = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "public-rental-calendar-"),
);
const reactUrl = pathToFileURL(require.resolve("react")).href;
const jsxRuntimeUrl = pathToFileURL(require.resolve("react/jsx-runtime")).href;

async function compile(relative, name, replacements = []) {
  let source = await readFile(new URL(relative, root), "utf8");
  for (const [from, to] of replacements) source = source.replaceAll(from, to);
  const output = ts
    .transpileModule(source, {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ESNext,
        jsx: ts.JsxEmit.ReactJSX,
      },
    })
    .outputText.replaceAll('"react/jsx-runtime"', JSON.stringify(jsxRuntimeUrl))
    .replaceAll('"react"', JSON.stringify(reactUrl));
  const file = path.join(temporary, name);
  await writeFile(file, output);
  return import(pathToFileURL(file));
}

await compile(
  "src/features/rental-demo/domain/calendar.ts",
  "calendar-domain.mjs",
);
await compile(
  "src/features/rental-demo/ui/month-calendar.tsx",
  "month-calendar.mjs",
  [["../domain/calendar", "./calendar-domain.mjs"]],
);
await compile(
  "src/features/public-catalog/domain/local-date.ts",
  "local-date.mjs",
);
await writeFile(
  path.join(temporary, "notice.mjs"),
  'export const publicAvailabilityNotice = "Consulta puntual.";',
);
await writeFile(
  path.join(temporary, "error.mjs"),
  'export const publicAvailabilityExceptionMessage = () => "No disponible.";',
);
await writeFile(
  path.join(temporary, "return.mjs"),
  'export const publicSpaceLoginHref = () => "/acceso";',
);
await writeFile(
  path.join(temporary, "runtime.mjs"),
  "export const publicRentalAvailability = { getAvailability: async () => ({ ok: true, value: { items: [] } }) };",
);
await writeFile(
  path.join(temporary, "quote-runtime.mjs"),
  "export const publicRentalQuote = { createQuote: async () => ({ ok: false, error: { kind: 'other', status: 500 } }) };",
);
await writeFile(
  path.join(temporary, "quote-format.mjs"),
  'export const formatRentalCLP = () => "$12.000 CLP";',
);
await writeFile(
  path.join(temporary, "auth.mjs"),
  'export const useAuth = () => ({ snapshot: { status: "checking", accountId: null, expiresAt: null } });',
);
await writeFile(
  path.join(temporary, "preview.mjs"),
  `import { createElement } from ${JSON.stringify(reactUrl)}; export function PublicRentalContractPreview() { return createElement("div"); }`,
);
await writeFile(
  path.join(temporary, "link.mjs"),
  `import { createElement } from ${JSON.stringify(reactUrl)}; export default function Link({ href, children, ...props }) { return createElement("a", { href, ...props }, children); }`,
);
const { PublicSpaceRentalAvailability } = await compile(
  "src/features/public-catalog/ui/public-space-rental-availability.tsx",
  "public-space-rental-availability.mjs",
  [
    ["@/features/rental-demo/ui/month-calendar", "./month-calendar.mjs"],
    ["../domain/local-date", "./local-date.mjs"],
    ["../domain/availability-notice", "./notice.mjs"],
    ["../domain/availability-error", "./error.mjs"],
    ["../domain/availability-return", "./return.mjs"],
    ["../infrastructure/rental-availability-runtime", "./runtime.mjs"],
    ["../infrastructure/rental-quote-runtime", "./quote-runtime.mjs"],
    ["../domain/contract-preview-format", "./quote-format.mjs"],
    ["@/features/auth/ui/use-auth", "./auth.mjs"],
    ["./public-rental-contract-preview", "./preview.mjs"],
    ["next/link", "./link.mjs"],
  ],
);

test.after(async () => {
  await rm(temporary, { recursive: true, force: true });
});

test("public availability permits choosing an unqueried date within the 90-day horizon", () => {
  const html = renderToStaticMarkup(
    createElement(PublicSpaceRentalAvailability, {
      spaceId: "11111111-1111-4111-8111-111111111111",
      rateUnit: "hora",
      basePriceCLP: 12000,
      today: "2030-01-15",
    }),
  );
  const buttons = [...html.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g)];
  const day = (number) =>
    buttons.find(([, , content]) => content.includes(`<span>${number}</span>`));
  const yesterday = day(14);
  const today = day(15);
  assert.ok(yesterday);
  assert.match(yesterday[1], /disabled=""/);
  assert.ok(today);
  assert.doesNotMatch(today[1], /disabled=""/);
  assert.match(today[2], /Sin información/);
});
