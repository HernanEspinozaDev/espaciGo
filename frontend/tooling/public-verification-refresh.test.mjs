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
import { parse } from "yaml";

const require = createRequire(import.meta.url);
const openApi = parse(
  await readFile(
    new URL("../../planning/openapi.yaml", import.meta.url),
    "utf8",
  ),
);
const schemaPropertyNames = (name) =>
  Object.keys(openApi.components.schemas[name].properties).sort();
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

test("public space adapters keep allowlisted fields and strip unknown private data", async () => {
  const original = globalThis.fetch;
  const hiddenMarkers = [
    "PRIVATE_OWNER_ID_MARKER",
    "PRIVATE_ADDRESS_MARKER",
    "PRIVATE_CONTACT_MARKER",
    "PRIVATE_FEATURE_MARKER",
    "PRIVATE_IMAGE_KEY_MARKER",
    "PRIVATE_REVIEWER_MARKER",
    "PRIVATE_SCHEDULE_MARKER",
  ];
  const wire = {
    ...fixture,
    region_code: "13",
    region_name: "Región Metropolitana de Santiago",
    comuna_code: "13123",
    comuna_name: "Providencia",
    cover_image_id: "22222222-2222-4222-8222-222222222222",
    owner_id: hiddenMarkers[0],
    exact_address: hiddenMarkers[1],
    owner_contact: hiddenMarkers[2],
    features: [
      {
        code: "wifi",
        label: "Wi-Fi",
        value: true,
        private_metadata: hiddenMarkers[3],
      },
    ],
    images: [
      {
        image_id: "22222222-2222-4222-8222-222222222222",
        is_cover: true,
        source: "media",
        object_key: hiddenMarkers[4],
      },
    ],
    verification: {
      identity_verified: true,
      space_verified: true,
      reviewer_id: hiddenMarkers[5],
    },
    weekly_schedule: {
      enabled: true,
      time_zone: "America/Santiago",
      private_metadata: hiddenMarkers[6],
      days: Array.from({ length: 7 }, (_, index) => ({
        weekday: index + 1,
        private_metadata: hiddenMarkers[6],
        periods:
          index === 0
            ? [
                {
                  open: "09:00",
                  close: "17:00",
                  private_metadata: hiddenMarkers[6],
                },
              ]
            : [],
      })),
    },
  };
  globalThis.fetch = async (url) =>
    Response.json(
      new URL(url).pathname.endsWith("/public/spaces")
        ? { items: [wire] }
        : { data: wire },
    );
  try {
    const listed = await adapter.listSpaces({ attribute: [] });
    const detailed = await adapter.getSpace(fixture.space_id);
    assert.equal(listed.ok, true);
    assert.equal(detailed.ok, true);
    const listSpace = listed.value.items[0];
    const detailSpace = detailed.value;
    const sortedKeys = (value) => Object.keys(value).sort();

    assert.deepEqual(sortedKeys(listSpace), schemaPropertyNames("PublicSpace"));
    assert.deepEqual(
      sortedKeys(detailSpace),
      schemaPropertyNames("PublicSpaceDetail"),
    );
    assert.deepEqual(sortedKeys(listSpace), [
      "area_m2",
      "base_price_clp",
      "category_code",
      "category_name",
      "comuna_code",
      "comuna_name",
      "cover_image_id",
      "currency",
      "description",
      "estimated_total_clp",
      "features",
      "images",
      "rate_unit",
      "region_code",
      "region_name",
      "space_id",
      "title",
      "verification",
    ]);
    assert.deepEqual(sortedKeys(detailSpace), [
      "area_m2",
      "base_price_clp",
      "category_code",
      "category_name",
      "comuna_code",
      "comuna_name",
      "currency",
      "description",
      "estimated_total_clp",
      "features",
      "images",
      "rate_unit",
      "region_code",
      "region_name",
      "space_id",
      "title",
      "usage_rules",
      "verification",
      "weekly_schedule",
    ]);

    for (const value of [listSpace, detailSpace]) {
      const serialized = JSON.stringify(value);
      for (const marker of hiddenMarkers)
        assert.equal(serialized.includes(marker), false, `${marker} leaked`);
      assert.deepEqual(sortedKeys(value.features[0]), [
        "code",
        "label",
        "value",
      ]);
      assert.deepEqual(value.features, [
        { code: "wifi", label: "Wi-Fi", value: true },
      ]);
      assert.deepEqual(sortedKeys(value.images[0]), [
        "image_id",
        "is_cover",
        "source",
      ]);
      assert.deepEqual(value.images, [
        {
          image_id: "22222222-2222-4222-8222-222222222222",
          is_cover: true,
          source: "media",
        },
      ]);
      assert.deepEqual(value.verification, {
        identityVerified: true,
        spaceVerified: true,
      });
      assert.deepEqual(sortedKeys(value.verification), [
        "identityVerified",
        "spaceVerified",
      ]);
      assert.equal(value.region_code, "13");
      assert.equal(value.region_name, "Región Metropolitana de Santiago");
      assert.equal(value.comuna_code, "13123");
      assert.equal(value.comuna_name, "Providencia");
    }
    assert.equal(
      listSpace.cover_image_id,
      "22222222-2222-4222-8222-222222222222",
    );
    assert.equal("cover_image_id" in detailSpace, false);
    assert.equal("weekly_schedule" in listSpace, false);
    assert.deepEqual(detailSpace.weekly_schedule, {
      enabled: true,
      time_zone: "America/Santiago",
      days: Array.from({ length: 7 }, (_, index) => ({
        weekday: index + 1,
        periods: index === 0 ? [{ open: "09:00", close: "17:00" }] : [],
      })),
    });
    assert.deepEqual(sortedKeys(detailSpace.weekly_schedule), [
      "days",
      "enabled",
      "time_zone",
    ]);
    assert.deepEqual(sortedKeys(detailSpace.weekly_schedule.days[0]), [
      "periods",
      "weekday",
    ]);
    assert.deepEqual(
      sortedKeys(detailSpace.weekly_schedule.days[0].periods[0]),
      ["close", "open"],
    );
  } finally {
    globalThis.fetch = original;
  }
});

