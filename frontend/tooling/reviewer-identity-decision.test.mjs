import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import os from "node:os";
import test from "node:test";
import { createRequire } from "node:module";
import ts from "typescript";

const frontend = path.resolve(import.meta.dirname, "..");
const temp = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "reviewer-identity-decision-"),
);
const source = await readFile(
  path.join(frontend, "src/features/reviewer-queue/domain/reviewer-case.ts"),
  "utf8",
);
await writeFile(
  path.join(temp, "reviewer-identity-decision.cjs"),
  ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.CommonJS,
    },
  }).outputText,
);
const require = createRequire(import.meta.url);
const {
  buildReviewerIdentityDecisionRequest,
  toReviewerDecisionResult,
} = require(path.join(temp, "reviewer-identity-decision.cjs"));
test.after(() => rm(temp, { recursive: true, force: true }));

const approvedChecklist = {
  names_match: true,
  rut_matches: true,
  birth_date_and_majority_match: true,
  portrait_matches_selfie: true,
  evidence_legible: true,
  document_unexpired: true,
  issue_date_and_format_verified: true,
};

test("builds an exact identity approval request with a Chilean civil date", () => {
  const request = buildReviewerIdentityDecisionRequest(
    {
      decision: "approve",
      reasonCode: "",
      documentFormatObserved: "new",
      documentExpiresAtObserved: "2026-11-12",
      checklist: approvedChecklist,
    },
    7,
    new Date("2026-11-12T12:00:00-03:00"),
  );

  assert.deepEqual(request, {
    capability: "review_identity",
    decision: "approve",
    expected_revision: 7,
    document_format_observed: "new",
    document_expires_at_observed: "2026-11-12",
    identity_review_checklist: approvedChecklist,
  });
});

test("builds a needs-evidence request without inventing an expiry date", () => {
  const checklist = { ...approvedChecklist, names_match: false };
  const request = buildReviewerIdentityDecisionRequest(
    {
      decision: "needs_evidence",
      reasonCode: "antecedentes_incompletos",
      documentFormatObserved: "undetermined",
      documentExpiresAtObserved: "",
      checklist,
    },
    "8",
    new Date("2026-11-12T12:00:00-03:00"),
  );

  assert.deepEqual(request, {
    capability: "review_identity",
    decision: "needs_evidence",
    expected_revision: 8,
    reason_code: "antecedentes_incompletos",
    document_format_observed: "undetermined",
    identity_review_checklist: checklist,
  });
});

test("builds an expired-document rejection from a strictly prior civil date", () => {
  const checklist = { ...approvedChecklist, document_unexpired: false };
  const request = buildReviewerIdentityDecisionRequest(
    {
      decision: "reject",
      reasonCode: "documento_vencido",
      documentFormatObserved: "new",
      documentExpiresAtObserved: "2026-11-11",
      checklist,
    },
    9,
    new Date("2026-11-12T12:00:00-03:00"),
  );

  assert.deepEqual(request, {
    capability: "review_identity",
    decision: "reject",
    expected_revision: 9,
    reason_code: "documento_vencido",
    document_format_observed: "new",
    document_expires_at_observed: "2026-11-11",
    identity_review_checklist: checklist,
  });
});

test("rejects decisions outside the reviewer API allowlist", () => {
  const request = buildReviewerIdentityDecisionRequest(
    {
      decision: "override",
      reasonCode: "antecedentes_incompletos",
      documentFormatObserved: "new",
      documentExpiresAtObserved: "",
      checklist: approvedChecklist,
    },
    10,
    new Date("2026-11-12T12:00:00-03:00"),
  );

  assert.equal(request, null);
});

test("fails closed when the checklist object is missing", () => {
  const request = buildReviewerIdentityDecisionRequest(
    {
      decision: "approve",
      reasonCode: "",
      documentFormatObserved: "new",
      documentExpiresAtObserved: "2026-11-12",
      checklist: null,
    },
    11,
    new Date("2026-11-12T12:00:00-03:00"),
  );

  assert.equal(request, null);
});

