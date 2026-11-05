import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";
import { parse as parseYaml } from "yaml";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-schedule-cas-"),
);
let calls = [],
  result;
const source = await readFile(
  path.join(
    root,
    "src/features/host-listings/infrastructure/host-listings-openapi-adapter.ts",
  ),
  "utf8",
);
const transformed = source
  .replace('import "client-only";', "")
  .replace(
    'import { apiClient } from "@/lib/api/client";',
    "const apiClient = globalThis.__scheduleCasApi;",
  )
  .replace(
    'import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";',
    'const ApiProtocolError = class extends Error {}; const normalizeApiError = (response) => Object.assign(new Error("api error"), { status: response.status });',
  )
  .replace(
    'import { serializeExactJson } from "@/lib/api/exact-json";',
    "const serializeExactJson = JSON.stringify;",
  )
  .replace(
    /import type \{[\s\S]*?\} from "..\/application\/host-listings\.port";/,
    "",
  )
  .replace(/import type \{[\s\S]*?\} from "..\/domain\/types";/, "")
  .replace(
    /import \{[\s\S]*?\} from "..\/domain\/validation";/,
    "const convertSlotsToWeeklyDays = () => [];",
  );
await writeFile(
  path.join(scratch, "adapter.mjs"),
  ts.transpileModule(transformed, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText,
);
globalThis.__scheduleCasApi = {
  GET: async (...a) => {
    calls.push(a);
    return result;
  },
  PUT: async (...a) => {
    calls.push(a);
    return result;
  },
  POST: async (...a) => {
    calls.push(a);
    return result;
  },
};
const { OpenApiHostListingsAdapter } = await import(
  pathToFileURL(path.join(scratch, "adapter.mjs")).href
);
const adapter = new OpenApiHostListingsAdapter();
function response(status = 200, etag = '"rev-2"', weekly = false) {
  return {
    response: {
      ok: status >= 200 && status < 300,
      status,
      headers: { get: (n) => (n === "ETag" ? etag : null) },
    },
    data: weekly
      ? {
          data: {
            enabled: true,
            days: [
              { weekday: 1, periods: [{ open: "09:00", close: "17:00" }] },
            ],
          },
        }
      : { space_id: "space-1", time_zone: "America/Santiago" },
  };
}
function reset(next = response()) {
  calls = [];
  result = next;
}
test.after(async () => rm(scratch, { recursive: true, force: true }));
test("createDraft returns the strong ETag needed by the first CAS mutation", async () => {
  calls = [];
  result = {
    response: {
      ok: true,
      status: 201,
      headers: { get: (name) => (name === "ETag" ? '"rev-1"' : null) },
    },
    data: {
      id: "space-1",
      category_code: "oficina",
      category_name: "Oficinas",
      title: "Oficina",
      description: "",
      area_m2: 10,
      capacity: 2,
      usage_rules: "",
      rate_unit: "hora",
      base_price_clp: 6000,
      address: "Dirección",
      state: "borrador",
      time_zone: null,
      region_code: "13",
      comuna_code: "Las Condes",
      attribute_schema_version: 1,
      attributes: {},
    },
  };
  const created = await adapter.createDraft(
    {
      category_code: "oficina",
      title: "Oficina",
      description: "",
      area_m2: 10,
      capacity: 2,
      usage_rules: "",
      rate_unit: "hora",
      base_price_clp: 6000,
      address: "Dirección",
    },
    "create-idempotency-key",
  );
  assert.equal(created.etag, '"rev-1"');
  assert.equal(calls.length, 1);
});
test("timezone and weekly mutations send If-Match and map renewed strong ETags", async () => {
  reset();
  const z = await adapter.configureTimeZone(
    "space-1",
    "America/Santiago",
    '"rev-1"',
  );
  assert.equal(calls[0][1].params.header["If-Match"], '"rev-1"');
  assert.equal(z.etag, '"rev-2"');
  reset(response(200, '"rev-3"', true));
  const s = await adapter.updateWeeklySchedule(
    "space-1",
    { enabled: true, weekly_slots: [] },
    '"rev-2"',
  );
  assert.equal(calls[0][0], "/spaces/{spaceId}/weekly-hours");
  assert.equal(calls[0][1].params.header["If-Match"], '"rev-2"');
  assert.equal(s.etag, '"rev-3"');
});
test("schedule mutations reject missing, weak, or malformed revisions before request", async () => {
  for (const revision of [undefined, "", 'W/"weak"', "rev-1", '"a", "b"'])
    for (const method of ["configureTimeZone", "updateWeeklySchedule"]) {
      reset();
      await assert.rejects(() =>
        method === "configureTimeZone"
          ? adapter[method]("space-1", "America/Santiago", revision)
          : adapter[method](
              "space-1",
              { enabled: true, weekly_slots: [] },
              revision,
            ),
      );
      assert.equal(calls.length, 0);
    }
});
test("schedule mutations fail closed without response ETag and propagate 412 without retry", async () => {
  for (const method of ["configureTimeZone", "updateWeeklySchedule"]) {
    const invoke = () =>
      method === "configureTimeZone"
        ? adapter[method]("space-1", "America/Santiago", '"rev-1"')
        : adapter[method](
            "space-1",
            { enabled: true, weekly_slots: [] },
            '"rev-1"',
          );
    reset(response(200, null, method === "updateWeeklySchedule"));
    await assert.rejects(invoke);
    assert.equal(calls.length, 1);
    reset(response(412, null, method === "updateWeeklySchedule"));
    await assert.rejects(invoke, (e) => e.status === 412);
    assert.equal(calls.length, 1);
  }
});
test("weekly schedule GET maps its response ETag", async () => {
  reset(response(200, '"rev-7"', true));
  const s = await adapter.getWeeklySchedule("space-1");
  assert.equal(calls[0][0], "/spaces/{spaceId}/weekly-hours");
  assert.equal(s.etag, '"rev-7"');
});

test("OpenAPI requires ETag response headers and CAS errors for weekly-hours", async () => {
  const spec = parseYaml(
    await readFile(path.join(root, "..", "planning", "openapi.yaml"), "utf8"),
  );
  const weekly = spec.paths["/spaces/{spaceId}/weekly-hours"];
  assert.ok(
    weekly,
    "owner schedule endpoint must exist outside local booking-trial",
  );
  assert.equal(weekly.get.operationId, "getOwnSpaceWeeklyHours");
  assert.equal(weekly.put.operationId, "replaceOwnSpaceWeeklyHours");
  for (const operation of [weekly.get, weekly.put]) {
    const etag = operation.responses["200"].headers?.ETag;
    assert.ok(etag, "weekly-hours 200 response must declare ETag");
    assert.equal(etag.required, true);
    assert.equal(etag.schema.type, "string");
  }
  assert.equal(
    weekly.put.parameters.find((parameter) => parameter.name === "If-Match")
      ?.required,
    true,
  );
  for (const status of ["400", "412", "428"]) {
    assert.ok(weekly.put.responses[status], `missing PUT response ${status}`);
  }
});
