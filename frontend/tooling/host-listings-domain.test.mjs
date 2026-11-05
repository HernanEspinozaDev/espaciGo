import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(
    process.env.TMPDIR ?? os.tmpdir(),
    "espacigo-host-listings-domain-",
  ),
);

async function compile(sourcePath, outputName, replacements = []) {
  let source = await readFile(path.join(root, sourcePath), "utf8");
  for (const [before, after] of replacements) {
    source = source.replaceAll(before, after);
  }
  const output = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText;
  const destination = path.join(scratch, outputName);
  await writeFile(destination, output);
  return destination;
}

const typesPath = await compile(
  "src/features/host-listings/domain/types.ts",
  "types.mjs",
);
const validationPath = await compile(
  "src/features/host-listings/domain/validation.ts",
  "validation.mjs",
  [["./types", pathToFileURL(typesPath).href]],
);

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

test("host-listings domain validates UUID and generates valid Idempotency-Key", async () => {
  const { isValidUUID, generateIdempotencyKey } = await import(
    pathToFileURL(validationPath).href
  );

  assert.equal(isValidUUID("123e4567-e89b-12d3-a456-426614174000"), true);
  assert.equal(isValidUUID("d3b07384-d113-4632-a6f9-03a89a05b38a"), true);
  assert.equal(isValidUUID("invalid-uuid"), false);
  assert.equal(isValidUUID(""), false);
  assert.equal(isValidUUID(null), false);
  assert.equal(isValidUUID(12345), false);

  const generatedKey = generateIdempotencyKey();
  assert.equal(typeof generatedKey, "string");
  assert.equal(isValidUUID(generatedKey), true);
});

test("host-listings domain enforces CLP pricing, area and capacity rules", async () => {
  const { isValidPriceCLP, isValidAreaM2, isValidCapacity } = await import(
    pathToFileURL(validationPath).href
  );

  // Price CLP must be positive safe integer
  assert.equal(isValidPriceCLP(15000), true);
  assert.equal(isValidPriceCLP(1), false);
  assert.equal(isValidPriceCLP(5001), true);
  assert.equal(isValidPriceCLP(0), false);
  assert.equal(isValidPriceCLP(-100), false);
  assert.equal(isValidPriceCLP(15000.5), false);
  assert.equal(isValidPriceCLP("15000"), false);
  assert.equal(isValidPriceCLP(NaN), false);
  assert.equal(isValidPriceCLP(Infinity), false);

  // Area m2 must be finite positive number
  assert.equal(isValidAreaM2(25), true);
  assert.equal(isValidAreaM2(12.5), true);
  assert.equal(isValidAreaM2(0), false);
  assert.equal(isValidAreaM2(-5), false);
  assert.equal(isValidAreaM2(Infinity), false);

  // Capacity must be safe integer >= 1
  assert.equal(isValidCapacity(1), true);
  assert.equal(isValidCapacity(50), true);
  assert.equal(isValidCapacity(0), false);
  assert.equal(isValidCapacity(2.5), false);
  assert.equal(isValidCapacity(-1), false);
});

test("host-listings domain validates draft input fields", async () => {
  const { validateListingDraftInput } = await import(
    pathToFileURL(validationPath).href
  );

  const validDraft = {
    category_code: "oficina",
    title: "Oficina Privada Providencia",
    description:
      "Excelente iluminación natural y conectividad de alta velocidad, a pasos del metro y con acceso controlado durante todo el día.",
    area_m2: 35,
    capacity: 6,
    usage_rules: "No fumar, silencio en pasillos.",
    rate_unit: "hora",
    base_price_clp: 18000,
    address: "Av. Providencia 1234, Of. 501",
  };

  const resValid = validateListingDraftInput(validDraft);
  assert.equal(resValid.valid, true);
  assert.deepEqual(resValid.errors, {});

  const resInvalid = validateListingDraftInput({
    category_code: "",
    title: "a".repeat(125),
    description: "   ",
    area_m2: -10,
    capacity: 0,
    base_price_clp: 0,
    rate_unit: "semana",
    address: "",
  });

  assert.equal(resInvalid.valid, false);
  assert.ok(resInvalid.errors.title);
  assert.ok(resInvalid.errors.description);
  assert.ok(resInvalid.errors.category_code);
  assert.ok(resInvalid.errors.area_m2);
  assert.ok(resInvalid.errors.capacity);
  assert.ok(resInvalid.errors.base_price_clp);
  assert.ok(resInvalid.errors.rate_unit);
  assert.ok(resInvalid.errors.address);
});

