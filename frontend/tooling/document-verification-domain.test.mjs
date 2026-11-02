import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "document-verification-test-"),
);
const source = await readFile(
  path.join(
    root,
    "src/features/document-verification/domain/document-verification.ts",
  ),
  "utf8",
);
const output = ts.transpileModule(source, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;
const compiled = path.join(scratch, "document-verification.mjs");
await writeFile(compiled, output);
test.after(async () => rm(scratch, { recursive: true, force: true }));

test("strong revision ETags accept only canonical quoted decimal revisions", async () => {
  const { isStrongETag } = await import(pathToFileURL(compiled).href);
  assert.equal(isStrongETag('"0"'), true);
  assert.equal(isStrongETag('"27"'), true);
  assert.equal(isStrongETag('W/"27"'), false);
  assert.equal(isStrongETag('"027"'), false);
  assert.equal(isStrongETag("27"), false);
  assert.equal(isStrongETag(null), false);
});

test("create payload normalization accepts only exact identity or space fields", async () => {
  const { normalizeDocumentVerificationCreate } = await import(
    pathToFileURL(compiled).href
  );
  assert.deepEqual(
    normalizeDocumentVerificationCreate({
      purpose: "identity",
      document_expires_at: "2030-01-01",
    }),
    { purpose: "identity", document_expires_at: "2030-01-01" },
  );
  assert.deepEqual(
    normalizeDocumentVerificationCreate({
      purpose: "space",
      space_id: "space-1",
    }),
    { purpose: "space", space_id: "space-1" },
  );
  assert.throws(
    () =>
      normalizeDocumentVerificationCreate({
        purpose: "identity",
        space_id: "extra",
      }),
    TypeError,
  );
  assert.throws(
    () => normalizeDocumentVerificationCreate({ purpose: "space" }),
    TypeError,
  );
  assert.throws(
    () => normalizeDocumentVerificationCreate({ purpose: "other" }),
    TypeError,
  );
});