test("public catalog adapter maps malformed success envelopes to unavailable", async () => {
  const original = globalThis.fetch;
  const malformedCases = [
    {
      name: "list root",
      body: null,
      load: () => adapter.listSpaces({ attribute: [] }),
    },
    {
      name: "list envelope",
      body: { items: null },
      load: () => adapter.listSpaces({ attribute: [] }),
    },
    {
      name: "list entry",
      body: { items: [null] },
      load: () => adapter.listSpaces({ attribute: [] }),
    },
    {
      name: "detail envelope",
      body: { data: null },
      load: () => adapter.getSpace(fixture.space_id),
    },
    {
      name: "detail root",
      body: null,
      load: () => adapter.getSpace(fixture.space_id),
    },
  ];

  try {
    for (const scenario of malformedCases) {
      globalThis.fetch = async () => Response.json(scenario.body);
      assert.deepEqual(
        await scenario.load(),
        { ok: false, error: { kind: "unavailable", status: 502 } },
        scenario.name,
      );
    }
  } finally {
    globalThis.fetch = original;
  }
});

test("public space adapters discard malformed nested values safely", async () => {
  const original = globalThis.fetch;
  const malformedWire = {
    ...fixture,
    features: [
      null,
      { code: 42, label: "Invalid", value: true },
      { code: "desk", label: "Desk", value: false, private_metadata: "hidden" },
    ],
    images: [
      null,
      { image_id: 7, is_cover: true, source: "media" },
      {
        image_id: "33333333-3333-4333-8333-333333333333",
        is_cover: false,
        source: "synthetic",
        private_metadata: "hidden",
      },
    ],
    verification: "malformed-private-value",
    weekly_schedule: {
      enabled: true,
      time_zone: "America/Santiago",
      days: Array.from({ length: 6 }, (_, index) => ({
        weekday: index + 1,
        periods: [],
      })),
    },
  };
  globalThis.fetch = async (url) =>
    Response.json(
      new URL(url).pathname.endsWith("/public/spaces")
        ? { items: [malformedWire] }
        : { data: malformedWire },
    );
  try {
    const listed = await adapter.listSpaces({ attribute: [] });
    const detailed = await adapter.getSpace(fixture.space_id);
    assert.equal(listed.ok, true);
    assert.equal(detailed.ok, true);

    for (const value of [listed.value.items[0], detailed.value]) {
      assert.deepEqual(value.features, [
        { code: "desk", label: "Desk", value: false },
      ]);
      assert.deepEqual(value.images, [
        {
          image_id: "33333333-3333-4333-8333-333333333333",
          is_cover: false,
          source: "synthetic",
        },
      ]);
      assert.deepEqual(value.verification, {
        identityVerified: false,
        spaceVerified: false,
      });
      for (const field of [
        "region_code",
        "region_name",
        "comuna_code",
        "comuna_name",
        "cover_image_id",
      ])
        assert.equal(Object.hasOwn(value, field), false);
    }
    assert.equal("weekly_schedule" in listed.value.items[0], false);
    assert.equal("weekly_schedule" in detailed.value, false);
  } finally {
    globalThis.fetch = original;
  }
});

