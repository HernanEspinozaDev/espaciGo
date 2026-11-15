import assert from "node:assert/strict";
import { mkdtemp, readFile, mkdir, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import os from "node:os";
import test from "node:test";
import ts from "typescript";
import { createRequire } from "node:module";

const frontend = path.resolve(import.meta.dirname, "..");
const temp = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "review-queue-facade-"),
);
const domainDir = path.join(temp, "domain");
const applicationDir = path.join(temp, "application");
await mkdir(domainDir, { recursive: true });
await mkdir(applicationDir, { recursive: true });

async function transpile(sourcePath, outputPath) {
  const source = await readFile(sourcePath, "utf8");
  const result = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.CommonJS,
    },
  });
  await writeFile(outputPath, result.outputText);
}

await transpile(
  path.join(frontend, "src/features/reviewer-queue/domain/reviewer-case.ts"),
  path.join(domainDir, "reviewer-case.js"),
);
await transpile(
  path.join(
    frontend,
    "src/features/reviewer-queue/application/reviewer-queue-facade.ts",
  ),
  path.join(applicationDir, "reviewer-queue-facade.js"),
);
const require = createRequire(import.meta.url);
const { ReviewerQueueFacade } = require(
  path.join(applicationDir, "reviewer-queue-facade.js"),
);
test.after(() => rm(temp, { recursive: true, force: true }));

function tomorrowInSantiago() {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Santiago",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(new Date());
  const part = (type) => parts.find((item) => item.type === type)?.value;
  const year = part("year");
  const month = part("month");
  const day = part("day");
  if (!year || !month || !day) throw new Error("Santiago date unavailable");
  return new Date(Date.UTC(Number(year), Number(month) - 1, Number(day) + 1))
    .toISOString()
    .slice(0, 10);
}

test("facade uses the authenticated signal and returns only assigned identity summaries", async () => {
  const controller = new AbortController();
  const calls = [];
  const auth = {
    getSnapshot: () => ({ status: "authenticated", restrictedMode: false }),
    runAuthenticatedOperation: (operation) =>
      operation("synthetic-access-token", controller.signal),
  };
  const port = {
    async listAssigned(token, signal) {
      calls.push({ token, signal });
      return {
        items: [
          {
            id: "770e8400-e29b-41d4-a716-446655440020",
            purpose: "identity",
            state: "en_revision",
            revision: 7,
            document_expires_at_observed: "2030-02-03",
            identity_declaration: { first_name: "Synthetic PII", rut: "1-9" },
          },
          {
            id: "space-case",
            purpose: "space",
            state: "en_revision",
            revision: 4,
          },
        ],
      };
    },
  };
  const facade = new ReviewerQueueFacade(port, auth);

  const rows = await facade.listAssigned();

  assert.deepEqual(calls, [
    { token: "synthetic-access-token", signal: controller.signal },
  ]);
  assert.deepEqual(rows, [
    {
      id: "770e8400-e29b-41d4-a716-446655440020",
      state: "en_revision",
      stateLabel: "En revisión",
      revision: 7,
      documentExpiresAtObserved: "2030-02-03",
    },
  ]);
  assert.equal(JSON.stringify(rows).includes("Synthetic PII"), false);
});

test("facade does not request assignments for anonymous or restricted sessions", async () => {
  let calls = 0;
  const port = {
    async listAssigned() {
      calls += 1;
      return { items: [] };
    },
    async downloadAssignedFile() {
      calls += 1;
      return new Blob(["synthetic"]);
    },
  };

  for (const snapshot of [
    { status: "anonymous", restrictedMode: false },
    { status: "authenticated", restrictedMode: true },
  ]) {
    const auth = {
      getSnapshot: () => snapshot,
      runAuthenticatedOperation: async (operation) =>
        operation("synthetic-access-token", new AbortController().signal),
    };
    const facade = new ReviewerQueueFacade(port, auth);
    await assert.rejects(facade.listAssigned(), /unavailable/i);
    await assert.rejects(
      facade.downloadAssignedIdentityFile(
        "770e8400-e29b-41d4-a716-446655440000",
        {
          id: "00000000-0000-4000-8000-000000000001",
          evidenceRole: "identity_front",
          state: "limpio",
          size: 9,
          mime: "image/png",
          pages: 1,
        },
      ),
      /unavailable/i,
    );
  }

  assert.equal(calls, 0);
});