test("approval requires every checklist criterion to be explicitly positive", () => {
  for (const field of Object.keys(approvedChecklist)) {
    const request = buildReviewerIdentityDecisionRequest(
      {
        decision: "approve",
        reasonCode: "",
        documentFormatObserved: "new",
        documentExpiresAtObserved: "2026-11-12",
        checklist: { ...approvedChecklist, [field]: false },
      },
      12,
      new Date("2026-11-12T12:00:00-03:00"),
    );

    assert.equal(request, null, `${field} must prevent approval`);
  }
});

test("rejects impossible, non-canonical, and timestamp expiry values", () => {
  const invalidDates = ["2031-02-29", "2031-4-05", "2031-04-05T23:59:59.999Z"];
  for (const documentExpiresAtObserved of invalidDates) {
    const approval = buildReviewerIdentityDecisionRequest(
      {
        decision: "approve",
        reasonCode: "",
        documentFormatObserved: "new",
        documentExpiresAtObserved,
        checklist: approvedChecklist,
      },
      14,
      new Date("2030-01-01T12:00:00-03:00"),
    );
    assert.equal(
      approval,
      null,
      `approval must reject ${documentExpiresAtObserved}`,
    );

    const needsEvidence = buildReviewerIdentityDecisionRequest(
      {
        decision: "needs_evidence",
        reasonCode: "antecedentes_incompletos",
        documentFormatObserved: "undetermined",
        documentExpiresAtObserved,
        checklist: approvedChecklist,
      },
      14,
      new Date("2030-01-01T12:00:00-03:00"),
    );
    assert.equal(
      needsEvidence,
      null,
      `needs-evidence decision must reject ${documentExpiresAtObserved}`,
    );
  }
});

test("preserves a valid leap-day expiry as the exact civil date", () => {
  const request = buildReviewerIdentityDecisionRequest(
    {
      decision: "approve",
      reasonCode: "",
      documentFormatObserved: "new",
      documentExpiresAtObserved: "2032-02-29",
      checklist: approvedChecklist,
    },
    15,
    new Date("2032-02-28T12:00:00-03:00"),
  );

  assert.equal(request?.document_expires_at_observed, "2032-02-29");
});

test("uses the Santiago civil date across the autumn DST transition", () => {
  const draft = {
    decision: "approve",
    reasonCode: "",
    documentFormatObserved: "new",
    documentExpiresAtObserved: "2026-04-04",
    checklist: approvedChecklist,
  };

  assert.notEqual(
    buildReviewerIdentityDecisionRequest(
      draft,
      13,
      new Date("2026-04-05T03:59:59Z"),
    ),
    null,
  );
  assert.equal(
    buildReviewerIdentityDecisionRequest(
      draft,
      13,
      new Date("2026-04-05T04:00:00Z"),
    ),
    null,
  );
});

test("accepts only the matching terminal result for the expected review revision", () => {
  const caseId = "770e8400-e29b-41d4-a716-446655440000";
  const result = toReviewerDecisionResult(
    {
      id: caseId,
      purpose: "identity",
      state: "aprobada",
      revision: 8,
      files: [],
    },
    caseId,
    7,
    "approve",
  );

  assert.deepEqual(result, {
    id: caseId,
    state: "aprobada",
    revision: 8,
  });
});

test("rejects a response with the wrong case, purpose, decision state, or revision", () => {
  const caseId = "770e8400-e29b-41d4-a716-446655440000";
  const validResponse = {
    id: caseId,
    purpose: "identity",
    state: "aprobada",
    revision: 8,
  };
  const invalidResponses = [
    { ...validResponse, id: "880e8400-e29b-41d4-a716-446655440000" },
    { ...validResponse, purpose: "space" },
    { ...validResponse, state: "rechazada" },
    { ...validResponse, revision: 9 },
  ];

  for (const response of invalidResponses) {
    assert.throws(
      () => toReviewerDecisionResult(response, caseId, 7, "approve"),
      TypeError,
    );
  }
});