test("host-listings common fields match Go rune, trim, and numeric constraints", async () => {
  const {
    isValidPriceCLP,
    isValidAreaM2,
    isValidCapacity,
    validateListingDraftInput,
  } = await import(pathToFileURL(validationPath).href);

  assert.equal(isValidPriceCLP(5000), false);
  assert.equal(isValidPriceCLP(5001), true);
  assert.equal(isValidAreaM2(0.01), true);
  assert.equal(isValidAreaM2(0.001), false);
  assert.equal(isValidAreaM2(1.234), false);
  assert.equal(isValidAreaM2(99999999.99), true);
  assert.equal(isValidAreaM2(100000000), false);
  assert.equal(isValidCapacity(100001), true);
  assert.equal(isValidCapacity(2147483647), true);
  assert.equal(isValidCapacity(2147483648), false);

  const base = {
    category_code: "x",
    title: "t".repeat(71),
    description: "d".repeat(100),
    area_m2: 1,
    capacity: 1,
    usage_rules: "ok",
    rate_unit: "hora",
    base_price_clp: 5001,
    address: "a",
  };
  assert.ok(
    validateListingDraftInput({ ...base, title: "😀".repeat(70) }).errors
      .title === undefined,
  );
  assert.ok(
    validateListingDraftInput({ ...base, title: "😀".repeat(71) }).errors.title,
  );
  assert.ok(
    validateListingDraftInput({ ...base, title: "  " + "x".repeat(70) }).errors
      .title,
  );
  assert.ok(
    validateListingDraftInput({ ...base, title: "  " + "x".repeat(71) }).errors
      .title,
  );
  assert.equal(
    validateListingDraftInput({ ...base, description: "  " + "d".repeat(100) })
      .errors.description,
    undefined,
  );
  assert.ok(
    validateListingDraftInput({ ...base, usage_rules: "   " }).errors
      .usage_rules,
  );
  assert.ok(
    validateListingDraftInput({ ...base, usage_rules: "x".repeat(251) }).errors
      .usage_rules,
  );
  assert.ok(
    validateListingDraftInput({ ...base, usage_rules: undefined }).errors
      .usage_rules,
  );
  assert.equal(
    validateListingDraftInput({ ...base, address: "😀".repeat(500) }).errors
      .address,
    undefined,
  );
  assert.ok(
    validateListingDraftInput({ ...base, address: "😀".repeat(501) }).errors
      .address,
  );
});

test("host-listings domain validates dynamic category attribute schemas and preserves version", async () => {
  const { validateCategoryAttributes, preserveAttributeSchemaVersion } =
    await import(pathToFileURL(validationPath).href);

  const mockSchema = {
    category_code: "oficina",
    schema_version: 3,
    attributes: [
      {
        code: "has_wifi",
        label: "Wi-Fi Alta Velocidad",
        type: "boolean",
        required: true,
        order: 1,
        filter_candidate: true,
      },
      {
        code: "workstations",
        label: "Puestos de trabajo",
        type: "integer",
        required: true,
        order: 2,
        minimum: 1,
        maximum: 20,
        filter_candidate: true,
      },
      {
        code: "access_type",
        label: "Tipo de Acceso",
        type: "enum",
        required: false,
        order: 3,
        options: ["llave", "tarjeta", "biometrico"],
        filter_candidate: false,
      },
      {
        code: "amenities",
        label: "Comodidades",
        type: "enum_list",
        required: false,
        order: 4,
        options: ["cafe", "pizarra", "proyector", "climatizacion"],
        filter_candidate: true,
      },
    ],
  };

  // Valid attributes
  const validAttrs = {
    has_wifi: true,
    workstations: 4,
    access_type: "tarjeta",
    amenities: ["cafe", "climatizacion"],
  };
  const resValid = validateCategoryAttributes(validAttrs, mockSchema);
  assert.equal(resValid.valid, true);
  assert.deepEqual(resValid.errors, {});

  // Missing required attribute & invalid types
  const invalidAttrs = {
    has_wifi: "yes", // must be boolean
    workstations: 25, // exceeds maximum 20
    access_type: "codigo_invalido", // not in options
    amenities: ["piscina"], // unlisted option
  };
  const resInvalid = validateCategoryAttributes(invalidAttrs, mockSchema);
  assert.equal(resInvalid.valid, false);
  assert.ok(resInvalid.errors.has_wifi);
  assert.ok(resInvalid.errors.workstations);
  assert.ok(resInvalid.errors.access_type);
  assert.ok(resInvalid.errors.amenities);

  // Missing required
  const missingRequired = validateCategoryAttributes({}, mockSchema);
  assert.equal(missingRequired.valid, false);
  assert.ok(missingRequired.errors.has_wifi);
  assert.ok(missingRequired.errors.workstations);

  // Mandatory preservation of attribute_schema_version
  const input = {
    category_code: "anterior",
    title: "Test",
    description: "Desc",
    area_m2: 20,
    capacity: 2,
    usage_rules: "Rules",
    rate_unit: "hora",
    base_price_clp: 10000,
    address: "Calle 123",
  };
  const preserved = preserveAttributeSchemaVersion(input, mockSchema);
  assert.equal(preserved.category_code, "oficina");
  assert.equal(preserved.attribute_schema_version, 3);
});