test("facade loads only the requested identity detail through the authenticated operation", async () => {
  const caseId = "770e8400-e29b-41d4-a716-446655440000";
  const controller = new AbortController();
  const calls = [];
  const auth = {
    getSnapshot: () => ({ status: "authenticated", restrictedMode: false }),
    runAuthenticatedOperation: (operation) =>
      operation("synthetic-access-token", controller.signal),
  };
  const port = {
    async getAssigned(token, requestedCaseId, signal) {
      calls.push({ token, caseId: requestedCaseId, signal });
      return {
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
        files: [
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
        ],
        storage_key: "synthetic-private-key",
      };
    },
  };
  const facade = new ReviewerQueueFacade(port, auth);

  const detail = await facade.getAssignedIdentityDetail(caseId);

  assert.deepEqual(calls, [
    { token: "synthetic-access-token", caseId, signal: controller.signal },
  ]);
  assert.equal(detail.id, caseId);
  assert.equal(detail.declaration.rut, "12.345.678-5");
  assert.equal(JSON.stringify(detail).includes("synthetic-private-key"), false);
});

test("facade downloads only the requested assigned evidence with the session signal", async () => {
  const caseId = "770e8400-e29b-41d4-a716-446655440000";
  const fileId = "00000000-0000-4000-8000-000000000001";
  const controller = new AbortController();
  const blob = new Blob(["synthetic-image"], { type: "image/jpeg" });
  const file = {
    id: fileId,
    evidenceRole: "identity_front",
    state: "limpio",
    size: blob.size,
    mime: blob.type,
    pages: 1,
  };
  const calls = [];
  const auth = {
    getSnapshot: () => ({ status: "authenticated", restrictedMode: false }),
    runAuthenticatedOperation: (operation) =>
      operation("synthetic-access-token", controller.signal),
  };
  const port = {
    async downloadAssignedFile(
      token,
      requestedCaseId,
      requestedFileId,
      signal,
    ) {
      calls.push({
        token,
        caseId: requestedCaseId,
        fileId: requestedFileId,
        signal,
      });
      return blob;
    },
  };
  const facade = new ReviewerQueueFacade(port, auth);

  const result = await facade.downloadAssignedIdentityFile(caseId, file);

  assert.deepEqual(calls, [
    {
      token: "synthetic-access-token",
      caseId,
      fileId,
      signal: controller.signal,
    },
  ]);
  assert.equal(result, blob);
});

test("facade propagates cancellation from an unmounted evidence preview", async () => {
  const caseId = "770e8400-e29b-41d4-a716-446655440000";
  const file = {
    id: "00000000-0000-4000-8000-000000000001",
    evidenceRole: "identity_front",
    state: "limpio",
    size: 8,
    mime: "image/png",
    pages: 1,
  };
  const requestController = new AbortController();
  let observedSignal;
  const auth = {
    getSnapshot: () => ({ status: "authenticated", restrictedMode: false }),
    runAuthenticatedOperation: (operation) =>
      operation("synthetic-access-token", new AbortController().signal),
  };
  const port = {
    downloadAssignedFile(_token, _caseId, _fileId, signal) {
      observedSignal = signal;
      return new Promise((_resolve, reject) => {
        signal.addEventListener(
          "abort",
          () => reject(new DOMException("Aborted", "AbortError")),
          { once: true },
        );
      });
    },
  };
  const facade = new ReviewerQueueFacade(port, auth);
  const pending = facade.downloadAssignedIdentityFile(
    caseId,
    file,
    requestController.signal,
  );

  requestController.abort();

  assert.equal(observedSignal.aborted, true);
  await assert.rejects(pending, (error) => error.name === "AbortError");
});

