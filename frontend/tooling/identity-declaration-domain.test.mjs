import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";
const root = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "identity-declaration-domain-"),
);
const source = await readFile(
  path.join(
    root,
    "src/features/identity-declaration/domain/identity-declaration.ts",
  ),
  "utf8",
);
await writeFile(
  path.join(scratch, "domain.mjs"),
  ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText,
);
const domain = await import(pathToFileURL(path.join(scratch, "domain.mjs")));
test.after(() => rm(scratch, { recursive: true, force: true }));
test("builds the exact allowlisted request preserving submitted strings", () => {
  const input = {
    firstName: " María ",
    lastName: "Ñuñez",
    dateOfBirth: "1990-02-28",
    rut: "12.345.678-5",
  };
  assert.deepEqual(domain.buildIdentityDeclarationRequest(input), {
    first_name: input.firstName,
    last_name: input.lastName,
    date_of_birth: input.dateOfBirth,
    rut: input.rut,
    id_country: "CHL",
    document_type: "ID",
    consent_given: true,
    consent_version: "id-consent-20261108",
  });
});
test("rejects blank/oversized names, malformed dates and RUT lengths", () => {
  const base = {
    firstName: "Ana",
    lastName: "Pérez",
    dateOfBirth: "2000-02-29",
    rut: "12345678",
  };
  for (const patch of [
    { firstName: "   " },
    { firstName: "x".repeat(151) },
    { dateOfBirth: "2001-02-29" },
    { rut: "1234567" },
    { rut: "1".repeat(21) },
  ])
    assert.throws(
      () => domain.buildIdentityDeclarationRequest({ ...base, ...patch }),
      TypeError,
    );
});
test("validates exact response schema and maps to camelCase", () => {
  const wire = {
    id: "550e8400-e29b-41d4-a716-446655440000",
    revision: 1,
    first_name: "Ana",
    last_name: "Pérez",
    date_of_birth: "2000-02-29",
    rut: "12345678",
    id_country: "CHL",
    document_type: "ID",
    consent_version: "id-consent-20261108",
    consent_given_at: "2026-11-08T12:00:00Z",
  };
  const declaration = domain.parseIdentityDeclarationResponse(wire);
  assert.equal(declaration.firstName, "Ana");
  assert.equal(declaration.revision, 1);
  for (const rut of ["1234567", "1".repeat(21)])
    assert.throws(
      () => domain.parseIdentityDeclarationResponse({ ...wire, rut }),
      TypeError,
    );
  for (const rut of ["12345678", "1".repeat(20)])
    assert.equal(
      domain.parseIdentityDeclarationResponse({ ...wire, rut }).rut,
      rut,
    );
  for (const invalid of [
    { ...wire, extra: "x" },
    { ...wire, revision: "1" },
    { ...wire, revision: 0 },
    { ...wire, revision: 1.5 },
    { ...wire, id_country: "USA" },
    { ...wire, consent_version: "other" },
  ])
    assert.throws(
      () => domain.parseIdentityDeclarationResponse(invalid),
      TypeError,
    );
});
