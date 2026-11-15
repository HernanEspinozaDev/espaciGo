import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import path from "node:path";
import ts from "typescript";
import { pathToFileURL } from "node:url";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import os from "node:os";
const src = await readFile(
  path.resolve(
    import.meta.dirname,
    "../src/features/reviewer-queue/domain/reviewer-case.ts",
  ),
  "utf8",
);
const dir = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "review-queue-"),
);
await writeFile(
  path.join(dir, "subject.mjs"),
  ts.transpileModule(src, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText,
);
const {
  reviewerIdentityEvidenceResourceKey,
  toReviewerSummary,
  toReviewerIdentityDetail,
} = await import(pathToFileURL(path.join(dir, "subject.mjs")));
test.after(() => rm(dir, { recursive: true, force: true }));
test("reads reviewer cases from the OpenAPI items envelope", () => {
  const output = toReviewerSummary({
    items: [
      {
        id: "550e8400-e29b-41d4-a716-446655440010",
        purpose: "identity",
        state: "en_revision",
        revision: 3,
      },
    ],
  });
  assert.deepEqual(output, [
    {
      id: "550e8400-e29b-41d4-a716-446655440010",
      state: "en_revision",
      stateLabel: "En revisión",
      revision: 3,
    },
  ]);
});
test("keeps only minimal identity summary and discards all sensitive and space data", () => {
  const output = toReviewerSummary({
    items: [
      {
        id: "550e8400-e29b-41d4-a716-446655440010",
        purpose: "identity",
        state: "en_revision",
        revision: 3,
        document_expires_at_observed: "2030-02-03",
        identity_declaration: { first_name: "Synthetic", rut: "1-9" },
        storage_key: "secret",
      },
      {
        id: "space-id",
        purpose: "space",
        state: "en_revision",
        revision: 1,
        space_id: "space",
      },
    ],
  });
  assert.deepEqual(output, [
    {
      id: "550e8400-e29b-41d4-a716-446655440010",
      state: "en_revision",
      stateLabel: "En revisión",
      revision: 3,
      documentExpiresAtObserved: "2030-02-03",
    },
  ]);
  assert.equal(JSON.stringify(output).includes("Synthetic"), false);
});
test("rejects malformed envelopes instead of presenting them as an empty queue", () => {
  assert.throws(() => toReviewerSummary([]), TypeError);
  assert.throws(() => toReviewerSummary({}), TypeError);
});
test("drops reviewer summaries with non-canonical case IDs", () => {
  const caseId = "550e8400-e29b-41d4-a716-446655440000";
  const output = toReviewerSummary({
    items: [
      {
        id: "not-a-uuid",
        purpose: "identity",
        state: "en_revision",
        revision: 1,
      },
      {
        id: caseId,
        purpose: "identity",
        state: "en_revision",
        revision: 2,
      },
    ],
  });
  assert.deepEqual(output, [
    {
      id: caseId,
      state: "en_revision",
      stateLabel: "En revisión",
      revision: 2,
    },
  ]);
});
test("keeps only active reviewer states and valid civil expiry dates", () => {
  const output = toReviewerSummary({
    items: [
      {
        id: "550e8400-e29b-41d4-a716-446655440012",
        purpose: "identity",
        state: "en_revision",
        revision: 2,
        document_expires_at_observed: "2030-02-03",
      },
      {
        id: "550e8400-e29b-41d4-a716-446655440013",
        purpose: "identity",
        state: "aprobada",
        revision: 3,
      },
      {
        id: "550e8400-e29b-41d4-a716-446655440014",
        purpose: "identity",
        state: "enviada",
        revision: 4,
        document_expires_at_observed: "2030-02-30",
      },
    ],
  });
  assert.deepEqual(output, [
    {
      id: "550e8400-e29b-41d4-a716-446655440012",
      state: "en_revision",
      stateLabel: "En revisión",
      revision: 2,
      documentExpiresAtObserved: "2030-02-03",
    },
    {
      id: "550e8400-e29b-41d4-a716-446655440014",
      state: "enviada",
      stateLabel: "Enviada",
      revision: 4,
    },
  ]);
});

test("does not accept inherited object property names as case states", () => {
  const output = toReviewerSummary({
    items: [
      {
        id: "550e8400-e29b-41d4-a716-446655440015",
        purpose: "identity",
        state: "toString",
        revision: 1,
      },
      {
        id: "550e8400-e29b-41d4-a716-446655440016",
        purpose: "identity",
        state: "constructor",
        revision: 2,
      },
      {
        id: "550e8400-e29b-41d4-a716-446655440017",
        purpose: "identity",
        state: "en_revision",
        revision: 3,
      },
    ],
  });

  assert.deepEqual(output, [
    {
      id: "550e8400-e29b-41d4-a716-446655440017",
      state: "en_revision",
      stateLabel: "En revisión",
      revision: 3,
    },
  ]);
});

test("identity detail maps only the scoped declaration and file allowlist", () => {
  const caseId = "770e8400-e29b-41d4-a716-446655440000";
  const fileId = "00000000-0000-4000-8000-000000000001";
  const fileBackId = "00000000-0000-4000-8000-000000000002";
  const fileSelfieId = "00000000-0000-4000-8000-000000000003";
  const detail = toReviewerIdentityDetail(
    {
      id: caseId,
      purpose: "identity",
      state: "en_revision",
      revision: 7,
      document_format_declared: "new",
      document_format_observed: "undetermined",
      document_expires_at_observed: "2030-02-03",
      identity_declaration: {
        first_name: "Synthetic",
        last_name: "Person",
        rut: "12.345.678-5",
        date_of_birth: "1990-01-01",
      },
      files: [
        {
          id: fileId,
          evidence_role: "identity_front",
          state: "limpio",
          size: 1024,
          mime: "image/jpeg",
          pages: 1,
          storage_key: "synthetic-private-storage-key",
        },
        {
          id: fileBackId,
          evidence_role: "identity_back",
          state: "limpio",
          size: 1024,
          mime: "image/jpeg",
          pages: 1,
        },
        {
          id: fileSelfieId,
          evidence_role: "identity_selfie",
          state: "limpio",
          size: 1024,
          mime: "image/png",
          pages: 1,
        },
      ],
      reviewer_note: "synthetic-unexpected-field",
    },
    caseId,
  );

  assert.deepEqual(detail, {
    id: caseId,
    state: "en_revision",
    stateLabel: "En revisión",
    revision: 7,
    documentFormatDeclaredLabel: "Nuevo",
    documentFormatObservedLabel: "Indeterminado",
    documentExpiresAtObserved: "2030-02-03",
    declaration: {
      firstName: "Synthetic",
      lastName: "Person",
      rut: "12.345.678-5",
      dateOfBirth: "1990-01-01",
    },
    files: [
      {
        id: fileId,
        evidenceRole: "identity_front",
        state: "limpio",
        size: 1024,
        mime: "image/jpeg",
        pages: 1,
      },
      {
        id: fileBackId,
        evidenceRole: "identity_back",
        state: "limpio",
        size: 1024,
        mime: "image/jpeg",
        pages: 1,
      },
      {
        id: fileSelfieId,
        evidenceRole: "identity_selfie",
        state: "limpio",
        size: 1024,
        mime: "image/png",
        pages: 1,
      },
    ],
  });
  assert.equal(JSON.stringify(detail).includes("storage_key"), false);
  assert.equal(JSON.stringify(detail).includes("reviewer_note"), false);
});

test("identity detail requires the three unique evidence roles", () => {
  const caseId = "770e8400-e29b-41d4-a716-446655440000";
  const files = [
    {
      id: "00000000-0000-4000-8000-000000000001",
      evidence_role: "identity_front",
      state: "limpio",
      size: 100,
      mime: "image/jpeg",
      pages: 1,
    },
    {
      id: "00000000-0000-4000-8000-000000000002",
      evidence_role: "identity_back",
      state: "limpio",
      size: 100,
      mime: "image/jpeg",
      pages: 1,
    },
    {
      id: "00000000-0000-4000-8000-000000000003",
      evidence_role: "identity_selfie",
      state: "limpio",
      size: 100,
      mime: "image/jpeg",
      pages: 1,
    },
  ];
  const base = {
    id: caseId,
    purpose: "identity",
    state: "en_revision",
    revision: 3,
    identity_declaration: {
      first_name: "Synthetic",
      last_name: "Person",
      rut: "12.345.678-5",
      date_of_birth: "1990-01-01",
    },
    files,
  };

  assert.doesNotThrow(() => toReviewerIdentityDetail(base, caseId));
  assert.throws(
    () =>
      toReviewerIdentityDetail(
        {
          ...base,
          files: [
            { ...files[0], evidence_role: undefined },
            files[1],
            files[2],
          ],
        },
        caseId,
      ),
    TypeError,
  );
  assert.throws(
    () =>
      toReviewerIdentityDetail(
        {
          ...base,
          files: [
            files[0],
            { ...files[1], evidence_role: "identity_front" },
            files[2],
          ],
        },
        caseId,
      ),
    TypeError,
  );
  assert.throws(
    () => toReviewerIdentityDetail({ ...base, files: files.slice(1) }, caseId),
    TypeError,
  );
});

test("identity evidence resource keys change when metadata or scope changes", () => {
  const caseId = "770e8400-e29b-41d4-a716-446655440000";
  const file = {
    id: "00000000-0000-4000-8000-000000000001",
    evidenceRole: "identity_front",
    state: "limpio",
    size: 100,
    mime: "image/jpeg",
    pages: 1,
  };
  const baseKey = reviewerIdentityEvidenceResourceKey(caseId, file);

  assert.notEqual(
    reviewerIdentityEvidenceResourceKey(`${caseId}-different`, file),
    baseKey,
  );
  for (const changedFile of [
    { ...file, id: "00000000-0000-4000-8000-000000000002" },
    { ...file, evidenceRole: "identity_back" },
    { ...file, state: "bloqueado" },
    { ...file, size: 101 },
    { ...file, mime: "image/png" },
    { ...file, pages: 2 },
  ]) {
    assert.notEqual(
      reviewerIdentityEvidenceResourceKey(caseId, changedFile),
      baseKey,
    );
  }
});

test("identity detail rejects wrong scope, mismatched IDs, invalid dates and malformed files", () => {
  const caseId = "770e8400-e29b-41d4-a716-446655440000";
  const files = [
    {
      id: "00000000-0000-4000-8000-000000000001",
      evidence_role: "identity_front",
      state: "limpio",
      size: 1024,
      mime: "image/jpeg",
      pages: 1,
    },
    {
      id: "00000000-0000-4000-8000-000000000002",
      evidence_role: "identity_back",
      state: "limpio",
      size: 1024,
      mime: "image/jpeg",
      pages: 1,
    },
    {
      id: "00000000-0000-4000-8000-000000000003",
      evidence_role: "identity_selfie",
      state: "limpio",
      size: 1024,
      mime: "image/png",
      pages: 1,
    },
  ];
  const base = {
    id: caseId,
    purpose: "identity",
    state: "en_revision",
    revision: 7,
    identity_declaration: {
      first_name: "Synthetic",
      last_name: "Person",
      rut: "12.345.678-5",
      date_of_birth: "1990-01-01",
    },
    files,
  };
  assert.throws(
    () => toReviewerIdentityDetail({ ...base, purpose: "space" }, caseId),
    TypeError,
  );
  assert.throws(
    () =>
      toReviewerIdentityDetail(
        { ...base, id: "660e8400-e29b-41d4-a716-446655440000" },
        caseId,
      ),
    TypeError,
  );
  assert.throws(
    () =>
      toReviewerIdentityDetail(
        { ...base, document_expires_at_observed: "2030-02-30" },
        caseId,
      ),
    TypeError,
  );
  assert.throws(
    () =>
      toReviewerIdentityDetail(
        { ...base, files: [{ id: "not-a-uuid", state: "limpio" }] },
        caseId,
      ),
    TypeError,
  );
  assert.throws(
    () =>
      toReviewerIdentityDetail(
        {
          ...base,
          files: [
            {
              id: "00000000-0000-4000-8000-000000000001",
              evidence_role: "identity_front",
              state: "limpio",
              size: 1024,
              mime: "application/pdf",
              pages: 1,
            },
          ],
        },
        caseId,
      ),
    TypeError,
  );
});
