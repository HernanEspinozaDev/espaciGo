import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { parse } from "yaml";

const root = path.resolve(import.meta.dirname, "..");
const spec = parse(
  await readFile(path.join(root, "../planning/openapi.yaml"), "utf8"),
);

test("identity reviewer queue and assigned detail contracts are scoped and use civil dates", () => {
  const operation = spec.paths["/document-verification/reviewer/cases"]?.get;
  assert.ok(operation, "reviewer queue GET operation is required");
  assert.equal(operation.operationId, "listAssignedDocumentCases");
  assert.deepEqual(operation.security, [{ bearerAuth: [] }]);
  assert.match(operation.description ?? "", /asignaciones vigentes/i);
  assert.match(
    operation.description ?? "",
    /no revela identidad ni claves de storage/i,
  );

  const detailPath = "/document-verification/reviewer/cases/{caseID}";
  const detail = spec.paths[detailPath]?.get;
  assert.ok(detail, "assigned reviewer detail GET operation is required");
  assert.equal(detail.operationId, "getAssignedDocumentCase");
  assert.deepEqual(detail.security, [{ bearerAuth: [] }]);
  assert.deepEqual(spec.paths[detailPath].parameters, [
    {
      name: "caseID",
      in: "path",
      required: true,
      schema: { type: "string", format: "uuid" },
    },
  ]);

  const downloadPath =
    "/document-verification/reviewer/cases/{caseID}/files/{fileID}/content";
  const download = spec.paths[downloadPath]?.get;
  assert.ok(download, "assigned clean evidence download GET is required");
  assert.equal(download.operationId, "downloadAssignedCleanDocument");
  assert.deepEqual(download.security, [{ bearerAuth: [] }]);
  assert.deepEqual(Object.keys(download.responses["200"].content).sort(), [
    "application/pdf",
    "image/jpeg",
    "image/png",
  ]);
  for (const media of Object.values(download.responses["200"].content)) {
    assert.deepEqual(media.schema, { type: "string", format: "binary" });
  }
  assert.match(download.description ?? "", /asignación vigente/i);

  const reviewPath = "/document-verification/reviewer/cases/{caseID}/review";
  const review = spec.paths[reviewPath]?.post;
  assert.ok(review, "assigned identity review POST operation is required");
  assert.equal(review.operationId, "reviewAssignedDocumentCase");
  assert.deepEqual(review.security, [{ bearerAuth: [] }]);
  assert.match(review.description ?? "", /ni permite autorrevisión/i);
  const reviewBody = review.requestBody.content["application/json"].schema;
  assert.equal(reviewBody.additionalProperties, false);
  assert.deepEqual(reviewBody.required, [
    "capability",
    "decision",
    "expected_revision",
  ]);
  assert.deepEqual(reviewBody.properties.capability.enum, [
    "review_identity",
    "review_space",
  ]);
  assert.deepEqual(reviewBody.properties.decision.enum, [
    "approve",
    "reject",
    "needs_evidence",
  ]);
  assert.equal(
    reviewBody.properties.identity_review_checklist.$ref,
    "#/components/schemas/IdentityReviewChecklist",
  );
  assert.equal(
    review.responses["200"].content["application/json"].schema.$ref,
    "#/components/schemas/DocumentVerificationReviewerCase",
  );

  const checklist = spec.components.schemas.IdentityReviewChecklist;
  assert.equal(checklist.additionalProperties, false);
  assert.deepEqual(checklist.required, [
    "names_match",
    "rut_matches",
    "birth_date_and_majority_match",
    "portrait_matches_selfie",
    "evidence_legible",
    "document_unexpired",
    "issue_date_and_format_verified",
  ]);
  for (const property of Object.values(checklist.properties)) {
    assert.deepEqual(property.type, ["boolean", "null"]);
  }

  const reviewerOperations = Object.entries(spec.paths)
    .filter(([path]) =>
      path.startsWith("/document-verification/reviewer/cases"),
    )
    .flatMap(([path, pathItem]) =>
      Object.entries(pathItem)
        .filter(([method]) =>
          ["get", "post", "put", "patch", "delete"].includes(method),
        )
        .map(([method]) => `${method.toUpperCase()} ${path}`),
    )
    .sort();
  assert.deepEqual(
    reviewerOperations,
    [
      `GET ${downloadPath}`,
      `GET ${detailPath}`,
      `POST ${reviewPath}`,
      "POST /document-verification/reviewer/cases/{caseID}/revoke",
      "GET /document-verification/reviewer/cases",
    ].sort(),
  );

  const sessionCapabilities = spec.components.schemas.AdminCapability.enum;
  assert.ok(!sessionCapabilities.includes("review_identity"));

  const reviewerCase = spec.components.schemas.DocumentVerificationReviewerCase;
  assert.equal(
    reviewerCase.properties.document_expires_at_observed.format,
    "date",
  );
  assert.deepEqual(
    spec.components.schemas.DocumentVerificationOwnerFile.properties
      .evidence_role.enum,
    ["identity_front", "identity_back", "identity_selfie"],
  );
});
