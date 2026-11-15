import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { after, afterEach, describe, it } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";
import ts from "typescript";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-verification-api-"),
);
const originalFetch = globalThis.fetch;

async function loadModule(relPath, outName) {
  const source = (await readFile(path.join(root, relPath), "utf8")).replace(
    'import "client-only";',
    "",
  );
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  });
  const file = path.join(scratch, outName);
  await writeFile(file, outputText);
  return import(pathToFileURL(file).href);
}

const verifApi = await loadModule(
  "src/features/verifications/verification-api.ts",
  "verification-api.mjs",
);
const rutUtils = await loadModule(
  "src/features/verifications/rut-utils.ts",
  "rut-utils.mjs",
);

after(async () => rm(scratch, { recursive: true, force: true }));
afterEach(() => {
  globalThis.fetch = originalFetch;
});

const UUID = "b3b6b741-52a9-4e10-8d33-7f96ee3d5142";
const jsonResponse = (body, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });

describe("identity declaration API & RUT contract", () => {
  it("normalizes and validates Chilean RUT with module 11", () => {
    assert.equal(rutUtils.normalizeRut("12.345.670-k"), "12345670-K");
    assert.equal(rutUtils.formatRut("12345670-k"), "12.345.670-K");
    assert.equal(rutUtils.isValidRut("12.345.670-K"), true);
    assert.equal(rutUtils.isValidRut("12.345.678-9"), false);
  });

  it("exposes no provider intent or status client", () => {
    for (const retired of [
      "createDiditIntent",
      "getDiditIntent",
      "getLatestDiditStatus",
      "inspectDiditStatus",
    ]) {
      assert.equal(retired in verifApi, false, `${retired} is retired`);
    }
  });

  it("keeps declaration GET bodyless and saves the exact JSON payload", async () => {
    const calls = [];
    const responseBody = {
      id: UUID,
      revision: 1,
      first_name: "Ana",
      last_name: "Pérez",
      date_of_birth: "1990-01-01",
      rut: "12345670-K",
      id_country: "CHL",
      document_type: "ID",
      consent_version: "id-consent-20261108",
      consent_given_at: "2026-11-08T10:00:00Z",
    };
    globalThis.fetch = async (input, init) => {
      calls.push({ input: String(input), init });
      return calls.length === 1
        ? new Response(null, { status: 404 })
        : jsonResponse(responseBody, 201);
    };

    assert.equal(await verifApi.getIdentityDeclaration(), null);
    const payload = {
      first_name: "Ana",
      last_name: "Pérez",
      date_of_birth: "1990-01-01",
      rut: "12345670-K",
      id_country: "CHL",
      document_type: "ID",
      consent_given: true,
      consent_version: "id-consent-20261108",
    };
    assert.deepEqual(
      await verifApi.saveIdentityDeclaration(payload),
      responseBody,
    );

    assert.equal(
      calls[0].input,
      "/api/backend/verifications/identity-declaration",
    );
    assert.equal(calls[0].init.method, undefined);
    assert.equal(calls[0].init.body, undefined);
    assert.equal(calls[0].init.credentials, "same-origin");
    assert.equal(calls[0].init.cache, "no-store");
    assert.equal(
      calls[1].input,
      "/api/backend/verifications/identity-declaration",
    );
    assert.equal(calls[1].init.method, "POST");
    assert.equal(calls[1].init.headers["content-type"], "application/json");
    assert.equal(calls[1].init.body, JSON.stringify(payload));
  });

  it("uses neutral copy for RUT conflicts without exposing private details", async () => {
    globalThis.fetch = async () =>
      jsonResponse(
        {
          code: "rut_already_claimed",
          message: "RUT: 12345678-5 pertenece a Camila Pérez",
        },
        409,
      );
    await assert.rejects(
      () =>
        verifApi.saveIdentityDeclaration({
          first_name: "Ana",
          last_name: "Pérez",
          date_of_birth: "1990-01-01",
          rut: "12345670-K",
          id_country: "CHL",
          document_type: "ID",
          consent_given: true,
          consent_version: "id-consent-20261108",
        }),
      (error) => {
        assert.equal(
          error.message,
          "No se pudo registrar este RUT. Revisa los datos ingresados o solicita ayuda.",
        );
        assert.doesNotMatch(error.message, /12345678-5|Camila|Pérez/);
        return true;
      },
    );
  });
});
