import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import ts from "typescript";
import { parse } from "yaml";

const openApi = parse(
  await readFile(
    new URL("../../planning/openapi.yaml", import.meta.url),
    "utf8",
  ),
);

const temporary = await mkdtemp(
  path.join(
    process.env.TMPDIR ?? os.tmpdir(),
    "host-listings-adapter-contract-",
  ),
);
const sourcePath = new URL(
  "../src/features/host-listings/infrastructure/host-listings-openapi-adapter.ts",
  import.meta.url,
);
const errorsSourcePath = new URL("../src/lib/api/errors.ts", import.meta.url);
const validationSourcePath = new URL(
  "../src/features/host-listings/domain/validation.ts",
  import.meta.url,
);
let source = await readFile(sourcePath, "utf8");
const errorsSource = await readFile(errorsSourcePath, "utf8");
const validationSource = await readFile(validationSourcePath, "utf8");
const replacements = [
  ['import "client-only";', ""],
  ['import { apiClient } from "@/lib/api/client";', "const apiClient = {};"],
  [
    'import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";',
    'import { ApiProtocolError } from "./errors.mjs";\nconst normalizeApiError = () => new Error("synthetic");',
  ],
  [
    'import { serializeExactJson } from "@/lib/api/exact-json";',
    "const serializeExactJson = (value) => JSON.stringify(value);",
  ],
  [
    'import {\n  convertSlotsToWeeklyDays,\n  generateIdempotencyKey,\n  isValidUUID,\n} from "../domain/validation";',
    'import {\n  convertSlotsToWeeklyDays,\n  generateIdempotencyKey,\n  isValidUUID,\n} from "./validation.mjs";',
  ],
  [
    "function mapPhotoToGalleryPhoto(",
    "export function mapPhotoToGalleryPhoto(",
  ],
];
for (const [from, to] of replacements) {
  assert.ok(source.includes(from), `Missing expected adapter source: ${from}`);
  source = source.replace(from, to);
}
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;
const compiledErrors = ts.transpileModule(errorsSource, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;
const compiledValidation = ts.transpileModule(validationSource, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;
const modulePath = path.join(temporary, "adapter.mjs");
await writeFile(path.join(temporary, "errors.mjs"), compiledErrors);
await writeFile(path.join(temporary, "validation.mjs"), compiledValidation);
await writeFile(modulePath, compiled);
const { mapPhotoToGalleryPhoto } = await import(pathToFileURL(modulePath));
const { ApiProtocolError } = await import(
  pathToFileURL(path.join(temporary, "errors.mjs"))
);

test("gallery media contract excludes profile-only avatar variants", () => {
  const schemas = openApi.components.schemas;
  const variants = schemas.SpaceMediaPhoto.properties.variants;
  assert.equal(variants.items.$ref, "#/components/schemas/SpaceMediaVariant");
  assert.deepEqual(schemas.SpaceMediaVariant.properties.name.enum, [
    "display",
    "thumbnail",
  ]);
});

test("pending photo without created_at stays without a fabricated timestamp", () => {
  const pending = mapPhotoToGalleryPhoto({
    id: "11111111-1111-4111-8111-111111111111",
    space_id: "22222222-2222-4222-8222-222222222222",
    state: "pending",
  });

  assert.equal(pending.state, "pending");
  assert.equal(Object.hasOwn(pending, "created_at"), false);
});

test("ready photo preserves its validated timestamp and contract variants", () => {
  const spaceId = "22222222-2222-4222-8222-222222222222";
  const photoId = "11111111-1111-4111-8111-111111111111";
  const createdAt = "2026-11-11T12:00:00Z";
  const ready = mapPhotoToGalleryPhoto({
    id: photoId,
    space_id: spaceId,
    alt_text: "Foto de prueba",
    visible: true,
    cover: false,
    position: 1,
    state: "ready",
    created_at: createdAt,
    variants: ["thumbnail", "display"].map((name) => ({
      name,
      url: `/api/v1/spaces/${spaceId}/gallery/${photoId}/variants/${name}`,
      mime_type: "image/webp",
      size_bytes: 42,
      sha256: "a".repeat(64),
      width: 10,
      height: 10,
    })),
  });

  assert.equal(ready.created_at, createdAt);
  assert.deepEqual(
    ready.variants.map(({ name, url }) => ({ name, url })),
    ["thumbnail", "display"].map((name) => ({
      name,
      url: `/api/backend/spaces/${spaceId}/gallery/${photoId}/variants/${name}`,
    })),
  );
});

test("ready photo accepts either single supported variant allowed by OpenAPI", () => {
  const spaceId = "22222222-2222-4222-8222-222222222222";
  const photoId = "11111111-1111-4111-8111-111111111111";
  for (const name of ["display", "thumbnail"]) {
    const photo = mapPhotoToGalleryPhoto({
      id: photoId,
      space_id: spaceId,
      alt_text: "Foto de prueba",
      visible: true,
      cover: false,
      position: 1,
      state: "ready",
      created_at: "2026-11-11T12:00:00Z",
      variants: [
        {
          name,
          url: `/api/v1/spaces/${spaceId}/gallery/${photoId}/variants/${name}`,
          mime_type: "image/webp",
          size_bytes: 42,
          sha256: "a".repeat(64),
          width: 10,
          height: 10,
        },
      ],
    });

    assert.deepEqual(
      photo.variants.map(({ name: mappedName, url }) => ({
        name: mappedName,
        url,
      })),
      [
        {
          name,
          url: `/api/backend/spaces/${spaceId}/gallery/${photoId}/variants/${name}`,
        },
      ],
    );
  }
});

test("ready photo accepts Go RFC3339Nano fractional seconds and offsets", () => {
  const spaceId = "22222222-2222-4222-8222-222222222222";
  const photoId = "11111111-1111-4111-8111-111111111111";
  const photo = {
    id: photoId,
    space_id: spaceId,
    alt_text: "Foto de prueba",
    visible: true,
    cover: false,
    position: 1,
    state: "ready",
    variants: [
      {
        name: "display",
        url: `/api/v1/spaces/${spaceId}/gallery/${photoId}/variants/display`,
        mime_type: "image/webp",
        size_bytes: 42,
        sha256: "a".repeat(64),
        width: 10,
        height: 10,
      },
    ],
  };

  for (const createdAt of [
    "2026-11-11T12:00:00.123456789Z",
    "2026-11-11T12:00:00+05:30",
  ]) {
    const mapped = mapPhotoToGalleryPhoto({ ...photo, created_at: createdAt });
    assert.equal(mapped.created_at, createdAt);
  }

  assert.throws(
    () =>
      mapPhotoToGalleryPhoto({
        ...photo,
        created_at: "2026-11-11T12:00:00+24:00",
      }),
    ApiProtocolError,
  );
});

test("ready photo rejects absent or invalid timestamp and unusable variants", () => {
  const spaceId = "22222222-2222-4222-8222-222222222222";
  const photoId = "11111111-1111-4111-8111-111111111111";
  const valid = {
    id: photoId,
    space_id: spaceId,
    alt_text: "Foto de prueba",
    visible: true,
    cover: false,
    position: 1,
    state: "ready",
    created_at: "2026-11-11T12:00:00Z",
    variants: [
      {
        name: "display",
        url: `/api/v1/spaces/${spaceId}/gallery/${photoId}/variants/display`,
        mime_type: "image/webp",
        size_bytes: 42,
        sha256: "a".repeat(64),
        width: 10,
        height: 10,
      },
    ],
  };

  for (const invalid of [
    { ...valid, created_at: undefined },
    { ...valid, created_at: "2026-02-30T12:00:00Z" },
    { ...valid, variants: undefined },
    { ...valid, variants: [{ ...valid.variants[0], mime_type: "image/png" }] },
  ])
    assert.throws(() => mapPhotoToGalleryPhoto(invalid), ApiProtocolError);
});

test("ready photo rejects missing or invalid required metadata", () => {
  const spaceId = "22222222-2222-4222-8222-222222222222";
  const photoId = "11111111-1111-4111-8111-111111111111";
  const valid = {
    id: photoId,
    space_id: spaceId,
    alt_text: "Foto de prueba",
    visible: true,
    cover: false,
    position: 1,
    state: "ready",
    created_at: "2026-11-11T12:00:00Z",
    variants: [
      {
        name: "display",
        url: `/api/v1/spaces/${spaceId}/gallery/${photoId}/variants/display`,
        mime_type: "image/webp",
        size_bytes: 42,
        sha256: "a".repeat(64),
        width: 10,
        height: 10,
      },
    ],
  };
  const missingVisibility = { ...valid };
  delete missingVisibility.visible;
  const missingCover = { ...valid };
  delete missingCover.cover;
  const missingAltText = { ...valid };
  delete missingAltText.alt_text;
  for (const invalid of [
    missingVisibility,
    missingCover,
    missingAltText,
    { ...valid, position: 0 },
    { ...valid, position: 31 },
    { ...valid, alt_text: "x".repeat(251) },
    { ...valid, id: "not-a-uuid" },
    { ...valid, space_id: "not-a-uuid" },
    { ...valid, state: "uploading" },
  ])
    assert.throws(() => mapPhotoToGalleryPhoto(invalid), ApiProtocolError);
});

test("ready photo rejects malformed variants and schema-bound violations", () => {
  const spaceId = "22222222-2222-4222-8222-222222222222";
  const photoId = "11111111-1111-4111-8111-111111111111";
  const variant = (name) => ({
    name,
    url: `/api/v1/spaces/${spaceId}/gallery/${photoId}/variants/${name}`,
    mime_type: "image/webp",
    size_bytes: 42,
    sha256: "a".repeat(64),
    width: 10,
    height: 10,
  });
  const validPhoto = (variants) =>
    mapPhotoToGalleryPhoto({
      id: photoId,
      space_id: spaceId,
      alt_text: "Foto de prueba",
      visible: true,
      cover: false,
      position: 1,
      state: "ready",
      created_at: "2026-11-11T12:00:00Z",
      variants,
    });

  const display = variant("display");
  const thumbnail = variant("thumbnail");
  for (const invalid of [
    variant("avatar"),
    { ...thumbnail, mime_type: "image/png" },
    { ...thumbnail, url: "https://example.invalid/photo.webp" },
    { ...thumbnail, size_bytes: 0 },
    { ...thumbnail, size_bytes: 10_485_761 },
    { ...thumbnail, sha256: "A".repeat(64) },
    { ...thumbnail, width: 0 },
    { ...thumbnail, width: 1601 },
    { ...thumbnail, height: 1.5 },
  ])
    assert.throws(() => validPhoto([display, invalid]), ApiProtocolError);

  assert.throws(() => validPhoto([]), ApiProtocolError);
  assert.throws(
    () => validPhoto([display, thumbnail, display]),
    ApiProtocolError,
  );
});

test("pending photo requires valid identifiers", () => {
  for (const invalid of [
    {
      id: "not-a-uuid",
      space_id: "22222222-2222-4222-8222-222222222222",
      state: "pending",
    },
    {
      id: "11111111-1111-4111-8111-111111111111",
      space_id: "not-a-uuid",
      state: "pending",
    },
  ])
    assert.throws(() => mapPhotoToGalleryPhoto(invalid), ApiProtocolError);
});

test.after(async () => rm(temporary, { recursive: true, force: true }));
