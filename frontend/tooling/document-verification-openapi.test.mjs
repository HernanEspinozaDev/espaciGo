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
const upload = spec.paths["/document-verification/cases/{id}/files"].post;

test("document verification OpenAPI matches Go errors and identity declaration semantics", () => {
  assert.ok(create.responses["400"], "create must document invalid input");
  assert.ok(
    submit.responses["400"],
    "submit must document invalid state/input",
  );

  const createSchema = spec.components.schemas.DocumentVerificationCaseCreate;
  const identitySchema = createSchema.oneOf.find(
    (branch) => branch.properties?.purpose?.const === "identity",
  );
  const spaceSchema = createSchema.oneOf.find(
    (branch) => branch.properties?.purpose?.const === "space",
  );
  assert.ok(identitySchema, "identity create branch must exist");
  assert.ok(spaceSchema, "space create branch must exist");
  assert.deepEqual(identitySchema.required, [
    "purpose",
    "document_format_declared",
    "capture_consent_given",
    "capture_consent_version",
  ]);
  assert.deepEqual(identitySchema.properties.document_format_declared.enum, [
    "legacy",
    "new",
    "unsure",
  ]);
  assert.equal(identitySchema.properties.capture_consent_given.const, true);
  assert.equal(
    identitySchema.properties.capture_consent_version.const,
    "identity-capture-local-v1",
  );
  assert.equal(
    Object.hasOwn(identitySchema.properties, "document_expires_at"),
    false,
    "identity create must not accept a user-declared document expiry",
  );
  assert.equal(
    spaceSchema.required.includes("document_expires_at"),
    false,
    "space document expiry remains optional",
  );
  assert.match(
    spaceSchema.properties.document_expires_at.description ?? "",
    /posterior al instante de recepción/i,
    "provided space expiry must be documented as strictly future",
  );
});

test("shared file upload declares typed identity roles without requiring them for spaces", () => {
  const schema = upload.requestBody.content["multipart/form-data"].schema;
  assert.deepEqual(schema.required, ["file"]);
  assert.deepEqual(schema.properties.evidence_role.enum, [
    "identity_front",
    "identity_back",
    "identity_selfie",
  ]);
  assert.equal(schema.properties.file.format, "binary");
});
