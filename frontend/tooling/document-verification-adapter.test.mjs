import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(
  path.join(
    process.env.TMPDIR ?? os.tmpdir(),
    "espacigo-document-verification-adapter-",
  ),
);
test.after(async () => rm(scratch, { recursive: true, force: true }));

function compile(source) {
  return ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText;
}

const domainSource = await readFile(
  path.join(
    root,
    "src/features/document-verification/domain/document-verification.ts",
  ),
  "utf8",
);
await writeFile(
  path.join(scratch, "domain-document-verification.mjs"),
  compile(domainSource),
);

for (const [sourcePath, outputName] of [
  ["src/lib/api/errors.ts", "api-errors.mjs"],
  ["src/lib/api/exact-json.ts", "exact-json.mjs"],
]) {
  const source = await readFile(path.join(root, sourcePath), "utf8");
  await writeFile(path.join(scratch, outputName), compile(source));
}

await writeFile(
  path.join(scratch, "api-client.mjs"),
  `export const apiClient = {
    calls: [],
    nextResult: undefined,
    async GET(path, options) {
      this.calls.push({ path, options });
      return this.nextResult;
    },
  };`,
);

const adapterSource = await readFile(
  path.join(
    root,
    "src/features/document-verification/infrastructure/document-verification-openapi-adapter.ts",
  ),
  "utf8",
);
const compiledAdapter = compile(
  adapterSource
    .replaceAll('import "client-only";', "")
    .replaceAll('from "@/lib/api/client";', 'from "./api-client.mjs";')
    .replaceAll('from "@/lib/api/errors";', 'from "./api-errors.mjs";')
    .replaceAll('from "@/lib/api/exact-json";', 'from "./exact-json.mjs";')
    .replaceAll(
      'from "../domain/document-verification";',
      'from "./domain-document-verification.mjs";',
    ),
);
const adapterPath = path.join(scratch, "document-verification-adapter.mjs");
await writeFile(adapterPath, compiledAdapter);

const [{ OpenApiDocumentVerificationAdapter }, { apiClient }] =
  await Promise.all([
    import(pathToFileURL(adapterPath)),
    import(pathToFileURL(path.join(scratch, "api-client.mjs"))),
  ]);

const ownerCase = {
  id: "550e8400-e29b-41d4-a716-446655440001",
  purpose: "identity",
  state: "requiere_antecedentes",
  revision: 2,
  owner_decision_message:
    "Faltan antecedentes para continuar con la revisión. Revisa los documentos solicitados y completa lo que falta antes de enviar nuevamente.",
  files: [],
};

test("list preserves the optional owner decision message from API data", async () => {
  assert.deepEqual(Object.keys(ownerCase).sort(), [
    "files",
    "id",
    "owner_decision_message",
    "purpose",
    "revision",
    "state",
  ]);
  assert.ok(Array.from(ownerCase.owner_decision_message).length <= 240);
  apiClient.calls.length = 0;
  apiClient.nextResult = {
    response: new Response("[]", { status: 200 }),
    data: [ownerCase],
  };

  const adapter = new OpenApiDocumentVerificationAdapter();
  const result = await adapter.list("synthetic-access-token");

  assert.deepEqual(result, [ownerCase]);
  assert.equal(
    result[0].owner_decision_message,
    ownerCase.owner_decision_message,
  );
  assert.equal(apiClient.calls.length, 1);
  assert.equal(apiClient.calls[0].path, "/document-verification/cases");
  assert.deepEqual(apiClient.calls[0].options.headers, {
    Authorization: "Bearer synthetic-access-token",
  });
});

test("getOwnCase preserves owner decision copy on the versioned detail read", async () => {
  apiClient.calls.length = 0;
  apiClient.nextResult = {
    response: new Response(JSON.stringify(ownerCase), {
      status: 200,
      headers: { etag: '"2"' },
    }),
    data: ownerCase,
  };

  const adapter = new OpenApiDocumentVerificationAdapter();
  const result = await adapter.getOwnCase(
    "synthetic-access-token",
    ownerCase.id,
  );

  assert.deepEqual(result, { value: ownerCase, etag: '"2"' });
  assert.equal(
    result.value.owner_decision_message,
    ownerCase.owner_decision_message,
  );
  assert.equal(apiClient.calls.length, 1);
  assert.equal(apiClient.calls[0].path, "/document-verification/cases/{id}");
  assert.deepEqual(apiClient.calls[0].options.params, {
    path: { id: ownerCase.id },
  });
  assert.deepEqual(apiClient.calls[0].options.headers, {
    Authorization: "Bearer synthetic-access-token",
  });
});