test("host-listings domain validates time format, day normalization and weekly schedule slots", async () => {
  const {
    isValidTimeFormat,
    normalizeDayOfWeek,
    denormalizeDayOfWeek,
    validateScheduleSlots,
    convertSlotsToWeeklyDays,
  } = await import(pathToFileURL(validationPath).href);

  // Time format
  assert.equal(isValidTimeFormat("08:30"), true);
  assert.equal(isValidTimeFormat("00:00"), true);
  assert.equal(isValidTimeFormat("23:59"), true);
  assert.equal(isValidTimeFormat("24:00", false), false);
  assert.equal(isValidTimeFormat("24:00", true), true);
  assert.equal(isValidTimeFormat("25:00"), false);
  assert.equal(isValidTimeFormat("8:30"), false);
  assert.equal(isValidTimeFormat("abc"), false);

  // Day normalization: 0..6 (Sun=0) -> 1..7 (Sun=7)
  assert.equal(normalizeDayOfWeek(0), 7);
  assert.equal(normalizeDayOfWeek(1), 1);
  assert.equal(normalizeDayOfWeek(6), 6);
  assert.equal(normalizeDayOfWeek(7), 7);
  assert.throws(() => normalizeDayOfWeek(8), RangeError);
  assert.throws(() => normalizeDayOfWeek(-1), RangeError);

  assert.equal(denormalizeDayOfWeek(7), 0);
  assert.equal(denormalizeDayOfWeek(1), 1);
  assert.equal(denormalizeDayOfWeek(6), 6);

  // Schedule slots validation
  const validSlots = [
    { day_of_week: 1, start_time: "09:00", end_time: "13:00" },
    { day_of_week: 1, start_time: "14:00", end_time: "18:00" },
    { day_of_week: 2, start_time: "10:00", end_time: "24:00" },
  ];
  const validSlotsRes = validateScheduleSlots(validSlots);
  assert.equal(validSlotsRes.valid, true);
  assert.equal(validSlotsRes.errors.length, 0);

  // Invalid: open >= close
  const invertedRes = validateScheduleSlots([
    { day_of_week: 1, start_time: "18:00", end_time: "09:00" },
  ]);
  assert.equal(invertedRes.valid, false);

  // Invalid: > 2 periods in one day
  const tooManyPeriods = validateScheduleSlots([
    { day_of_week: 1, start_time: "08:00", end_time: "10:00" },
    { day_of_week: 1, start_time: "11:00", end_time: "13:00" },
    { day_of_week: 1, start_time: "14:00", end_time: "16:00" },
  ]);
  assert.equal(tooManyPeriods.valid, false);

  // Invalid: overlapping periods in same day
  const overlappingRes = validateScheduleSlots([
    { day_of_week: 1, start_time: "09:00", end_time: "13:00" },
    { day_of_week: 1, start_time: "12:00", end_time: "15:00" },
  ]);
  assert.equal(overlappingRes.valid, false);

  // Convert slots to 7 weekly days
  const weeklyDays = convertSlotsToWeeklyDays(validSlots);
  assert.equal(weeklyDays.length, 7);
  assert.equal(weeklyDays[0].weekday, 1);
  assert.equal(weeklyDays[0].periods.length, 2);
  assert.equal(weeklyDays[0].periods[0].open, "09:00");
  assert.equal(weeklyDays[0].periods[1].open, "14:00");
  assert.equal(weeklyDays[1].weekday, 2);
  assert.equal(weeklyDays[1].periods.length, 1);
  assert.equal(weeklyDays[1].periods[0].close, "24:00");
  assert.equal(weeklyDays[2].periods.length, 0); // día cerrado
});

test("host-listings domain enforces publication eligibility and IANA timezone requirement", async () => {
  const { canPublishListing, isValidIanaTimeZone } = await import(
    pathToFileURL(validationPath).href
  );

  assert.equal(isValidIanaTimeZone("America/Santiago"), true);
  assert.equal(isValidIanaTimeZone("UTC"), true);
  assert.equal(isValidIanaTimeZone("Invalida/Zona"), false);
  assert.equal(isValidIanaTimeZone(""), false);
  assert.equal(isValidIanaTimeZone(null), false);

  const readyListing = {
    id: "123e4567-e89b-12d3-a456-426614174000",
    category_code: "oficina",
    category_name: "Oficina",
    title: "Espacio Creativo Providencia",
    description: "Oficina completamente equipada.",
    area_m2: 40,
    capacity: 8,
    usage_rules: "Cuidado de mobiliario.",
    rate_unit: "hora",
    base_price_clp: 20000,
    address: "Providencia 100",
    state: "borrador",
    time_zone: "America/Santiago",
    attribute_schema_version: 1,
    attributes: {},
  };

  const canPublish = canPublishListing(readyListing);
  assert.equal(canPublish.eligible, true);
  assert.equal(canPublish.reasons.length, 0);

  // Missing time_zone prevents publication per backend rule
  const withoutTz = { ...readyListing, time_zone: null };
  const cannotPublish = canPublishListing(withoutTz);
  assert.equal(cannotPublish.eligible, false);
  assert.ok(cannotPublish.reasons.some((r) => r.includes("zona horaria IANA")));

  // Missing title or invalid price
  const incomplete = {
    ...readyListing,
    title: "",
    base_price_clp: 0,
  };
  const incRes = canPublishListing(incomplete);
  assert.equal(incRes.eligible, false);
  assert.ok(incRes.reasons.some((r) => r.includes("título")));
  assert.ok(incRes.reasons.some((r) => r.includes("precio base")));
});
