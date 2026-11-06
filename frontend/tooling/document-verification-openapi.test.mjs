import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { parse } from "yaml";

const root = path.resolve(import.meta.dirname, "..");
const specPath = path.join(root, "../planning/openapi.yaml");
const spec = parse(await readFile(specPath, "utf8"));
const create = spec.paths["/document-verification/cases"].post;
const submit = spec.paths["/document-verification/cases/{id}/submit"].post;

test("document verification OpenAPI matches Go errors and expiry semantics", () => {
  assert.ok(create.responses["400"], "create must document invalid input");
  assert.ok(
    submit.responses["400"],
    "submit must document invalid state/input",
  );

  const createSchema = spec.components.schemas.DocumentVerificationCaseCreate;
  const expirationSchemas = [
    createSchema.oneOf[0].properties.document_expires_at,
    createSchema.oneOf[1].properties.document_expires_at,
  ];
  for (const expiration of expirationSchemas) {
    assert.match(
      expiration.description ?? "",
      /posterior al instante de recepción/i,
      "provided expiration must be documented as strictly future",
    );
  }
});
