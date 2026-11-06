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
const temporary = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "public-verification-"),
);
async function compile(relative, name, replacements = []) {
  let source = await readFile(
    new URL(`../${relative}`, import.meta.url),
    "utf8",
  );
  for (const [from, to] of replacements) source = source.replaceAll(from, to);
  const result = ts
    .transpileModule(source, {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ESNext,
        jsx: ts.JsxEmit.ReactJSX,
      },
    })
    .outputText.replaceAll(
      '"react/jsx-runtime"',
      JSON.stringify(pathToFileURL(require.resolve("react/jsx-runtime")).href),
    );
  const file = path.join(temporary, name);
  await writeFile(file, result);
  return import(pathToFileURL(file));
}
await writeFile(
  path.join(temporary, "server.mjs"),
  'export const getApiBackendBaseUrl = () => new URL("http://127.0.0.1:18082/api/v1");',
);
await compile("src/lib/api/clp.ts", "clp.mjs");
await compile("src/lib/api/time.ts", "time.mjs");
const queryPath = await compile(
  "src/features/public-catalog/domain/query-state.ts",
  "query-state.mjs",
  [
    ["@/lib/api/clp", "./clp.mjs"],
    ["@/lib/api/time", "./time.mjs"],
  ],
);
await writeFile(
  path.join(temporary, "query.mjs"),
  'export { serializeCatalogQuery } from "./query-state.mjs";',
);
await compile(
  "src/features/public-catalog/domain/verification.ts",
  "verification.mjs",
);
await writeFile(
  path.join(temporary, "cloud-run-identity.mjs"),
  "export const getCloudRunIdentityHeadersForBackend = async () => ({});",
);
const { goPublicCatalogAdapter: adapter } = await compile(
  "src/features/public-catalog/infrastructure/go-public-catalog-adapter.ts",
  "adapter.mjs",
  [
    ['import "server-only";', ""],
    ["@/lib/api/server-config", "./server.mjs"],
    ["@/lib/api/cloud-run-identity", "./cloud-run-identity.mjs"],
    ["../domain/query-state", "./query.mjs"],
    ["../domain/verification", "./verification.mjs"],
  ],
);
const { PublicVerificationBadges: Badges } = await compile(
  "src/features/public-catalog/ui/public-verification-badges.tsx",
  "badges.mjs",
);
const fixture = {
  space_id: "11111111-1111-4111-8111-111111111111",
  title: "Ensayo sintético",
  category_code: "oficinas",
  category_name: "Oficinas",
  area_m2: 20,
  description: "Control documental",
  currency: "CLP",
  rate_unit: "dia",
  base_price_clp: 1000,
  estimated_total_clp: null,
  usage_rules: "Ensayo",
  features: [],
  images: [],
};
const render = (verification) =>
  renderToStaticMarkup(createElement(Badges, { verification }));

test("catalog and detail refresh remove old verification flags without retaining a badge", async () => {
  const original = globalThis.fetch;
  let flags = { identity_verified: true, space_verified: true };
  globalThis.fetch = async (url, options) => {
    assert.equal(options.cache, "no-store");
    const space = {
      ...fixture,
      ...(flags === undefined ? {} : { verification: flags }),
    };
    return Response.json(
      new URL(url).pathname.endsWith("/public/spaces")
        ? { items: [space] }
        : { data: space },
    );
  };
  try {
    for (const load of [
      async () => {
        const r = await adapter.listSpaces({ attribute: [] });
        assert.equal(r.ok, true);
        return r.value.items[0];
      },
      async () => {
        const r = await adapter.getSpace(fixture.space_id);
        assert.equal(r.ok, true);
        return r.value;
      },
    ]) {
      flags = { identity_verified: true, space_verified: true };
      const initial = render((await load()).verification);
      assert.match(initial, /Identidad verificada/);
      assert.match(initial, /Antecedentes del espacio revisados/);
      flags = { identity_verified: true, space_verified: false };
      const independent = render((await load()).verification);
      assert.match(independent, /Identidad verificada/);
      assert.doesNotMatch(independent, /Antecedentes del espacio revisados/);
      flags = undefined;
      assert.equal(render((await load()).verification), "");
      flags = { identity_verified: false, space_verified: true };
      assert.equal(render((await load()).verification), "");
      flags = { identity_verified: "true", space_verified: true };
      assert.equal(render((await load()).verification), "");
    }
  } finally {
    globalThis.fetch = original;
  }
});

test("verification disclaimers are readable without relying on hover titles", () => {
  const markup = render({ identityVerified: true, spaceVerified: true });
  assert.match(markup, /class="sr-only"[^>]*>[^<]*no acredita/);
});

test.after(async () => rm(temporary, { recursive: true, force: true }));