test("public detail omits semantically invalid weekly periods", async () => {
  const original = globalThis.fetch;
  const invalidPeriodSets = [
    [{ open: "15:00", close: "14:00" }],
    [{ open: "14:00", close: "14:00" }],
    [
      { open: "09:00", close: "13:00" },
      { open: "12:00", close: "17:00" },
    ],
  ];
  globalThis.fetch = async () => {
    const periods = invalidPeriodSets.shift();
    assert.ok(periods);
    return Response.json({
      data: {
        ...fixture,
        weekly_schedule: {
          enabled: true,
          time_zone: "America/Santiago",
          days: Array.from({ length: 7 }, (_, index) => ({
            weekday: index + 1,
            periods: index === 0 ? periods : [],
          })),
        },
      },
    });
  };
  try {
    for (let index = 0; index < 3; index++) {
      const result = await adapter.getSpace(fixture.space_id);
      assert.equal(result.ok, true);
      assert.equal(Object.hasOwn(result.value, "weekly_schedule"), false);
    }
  } finally {
    globalThis.fetch = original;
  }
});

test("public detail preserves adjacent non-overlapping weekly periods", async () => {
  const original = globalThis.fetch;
  const periods = [
    { open: "09:00", close: "12:00" },
    { open: "12:00", close: "17:00" },
  ];
  globalThis.fetch = async () =>
    Response.json({
      data: {
        ...fixture,
        weekly_schedule: {
          enabled: true,
          time_zone: "America/Santiago",
          days: Array.from({ length: 7 }, (_, index) => ({
            weekday: index + 1,
            periods: index === 0 ? periods : [],
          })),
        },
      },
    });
  try {
    const result = await adapter.getSpace(fixture.space_id);
    assert.equal(result.ok, true);
    assert.deepEqual(result.value.weekly_schedule?.days[0]?.periods, periods);
  } finally {
    globalThis.fetch = original;
  }
});

test("public detail omits a weekly schedule with an invalid IANA time zone", async () => {
  const original = globalThis.fetch;
  globalThis.fetch = async () =>
    Response.json({
      data: {
        ...fixture,
        weekly_schedule: {
          enabled: true,
          time_zone: "../UTC",
          days: Array.from({ length: 7 }, (_, index) => ({
            weekday: index + 1,
            periods: index === 0 ? [{ open: "09:00", close: "17:00" }] : [],
          })),
        },
      },
    });
  try {
    const result = await adapter.getSpace(fixture.space_id);
    assert.equal(result.ok, true);
    assert.equal(Object.hasOwn(result.value, "weekly_schedule"), false);
  } finally {
    globalThis.fetch = original;
  }
});

test("public detail accepts unordered non-overlapping periods without reordering them", async () => {
  const original = globalThis.fetch;
  const periods = [
    { open: "16:00", close: "18:00" },
    { open: "09:00", close: "12:00" },
  ];
  globalThis.fetch = async () =>
    Response.json({
      data: {
        ...fixture,
        weekly_schedule: {
          enabled: true,
          time_zone: "America/Santiago",
          days: Array.from({ length: 7 }, (_, index) => ({
            weekday: index + 1,
            periods: index === 0 ? periods : [],
          })),
        },
      },
    });
  try {
    const result = await adapter.getSpace(fixture.space_id);
    assert.equal(result.ok, true);
    assert.deepEqual(result.value.weekly_schedule?.days[0]?.periods, periods);
  } finally {
    globalThis.fetch = original;
  }
});

test("verification disclaimers are readable without relying on hover titles", () => {
  const markup = render({ identityVerified: true, spaceVerified: true });
  assert.match(markup, /class="sr-only"[^>]*>[^<]*no acredita/);
});

test.after(async () => rm(temporary, { recursive: true, force: true }));
