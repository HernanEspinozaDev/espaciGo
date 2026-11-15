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

test("identity create contract and space expiry semantics are explicit", () => {
  assert.ok(create.responses["400"], "create must document invalid input");
  assert.ok(
    submit.responses["400"],
    "submit must document invalid state/input",
  );

  const createSchema = spec.components.schemas.DocumentVerificationCaseCreate;
  const identitySchema = createSchema.oneOf.find(
    (variant) => variant.properties?.purpose?.const === "identity",
  );
  const spaceSchema = createSchema.oneOf.find(
    (variant) => variant.properties?.purpose?.const === "space",
  );
  assert.ok(identitySchema, "identity create variant must be documented");
  assert.ok(spaceSchema, "space create variant must be documented");
  assert.deepEqual(
    [...identitySchema.required].sort(),
    [
      "purpose",
      "document_format_declared",
      "capture_consent_given",
      "capture_consent_version",
    ].sort(),
  );
  assert.equal(identitySchema.additionalProperties, false);
  assert.deepEqual(
    [...identitySchema.properties.document_format_declared.enum].sort(),
    ["legacy", "new", "unsure"].sort(),
  );
  assert.equal(identitySchema.properties.capture_consent_given.const, true);
  assert.equal(
    identitySchema.properties.capture_consent_version.const,
    "identity-capture-local-v1",
  );
  assert.match(
    identitySchema.properties.capture_consent_given.description,
    /not authorization for real personal data/i,
  );
  assert.equal(identitySchema.properties.document_expires_at, undefined);
  assert.equal(
    identitySchema.properties.identity_declaration_revision_id,
    undefined,
  );

  const expiration = spaceSchema.properties.document_expires_at;
  assert.match(
    expiration.description ?? "",
    /posterior al instante de recepción/i,
    "provided expiration must be documented as strictly future",
  );
});

test("owner document-verification response schemas are closed and omit reviewer details", () => {
  const ownerCase = spec.components.schemas.DocumentVerificationOwnerCase;
  const ownerFile = spec.components.schemas.DocumentVerificationOwnerFile;

  assert.equal(ownerCase.additionalProperties, false);
  assert.deepEqual(
    Object.keys(ownerCase.properties).sort(),
    [
      "document_expires_at",
      "document_format_declared",
      "files",
      "id",
      "owner_decision_message",
      "purpose",
      "revision",
      "space_id",
      "state",
      "verified_at",
      "verified_until",
    ].sort(),
  );
  assert.deepEqual(
    [...ownerCase.required].sort(),
    ["id", "purpose", "state", "revision", "files"].sort(),
  );
  assert.equal(ownerCase.properties.owner_decision_message.type, "string");
  assert.equal(ownerCase.properties.owner_decision_message.maxLength, 240);
  assert.match(
    ownerCase.properties.owner_decision_message.description,
    /sanitized identity decision copy/i,
  );
  assert.deepEqual(
    [...ownerCase.properties.state.enum].sort(),
    [
      "aprobada",
      "borrador",
      "en_revision",
      "enviada",
      "rechazada",
      "requiere_antecedentes",
      "revocada",
      "vencida",
    ].sort(),
  );
  for (const privateField of [
    "reason_code",
    "reason",
    "reviewer_reason",
    "reviewer_id",
    "identity_declaration_revision_id",
    "declaration_revision_id",
    "document_format_observed",
    "document_expires_at_observed",
  ]) {
    assert.equal(
      ownerCase.properties[privateField],
      undefined,
      `owner case schema must not expose ${privateField}`,
    );
  }

  assert.equal(ownerFile.additionalProperties, false);
  assert.deepEqual(
    Object.keys(ownerFile.properties).sort(),
    ["evidence_role", "id", "mime", "pages", "size", "state"].sort(),
  );
  for (const privateField of [
    "reviewer_observation",
    "scan_details",
    "storage_key",
  ]) {
    assert.equal(
      ownerFile.properties[privateField],
      undefined,
      `owner file schema must not expose ${privateField}`,
    );
  }
});
