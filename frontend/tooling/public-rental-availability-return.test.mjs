import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { readFile } from "node:fs/promises";
import test from "node:test";

async function loadModule(path, aliases = {}) {
  const sourceUrl = new URL(path, import.meta.url);
  const source = await readFile(sourceUrl, "utf8");
  const ts = await import("typescript");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const mod = { exports: {} };
  const require = createRequire(sourceUrl);
  const localRequire = (specifier) => {
    if (Object.hasOwn(aliases, specifier)) return aliases[specifier];
    return specifier === "@/features/public-catalog/domain/query-state"
      ? {
          parseCatalogQuery: () => {
            throw new Error(
              "explore query parser should not run for a space return",
            );
          },
          serializeCatalogQuery: () => {
            throw new Error(
              "explore query serializer should not run for a space return",
            );
          },
        }
      : require(specifier);
  };
  new Function("require", "module", "exports", compiled)(
    localRequire,
    mod,
    mod.exports,
  );
  return mod.exports;
}

test("public rental login link uses the access screen's safe return_to contract", async () => {
  const { publicSpaceLoginHref } = await loadModule(
    "../src/features/public-catalog/domain/availability-return.ts",
  );
  const availabilityReturn = await loadModule(
    "../src/features/public-catalog/domain/availability-return.ts",
  );
  const { resolveReturnParam } = await loadModule(
    "../src/features/auth/domain/return-target.ts",
    {
      "@/features/public-catalog/domain/availability-return":
        availabilityReturn,
    },
  );
  const spaceId = "11111111-1111-4111-8111-111111111111";
  const href = publicSpaceLoginHref(spaceId);
  const url = new URL(href, "https://example.test");

  assert.equal(url.pathname, "/acceso");
  assert.deepEqual([...url.searchParams.keys()], ["return_to"]);
  assert.equal(
    resolveReturnParam(url.searchParams.getAll("return_to")),
    `/espacios/${spaceId}`,
  );

  const selection = {
    date: "2026-11-16",
    duration: 1,
    startAt: "2026-11-16T12:00:00Z",
    endAt: "2026-11-16T13:00:00Z",
  };
  const selectedHref = publicSpaceLoginHref(spaceId, selection);
  const selectedUrl = new URL(selectedHref, "https://example.test");
  const selectedReturn = resolveReturnParam(
    selectedUrl.searchParams.getAll("return_to"),
  );
  const selectedTarget = new URL(selectedReturn, "https://example.test");
  assert.equal(selectedTarget.pathname, `/espacios/${spaceId}`);
  assert.deepEqual(
    [...selectedTarget.searchParams.entries()],
    [
      ["rental_date", selection.date],
      ["rental_duration", String(selection.duration)],
      ["rental_start_at", selection.startAt],
      ["rental_end_at", selection.endAt],
    ],
  );

  const invalidHref = publicSpaceLoginHref(spaceId, {
    ...selection,
    endAt: selection.startAt,
  });
  const invalidUrl = new URL(invalidHref, "https://example.test");
  const invalidReturn = resolveReturnParam(
    invalidUrl.searchParams.getAll("return_to"),
  );
  assert.equal(invalidReturn, `/espacios/${spaceId}`);
});
