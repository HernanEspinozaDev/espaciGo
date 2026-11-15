import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";
const root = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "identity-declaration-facade-"),
);
const domainSrc = await readFile(
  path.join(
    root,
    "src/features/identity-declaration/domain/identity-declaration.ts",
  ),
  "utf8",
);
const facadeSrc = await readFile(
  path.join(
    root,
    "src/features/identity-declaration/application/identity-declaration-facade.ts",
  ),
  "utf8",
);
const compile = (s) =>
  ts.transpileModule(s, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText;
await writeFile(path.join(scratch, "domain.mjs"), compile(domainSrc));
await writeFile(
  path.join(scratch, "facade.mjs"),
  compile(
    facadeSrc.replace(
      'from "../domain/identity-declaration"',
      'from "./domain.mjs"',
    ),
  ),
);
const { IdentityDeclarationFacade } = await import(
  pathToFileURL(path.join(scratch, "facade.mjs"))
);
test.after(() => rm(scratch, { recursive: true, force: true }));
const saved = {
  id: "550e8400-e29b-41d4-a716-446655440000",
  revision: 2,
  firstName: "Ana",
  lastName: "Pérez",
  dateOfBirth: "2000-02-29",
  rut: "12345678",
  consentVersion: "id-consent-20261108",
  consentGivenAt: "2026-11-08T12:00:00Z",
};
const input = {
  firstName: "Ana",
  lastName: "Pérez",
  dateOfBirth: "2000-02-29",
  rut: "12345678",
};
test("saves once and returns only independent authoritative readback", async () => {
  const calls = [];
  const port = {
    save: async (token, request) => {
      calls.push({ method: "save", token, request });
      return { ...saved, revision: 1 };
    },
    get: async (token) => {
      calls.push({ method: "get", token });
      return saved;
    },
  };
  const facade = new IdentityDeclarationFacade(port);
  assert.deepEqual(
    await facade.saveAndReadback("synthetic-token", input, true),
    saved,
  );
  assert.deepEqual(calls, [
    {
      method: "save",
      token: "synthetic-token",
      request: {
        first_name: "Ana",
        last_name: "Pérez",
        date_of_birth: "2000-02-29",
        rut: "12345678",
        id_country: "CHL",
        document_type: "ID",
        consent_given: true,
        consent_version: "id-consent-20261108",
      },
    },
    { method: "get", token: "synthetic-token" },
  ]);
});
test("requires explicit consent and fails closed when readback is absent", async () => {
  let saves = 0;
  const facade = new IdentityDeclarationFacade({
    save: async () => {
      saves++;
      return saved;
    },
    get: async () => null,
  });
  await assert.rejects(facade.saveAndReadback("synthetic-token", input, false));
  assert.equal(saves, 0);
  await assert.rejects(
    facade.saveAndReadback("synthetic-token", input, true),
    /could not be confirmed/,
  );
  assert.equal(saves, 1);
});