test("facade rejects evidence whose bytes disagree with the assigned metadata", async () => {
  const caseId = "770e8400-e29b-41d4-a716-446655440000";
  const fileId = "00000000-0000-4000-8000-000000000001";
  const blob = new Blob(["synthetic-png"], { type: "image/png" });
  let calls = 0;
  const auth = {
    getSnapshot: () => ({ status: "authenticated", restrictedMode: false }),
    runAuthenticatedOperation: (operation) =>
      operation("synthetic-access-token", new AbortController().signal),
  };
  const port = {
    async downloadAssignedFile() {
      calls += 1;
      return blob;
    },
  };
  const facade = new ReviewerQueueFacade(port, auth);
  const file = {
    id: fileId,
    evidenceRole: "identity_front",
    state: "limpio",
    size: blob.size,
    mime: "image/png",
    pages: 1,
  };

  await assert.rejects(
    facade.downloadAssignedIdentityFile(caseId, { ...file, size: 1 }),
    /invalid reviewer evidence response/i,
  );
  await assert.rejects(
    facade.downloadAssignedIdentityFile(caseId, {
      ...file,
      mime: "image/jpeg",
    }),
    /invalid reviewer evidence response/i,
  );
  assert.equal(calls, 2);
});

test("facade blocks malformed detail IDs before making an API call", async () => {
  let calls = 0;
  const auth = {
    getSnapshot: () => ({ status: "authenticated", restrictedMode: false }),
    runAuthenticatedOperation: async (operation) =>
      operation("synthetic-access-token", new AbortController().signal),
  };
  const port = {
    async getAssigned() {
      calls += 1;
      return {};
    },
    async downloadAssignedFile() {
      calls += 1;
      return new Blob(["synthetic"]);
    },
  };
  const facade = new ReviewerQueueFacade(port, auth);

  await assert.rejects(facade.getAssignedIdentityDetail("not-a-uuid"));
  await assert.rejects(
    facade.downloadAssignedIdentityFile(
      "770e8400-e29b-41d4-a716-446655440000",
      {
        id: "not-a-uuid",
        evidenceRole: "identity_front",
        state: "limpio",
        size: 1,
        mime: "image/png",
        pages: 1,
      },
    ),
  );
  assert.equal(calls, 0);
});

test("facade submits identity decisions with authenticated scope and expected revision", async () => {
  const caseId = "770e8400-e29b-41d4-a716-446655440000";
  const documentExpiresAtObserved = tomorrowInSantiago();
  const signal = new AbortController().signal;
  const calls = [];
  const checklist = {
    names_match: true,
    rut_matches: true,
    birth_date_and_majority_match: true,
    portrait_matches_selfie: true,
    evidence_legible: true,
    document_unexpired: true,
    issue_date_and_format_verified: true,
  };
  const auth = {
    getSnapshot: () => ({ status: "authenticated", restrictedMode: false }),
    runAuthenticatedOperation: (operation) =>
      operation("synthetic-access-token", signal),
  };
  const port = {
    async reviewAssigned(token, requestedCaseId, input, requestSignal) {
      calls.push({
        token,
        caseId: requestedCaseId,
        input,
        signal: requestSignal,
      });
      return {
        id: caseId,
        purpose: "identity",
        state: "aprobada",
        revision: 8,
        files: [],
      };
    },
  };
  const facade = new ReviewerQueueFacade(port, auth);

  const result = await facade.reviewAssignedIdentityCase(caseId, 7, {
    decision: "approve",
    reasonCode: "",
    documentFormatObserved: "new",
    documentExpiresAtObserved,
    checklist,
  });

  assert.deepEqual(calls, [
    {
      token: "synthetic-access-token",
      caseId,
      input: {
        capability: "review_identity",
        decision: "approve",
        expected_revision: 7,
        document_format_observed: "new",
        document_expires_at_observed: documentExpiresAtObserved,
        identity_review_checklist: checklist,
      },
      signal,
    },
  ]);
  assert.deepEqual(result, {
    id: caseId,
    state: "aprobada",
    revision: 8,
  });
});
