import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const repoRoot = path.resolve(import.meta.dirname, "..");
const securityCardPath = path.join(
  repoRoot,
  "src/features/account/ui/security-card.tsx",
);

const scratch = await mkdtemp(
  path.join(os.tmpdir(), "security-session-key-test-"),
);
const transpiledPath = path.join(scratch, "session-fingerprint.mjs");

const rawCode = await readFile(securityCardPath, "utf8");
const sourceFile = ts.createSourceFile(
  "security-card.tsx",
  rawCode,
  ts.ScriptTarget.Latest,
  true,
  ts.ScriptKind.TSX,
);

let sessionFingerprintNode = null;
for (const statement of sourceFile.statements) {
  if (
    ts.isFunctionDeclaration(statement) &&
    statement.name?.text === "sessionFingerprint"
  ) {
    sessionFingerprintNode = statement;
    break;
  }
}

assert.ok(
  sessionFingerprintNode,
  "sessionFingerprint function declaration not found in security-card.tsx AST",
);

const functionText = sessionFingerprintNode.getText(sourceFile);
const exportCode = `export ${functionText}`;

const transpiled = ts.transpileModule(exportCode, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;

await writeFile(transpiledPath, transpiled, "utf8");

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

const { sessionFingerprint } = await import(pathToFileURL(transpiledPath).href);

test("sessionFingerprint genera la misma huella para mismo DTO con referencias nuevas", () => {
  const dtoA = {
    status: "authenticated",
    accountId: "acc-42",
    roles: ["host", "user"],
    capabilities: ["account:write", "booking:read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  };
  const dtoB = {
    status: "authenticated",
    accountId: "acc-42",
    roles: ["host", "user"],
    capabilities: ["account:write", "booking:read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  };

  assert.equal(sessionFingerprint(dtoA), sessionFingerprint(dtoB));
});

test("sessionFingerprint exige huella distinta para roles diferentes que colisionaban con join por coma", () => {
  const combinedRoles = {
    status: "authenticated",
    accountId: "acc-42",
    roles: ["a,b"],
    capabilities: [],
    restrictedMode: false,
    expiresAt: null,
  };
  const separateRoles = {
    status: "authenticated",
    accountId: "acc-42",
    roles: ["a", "b"],
    capabilities: [],
    restrictedMode: false,
    expiresAt: null,
  };

  assert.notEqual(
    sessionFingerprint(combinedRoles),
    sessionFingerprint(separateRoles),
  );
});

test("sessionFingerprint exige huella distinta para capabilities con delimitador embebido", () => {
  const combinedCaps = {
    status: "authenticated",
    accountId: "acc-42",
    roles: [],
    capabilities: ["x,y"],
    restrictedMode: false,
    expiresAt: null,
  };
  const separateCaps = {
    status: "authenticated",
    accountId: "acc-42",
    roles: [],
    capabilities: ["x", "y"],
    restrictedMode: false,
    expiresAt: null,
  };

  assert.notEqual(
    sessionFingerprint(combinedCaps),
    sessionFingerprint(separateCaps),
  );
});

test("sessionFingerprint exige huella distinta cuando campos contienen separadores delimitadores", () => {
  const snapshotA = {
    status: "authenticated",
    accountId: "acc:1",
    roles: ["role2"],
    capabilities: [],
    restrictedMode: false,
    expiresAt: null,
  };
  const snapshotB = {
    status: "authenticated",
    accountId: "acc",
    roles: ["1:role2"],
    capabilities: [],
    restrictedMode: false,
    expiresAt: null,
  };

  assert.notEqual(sessionFingerprint(snapshotA), sessionFingerprint(snapshotB));
});

test("SecurityCard utiliza copy de sesión respaldada por Go y cookie HttpOnly sin afirmaciones obsoletas", () => {
  const normalizedCode = rawCode.replace(/\s+/g, " ");
  assert.ok(
    !rawCode.includes("Sesión criptográficamente firmada"),
    "No debe contener 'Sesión criptográficamente firmada'",
  );
  assert.ok(
    !rawCode.includes(
      "Tus tokens de acceso residen en memoria protegida del cliente",
    ),
    "No debe afirmar que los tokens residen en memoria del cliente",
  );
  assert.ok(
    rawCode.includes("Sesión confirmada por Go"),
    "Debe contener 'Sesión confirmada por Go'",
  );
  assert.ok(
    normalizedCode.includes(
      "La credencial viaja mediante una cookie HttpOnly al BFF y no se expone a JavaScript.",
    ),
    "Debe contener 'La credencial viaja mediante una cookie HttpOnly al BFF y no se expone a JavaScript.'",
  );
});
