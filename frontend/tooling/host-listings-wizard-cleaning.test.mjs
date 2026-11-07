import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

async function load(path) {
  const source = await readFile(new URL(path, import.meta.url), "utf8");
  const javascript = ts.transpile(source, { module: ts.ModuleKind.CommonJS });
  const loadedModule = { exports: {} };
  new Function("module", "exports", javascript)(
    loadedModule,
    loadedModule.exports,
  );
  return loadedModule.exports;
}

const {
  cleanAttributesForPayload,
  formatAttributeLabel,
  formatAttributeValue,
} = await load("../src/features/host-listings/domain/attribute-formatting.ts");
const { serializeExactJson } = await load("../src/lib/api/exact-json.ts");
const { validateCategoryAttributes } = await load(
  "../src/features/host-listings/domain/validation.ts",
);

test("cleans absent optional attribute values before real exact JSON serialization", () => {
  const values = {
    optional: undefined,
    blank: "  ",
    absent: null,
    enabled: false,
    count: 0,
    selected: ["a", ""],
  };
  const clean = cleanAttributesForPayload(values);
  assert.deepEqual(clean, {
    enabled: false,
    count: 0,
    selected: ["a", ""],
  });
  assert.doesNotThrow(() => serializeExactJson(clean));
});

test("does not convert invalid non-finite numeric values into absent values", () => {
  assert.throws(() =>
    serializeExactJson(cleanAttributesForPayload({ bad: NaN })),
  );
});

test("preserves nonblank strings byte-for-byte and leaves invalid enum-list members for validation", () => {
  const schema = {
    category_code: "test",
    schema_version: 1,
    attributes: [
      {
        code: "required_text",
        label: "Texto requerido",
        type: "enum",
        required: true,
        order: 1,
        options: [" opcion "],
        filter_candidate: false,
      },
      {
        code: "choices",
        label: "Opciones",
        type: "enum_list",
        order: 2,
        options: ["a"],
        filter_candidate: false,
      },
    ],
  };
  const cleaned = cleanAttributesForPayload({
    required_text: " opcion ",
    choices: ["a", "invalid"],
    unknown: true,
    optional: undefined,
  });
  assert.deepEqual(cleaned, {
    required_text: " opcion ",
    choices: ["a", "invalid"],
    unknown: true,
  });
  assert.deepEqual(validateCategoryAttributes(cleaned, schema).errors, {
    choices: "Contiene opciones no válidas para 'Opciones'.",
  });
  assert.equal(
    validateCategoryAttributes(
      { ...cleaned, choices: ["a"], enabled: false, count: 0 },
      schema,
    ).valid,
    true,
  );
});

test("required missing values fail while optional boolean false and integer zero remain valid", async () => {
  const fixture = JSON.parse(
    await readFile(
      new URL("../tests/fixtures/category-profiles-v1.json", import.meta.url),
      "utf8",
    ),
  );
  const inmueble = fixture.profiles.inmuebles;
  const required = inmueble.attributes.find((attribute) => attribute.required);
  const optionalBoolean = inmueble.attributes.find(
    (attribute) => attribute.type === "boolean" && !attribute.required,
  );
  const optionalInteger = inmueble.attributes.find(
    (attribute) => attribute.type === "integer" && !attribute.required,
  );
  assert.ok(required, "fixture must expose an actual required definition");
  assert.ok(
    optionalBoolean,
    "fixture must expose an actual boolean definition",
  );
  assert.ok(
    optionalInteger,
    "fixture must expose an actual integer definition",
  );

  const schema = {
    category_code: inmueble.category_code,
    schema_version: inmueble.schema_version,
    attributes: [required, optionalBoolean, optionalInteger],
  };
  const withoutRequired = {
    [optionalBoolean.code]: false,
    [optionalInteger.code]: 0,
  };
  assert.deepEqual(validateCategoryAttributes(withoutRequired, schema).errors, {
    [required.code]: `El atributo '${required.label}' es obligatorio.`,
  });
  assert.deepEqual(
    validateCategoryAttributes(
      { ...withoutRequired, [required.code]: required.options[0] },
      schema,
    ).errors,
    {},
  );
  assert.deepEqual(
    cleanAttributesForPayload({
      ...withoutRequired,
      [required.code]: required.options[0],
    }),
    {
      [required.code]: required.options[0],
      [optionalBoolean.code]: false,
      [optionalInteger.code]: 0,
    },
  );
});

test("keeps unknown keys intact without inventing an unknown-key validation rule", () => {
  const schema = {
    category_code: "test",
    schema_version: 1,
    attributes: [],
  };
  const attributes = { server_extension: "preserve-me" };
  const cleaned = cleanAttributesForPayload(attributes);
  assert.deepEqual(cleaned, attributes);
  assert.equal(validateCategoryAttributes(cleaned, schema).valid, true);
});

test("renders schema labels and readable values without raw technical keys", () => {
  const schema = {
    attributes: [
      {
        code: "tipo_inmueble",
        label: "Tipo de propiedad",
        options: ["casa_campo"],
      },
      { code: "aire_acondicionado", label: "Aire acondicionado" },
    ],
  };
  assert.equal(
    formatAttributeLabel("tipo_inmueble", schema),
    "Tipo de propiedad",
  );
  assert.equal(
    formatAttributeValue("casa_campo", schema.attributes[0]),
    "Casa campo",
  );
  assert.equal(formatAttributeValue(true, schema.attributes[1]), "Sí");
  assert.equal(formatAttributeValue(false, schema.attributes[1]), "No");
  assert.equal(
    formatAttributeLabel("atributo_desconocido", schema),
    "Atributo desconocido",
  );
});
