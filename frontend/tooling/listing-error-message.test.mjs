import assert from "node:assert/strict";
import test from "node:test";
import ts from "typescript";
import { readFile } from "node:fs/promises";

const source = await readFile(
  new URL(
    "../src/features/host-listings/domain/listing-error-message.ts",
    import.meta.url,
  ),
  "utf8",
);
const errorModule = `data:text/javascript;base64,${Buffer.from("export class ApiError extends Error { constructor(input) { super('API request failed (' + input.status + ': ' + input.code + ')'); this.status = input.status; this.code = input.code; } }").toString("base64")}`;
const transpiled = ts.transpileModule(
  source.replace('"@/lib/api/errors"', JSON.stringify(errorModule)),
  {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  },
).outputText;
const { listingErrorMessage } = await import(
  `data:text/javascript;base64,${Buffer.from(transpiled).toString("base64")}`
);
const ApiError = (await import(errorModule)).ApiError;

test("listing transport errors use fixed Spanish messages without leaking technical details", () => {
  const error = (status, code) =>
    new ApiError({
      status,
      code,
      requestId: null,
      retryAfter: null,
      fields: [],
    });
  assert.equal(
    listingErrorMessage(error(412, "precondition_failed")),
    "Este espacio cambió. Actualiza la información antes de volver a guardar.",
  );
  assert.equal(
    listingErrorMessage(error(401, "unauthenticated")).includes("API request"),
    false,
  );
  assert.equal(
    listingErrorMessage(error(403, "forbidden")).includes("forbidden"),
    false,
  );
  assert.equal(
    listingErrorMessage(error(422, "validation_error")).includes(
      "validation_error",
    ),
    false,
  );
  assert.equal(
    listingErrorMessage(error(428, "reauthentication_required")).includes(
      "428",
    ),
    false,
  );
  assert.equal(
    listingErrorMessage(
      new Error("API request failed (500: secret_backend_detail)"),
    ).includes("secret_backend_detail"),
    false,
  );
  assert.equal(
    listingErrorMessage(new Error("fetch failed")).includes("fetch failed"),
    false,
  );
});
