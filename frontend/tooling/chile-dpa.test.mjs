import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import test from "node:test";

const require = createRequire(import.meta.url);
const ts = require("typescript");
const here = new URL("../", import.meta.url);
const sourcePath = fileURLToPath(
  new URL("src/features/host-listings/domain/chile-dpa.ts", here),
);
const source = readFileSync(sourcePath, "utf8");
const javascript = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  },
}).outputText;
const loaded = { exports: {} };
new Function("exports", "require", javascript)(loaded.exports, require);
const { CHILE_REGIONS, CHILE_COMUNAS, getComunasByRegion } = loaded.exports;

const locationPath = fileURLToPath(
  new URL("src/features/host-listings/domain/wizard-location.ts", here),
);
const locationSource = readFileSync(locationPath, "utf8");
const locationJavascript = ts.transpileModule(locationSource, {
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  },
}).outputText;
const locationModule = { exports: {} };
new Function("exports", "require", locationJavascript)(
  locationModule.exports,
  (specifier) => {
    assert.equal(specifier, "./chile-dpa");
    return loaded.exports;
  },
);
const { validateWizardLocation } = locationModule.exports;

function readCsv(relativePath) {
  const text = readFileSync(
    fileURLToPath(new URL(relativePath, here)),
    "utf8",
  ).trimEnd();
  const [header, ...lines] = text.split(/\r?\n/);
  const columns = header.split(",");
  return lines.map((line) => {
    const fields = line.split(",");
    assert.equal(
      fields.length,
      columns.length,
      `CSV row width in ${relativePath}: ${line}`,
    );
    return Object.fromEntries(
      columns.map((column, index) => [column, fields[index]]),
    );
  });
}
const baseRows = readCsv("../db/reference/dpa_cl_cut_2018.csv");
const supplementRows = readCsv(
  "../db/reference/dpa_cl_supplement_2026-11-06.csv",
);

test("module exactly covers base CUT snapshot plus sourced supplements", () => {
  const expected = [...baseRows, ...supplementRows];
  const byCode = new Map();
  for (const row of expected) {
    assert.ok(
      !byCode.has(row.commune_code),
      `duplicate source code ${row.commune_code}`,
    );
    byCode.set(row.commune_code, row);
  }
  const expectedRegions = new Map();
  for (const row of expected) {
    const prior = expectedRegions.get(row.region_code);
    assert.ok(
      !prior || prior === row.region_name,
      `conflicting source region name ${row.region_code}`,
    );
    expectedRegions.set(row.region_code, row.region_name);
  }
  assert.deepEqual(
    [...CHILE_REGIONS.map(({ code, name }) => [code, name])].sort(),
    [...expectedRegions].sort(),
    "missing, extra or mismatched source region",
  );
  const actualByCode = new Map();
  for (const comuna of CHILE_COMUNAS) {
    assert.ok(
      !actualByCode.has(comuna.code),
      `duplicate module code ${comuna.code}`,
    );
    actualByCode.set(comuna.code, comuna);
  }
  assert.deepEqual(
    [...actualByCode.keys()].sort(),
    [...byCode.keys()].sort(),
    "missing or extra commune codes",
  );
  for (const [code, row] of byCode) {
    const actual = actualByCode.get(code);
    assert.equal(actual.name, row.commune_name, `name mismatch at ${code}`);
    assert.equal(
      actual.regionCode,
      row.region_code,
      `region mismatch at ${code}`,
    );
  }
  assert.equal(supplementRows.length, 1);
  assert.equal(
    supplementRows[0].source_url,
    "https://www.suseso.gob.cl/606/articles-19445_recurso_1.pdf",
  );
  assert.equal(supplementRows[0].verified_on, "2026-11-06");
});

test("location validation accepts only catalog region/comuna pairs and permits both blank", () => {
  assert.deepEqual(validateWizardLocation("", ""), { valid: true });
  assert.deepEqual(validateWizardLocation("13", "13101"), { valid: true });
  assert.equal(validateWizardLocation("13", "13999").valid, false);
  assert.equal(validateWizardLocation("99", "99101").valid, false);
  assert.equal(validateWizardLocation("12", "12202").valid, false);
  assert.equal(validateWizardLocation("", "13101").valid, false);
});

test("all region and commune codes are unique, well-formed and consistently linked", () => {
  const regionCodes = new Set(CHILE_REGIONS.map(({ code }) => code));
  assert.equal(regionCodes.size, CHILE_REGIONS.length);
  assert.ok(CHILE_REGIONS.every(({ code }) => /^\d{2}$/.test(code)));
  assert.equal(
    new Set(CHILE_COMUNAS.map(({ code }) => code)).size,
    CHILE_COMUNAS.length,
  );
  assert.ok(
    CHILE_COMUNAS.every(
      ({ code, regionCode }) =>
        /^\d{5}$/.test(code) &&
        regionCodes.has(regionCode) &&
        code.startsWith(regionCode),
    ),
  );
  assert.ok(
    CHILE_REGIONS.some(({ code, name }) => code === "16" && name === "Ñuble"),
  );
  assert.deepEqual(getComunasByRegion("99"), []);
});

test("filters regional CUT payload data and includes supplemental Antártica", () => {
  const magallanes = getComunasByRegion("12");
  assert.ok(
    magallanes.some(
      ({ code, name }) => code === "12202" && name === "Antártica",
    ),
  );
  for (const [name, code] of [
    ["Cartagena", "05603"],
    ["San Antonio", "05601"],
    ["Algarrobo", "05602"],
  ]) {
    assert.ok(
      getComunasByRegion("05").some(
        (comuna) => comuna.name === name && comuna.code === code,
      ),
    );
  }
});
