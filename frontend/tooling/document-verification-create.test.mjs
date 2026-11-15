import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import ts from "typescript";

const root = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(
  path.join(
    process.env.TMPDIR ?? os.tmpdir(),
    "espacigo-document-verification-",
  ),
);
const source = await readFile(
  path.join(
    root,
    "src/features/document-verification/domain/document-verification.ts",
  ),
  "utf8",
);
const { outputText } = ts.transpileModule(source, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
});
const transpiledPath = path.join(scratch, "document-verification.mjs");
await writeFile(transpiledPath, outputText, "utf8");
const domain = await import(pathToFileURL(transpiledPath).href);

test.after(async () => rm(scratch, { recursive: true, force: true }));

test("identity case create requires the explicit local notice acknowledgement", () => {
  const payload = {
    purpose: "identity",
    document_format_declared: "unsure",
    capture_consent_given: true,
    capture_consent_version: "identity-capture-local-v1",
  };

  assert.deepEqual(
    domain.normalizeDocumentVerificationCreate(payload),
    payload,
  );
  for (const invalid of [
    { ...payload, capture_consent_given: false },
    { ...payload, capture_consent_version: "identity-capture-v0" },
    { ...payload, document_format_declared: "unknown" },
    { ...payload, document_expires_at: "2030-01-01T00:00:00Z" },
    { ...payload, declaration_revision_id: "arbitrary" },
  ]) {
    assert.throws(
      () => domain.normalizeDocumentVerificationCreate(invalid),
      TypeError,
    );
  }
});

test("space case create retains its separate payload contract", () => {
  assert.deepEqual(
    domain.normalizeDocumentVerificationCreate({
      purpose: "space",
      space_id: "88888888-8888-4888-8888-888888888888",
    }),
    { purpose: "space", space_id: "88888888-8888-4888-8888-888888888888" },
  );
});
