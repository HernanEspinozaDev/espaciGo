import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { parse } from "yaml";

const specPath = new URL("../../planning/openapi.yaml", import.meta.url);
const spec = parse(await readFile(specPath, "utf8"));
const generatedPaths = [
  new URL("../src/lib/api/generated/openapi.d.ts", import.meta.url),
  new URL("../src/lib/api/generated/gateway-operations.ts", import.meta.url),
];

test("external identity-provider routes and schemas are absent from OpenAPI", async () => {
  const providerPaths = Object.keys(spec.paths).filter((path) =>
    /didit/i.test(path),
  );
  const providerOperations = Object.values(spec.paths)
    .flatMap((path) => Object.values(path ?? {}))
    .filter(
      (operation) =>
        operation &&
        typeof operation === "object" &&
        /didit/i.test(operation.operationId ?? ""),
    );
  const providerSchemas = Object.keys(spec.components?.schemas ?? {}).filter(
    (name) => /didit/i.test(name),
  );

  assert.deepEqual(providerPaths, []);
  assert.deepEqual(providerOperations, []);
  assert.deepEqual(providerSchemas, []);

  for (const path of generatedPaths) {
    const generated = await readFile(path, "utf8");
    assert.doesNotMatch(
      generated,
      /didit/i,
      `${path.pathname} must not expose provider contracts`,
    );
  }
});
