import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const repoRoot = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(path.join(os.tmpdir(), "admin-domain-test-"));

async function compile(sourcePath, outputName, replacements = []) {
  let source = await readFile(path.join(repoRoot, sourcePath), "utf8");
  for (const [before, after] of replacements) {
    source = source.replaceAll(before, after);
  }
  const output = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText;
  const destination = path.join(scratch, outputName);
  await writeFile(destination, output, "utf8");
  return destination;
}

// Mock private blob urls
await writeFile(
  path.join(scratch, "private-blob-urls.mjs"),
  "export const privateBlobUrls = { clearCount: 0, clear() { this.clearCount++; } };",
  "utf8",
);

const rulesPath = await compile(
  "src/features/admin/domain/admin-rules.ts",
  "admin-rules.mjs",
);

const facadePath = await compile(
  "src/features/admin/application/admin-facade.ts",
  "admin-facade.mjs",
  [
    ['import "client-only";', ""],
    ["@/lib/api/private-blob-urls", "./private-blob-urls.mjs"],
    ["../domain/admin-rules", "./admin-rules.mjs"],
  ],
);

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

test("admin rules: validates UUID and generates valid v4 Idempotency-Key", async () => {
  const { isValidUUID, generateIdempotencyKey } = await import(
    pathToFileURL(rulesPath).href
  );

  assert.equal(isValidUUID("123e4567-e89b-12d3-a456-426614174000"), true);
  assert.equal(isValidUUID("d3b07384-d113-4632-a6f9-03a89a05b38a"), true);
  assert.equal(isValidUUID("invalid-uuid"), false);
  assert.equal(isValidUUID(""), false);
  assert.equal(isValidUUID(null), false);
  assert.equal(isValidUUID(undefined), false);

  const key = generateIdempotencyKey();
  assert.equal(typeof key, "string");
  assert.equal(isValidUUID(key), true);
});

test("admin rules: validates mandatory reason (minimum 5 chars, trimmed, no control chars)", async () => {
  const { isValidAdminReason } = await import(pathToFileURL(rulesPath).href);

  assert.equal(isValidAdminReason("Motivo válido de moderación"), true);
  assert.equal(isValidAdminReason("12345"), true);
  assert.equal(isValidAdminReason("    12345    "), true);
  assert.equal(isValidAdminReason("1234"), false);
  assert.equal(isValidAdminReason("   1234   "), false);
  assert.equal(isValidAdminReason(""), false);
  assert.equal(isValidAdminReason(null), false);
  assert.equal(isValidAdminReason(undefined), false);
  assert.equal(isValidAdminReason(12345), false);
  assert.equal(isValidAdminReason("Motivo con\u0000caracter nulo"), false);
});

test("admin rules: validates expected_updated_at ISO-8601 concurrency format", async () => {
  const { isValidExpectedUpdatedAt } = await import(
    pathToFileURL(rulesPath).href
  );

  assert.equal(isValidExpectedUpdatedAt("2026-10-25T14:30:00Z"), true);
  assert.equal(isValidExpectedUpdatedAt("2026-10-25T14:30:00.123Z"), true);
  assert.equal(isValidExpectedUpdatedAt("2026-10-25T11:30:00-03:00"), true);
  assert.equal(isValidExpectedUpdatedAt("invalid-date"), false);
  assert.equal(isValidExpectedUpdatedAt("2026-10-25"), false);
  assert.equal(isValidExpectedUpdatedAt(""), false);
  assert.equal(isValidExpectedUpdatedAt(null), false);
});

test("admin rules: RBAC roles and administrative hierarchy", async () => {
  const { hasRequiredRole, isAdminOrGlobalAdmin } = await import(
    pathToFileURL(rulesPath).href
  );

  const rolesUser = ["arrendatario"];
  const rolesAdmin = ["arrendatario", "administrador"];
  const rolesGlobal = ["administrador_global"];

  assert.equal(hasRequiredRole(rolesUser, "administrador"), false);
  assert.equal(hasRequiredRole(rolesAdmin, "administrador"), true);
  assert.equal(hasRequiredRole(rolesGlobal, "administrador_global"), true);
  assert.equal(hasRequiredRole(null, "administrador"), false);

  assert.equal(isAdminOrGlobalAdmin(rolesUser), false);
  assert.equal(isAdminOrGlobalAdmin(rolesAdmin), true);
  assert.equal(isAdminOrGlobalAdmin(rolesGlobal), true);
  assert.equal(isAdminOrGlobalAdmin([]), false);
  assert.equal(isAdminOrGlobalAdmin(undefined), false);
});

test("admin rules: capabilities and section navigation gate", async () => {
  const { hasCapability, canAccessSection } = await import(
    pathToFileURL(rulesPath).href
  );

  const caps = ["kpi.read", "publications.list", "audit.read"];

  assert.equal(hasCapability(caps, "kpi.read"), true);
  assert.equal(hasCapability(caps, "accounts.block"), false);
  assert.equal(hasCapability(null, "kpi.read"), false);

  // KPI section requires kpi.read
  assert.equal(canAccessSection(caps, "kpi"), true);
  assert.equal(canAccessSection(["accounts.list"], "kpi"), false);

  // Accounts section requires accounts.*
  assert.equal(canAccessSection(caps, "accounts"), false);
  assert.equal(canAccessSection(["accounts.read"], "accounts"), true);

  // Publications section requires publications.*
  assert.equal(canAccessSection(caps, "publications"), true);

  // Audit section requires audit.read or audit.export
  assert.equal(canAccessSection(caps, "audit"), true);
  assert.equal(canAccessSection(["audit.export"], "audit"), true);

  // Local admin role grants operational sections
  assert.equal(canAccessSection([], "verifications", ["administrador"]), true);
  assert.equal(canAccessSection([], "disputes", ["administrador"]), true);
  assert.equal(canAccessSection([], "verifications", ["arrendatario"]), false);

  // Global admin role grants overview
  assert.equal(
    canAccessSection([], "overview", ["administrador_global"]),
    true,
  );
});

test("admin rules: conflict, 428 recent auth, and forbidden error checkers", async () => {
  const { isConcurrencyConflict, isRecentAuthRequired, isForbiddenError } =
    await import(pathToFileURL(rulesPath).href);

  assert.equal(isConcurrencyConflict({ status: 409 }), true);
  assert.equal(isConcurrencyConflict({ code: "concurrency_conflict" }), true);
  assert.equal(isConcurrencyConflict({ status: 400 }), false);

  assert.equal(isRecentAuthRequired({ status: 428 }), true);
  assert.equal(isRecentAuthRequired({ code: "precondition_required" }), true);
  assert.equal(isRecentAuthRequired({ status: 401 }), false);

  assert.equal(isForbiddenError({ status: 403 }), true);
  assert.equal(isForbiddenError({ code: "forbidden" }), true);
  assert.equal(isForbiddenError({ status: 500 }), false);
});

test("admin rules: state and code validators", async () => {
  const {
    isValidPublicationState,
    isValidAccountState,
    isValidVerificationDecision,
    isValidDamageClaimDecision,
    isValidDisputeCloseReason,
    isValidReviewReportAction,
    isValidReviewReportReasonCode,
    isValidNoticeReopenReasonCode,
  } = await import(pathToFileURL(rulesPath).href);

  assert.equal(isValidPublicationState("activa"), true);
  assert.equal(isValidPublicationState("eliminada"), true);
  assert.equal(isValidPublicationState("otra"), false);

  assert.equal(isValidAccountState("activo"), true);
  assert.equal(isValidAccountState("bloqueado"), true);
  assert.equal(isValidAccountState("invalido"), false);

  assert.equal(isValidVerificationDecision("aprobar"), true);
  assert.equal(isValidVerificationDecision("rechazar"), true);
  assert.equal(isValidVerificationDecision("eliminar"), false);

  assert.equal(isValidDamageClaimDecision("aceptado"), true);
  assert.equal(isValidDamageClaimDecision("parcial"), true);
  assert.equal(isValidDamageClaimDecision("invalido"), false);

  assert.equal(isValidDisputeCloseReason("ensayo_finalizado"), true);
  assert.equal(isValidDisputeCloseReason("otro"), false);

  assert.equal(isValidReviewReportAction("hide"), true);
  assert.equal(isValidReviewReportAction("dismiss"), true);
  assert.equal(isValidReviewReportAction("delete"), false);

  assert.equal(isValidReviewReportReasonCode("contenido_inadecuado"), true);
  assert.equal(isValidReviewReportReasonCode("desconocido"), false);

  assert.equal(isValidNoticeReopenReasonCode("smtp_restaurado"), true);
  assert.equal(isValidNoticeReopenReasonCode("desconocido"), false);
});

test("admin facade: snapshot immutability, reactivity, and pure validation gate", async () => {
  const { AdminFacade } = await import(pathToFileURL(facadePath).href);

  let calls = [];
  const mockPort = {
    async getGlobalAdminMe() {
      calls.push("getGlobalAdminMe");
      return {
        account_id: "11111111-1111-4111-8111-111111111111",
        roles: ["administrador_global"],
        capabilities: ["kpi.read", "accounts.list"],
        generated_at: new Date().toISOString(),
      };
    },
    async updateGlobalAccountProfile() {
      calls.push("updateGlobalAccountProfile");
    },
    async setGlobalAccountBlocked() {
      calls.push("setGlobalAccountBlocked");
    },
    async reviewVerification() {
      calls.push("reviewVerification");
      return {};
    },
  };

  const facade = new AdminFacade(mockPort);
  const initial = facade.getSnapshot();

  assert.equal(initial.loading, false);
  assert.equal(initial.error, null);
  assert.equal(Array.isArray(initial.globalAccounts), true);
  assert.equal(Object.isFrozen(initial.globalAccounts), true);
  assert.equal(Object.isFrozen(initial.verifications), true);

  // Subscription reactivity
  let notifications = 0;
  const unsubscribe = facade.subscribe(() => {
    notifications++;
  });

  // Load global me
  await facade.loadGlobalMe("test-token");
  assert.ok(notifications > 0);
  const afterLoad = facade.getSnapshot();
  assert.equal(
    afterLoad.me?.account_id,
    "11111111-1111-4111-8111-111111111111",
  );
  assert.equal(facade.hasCapability("kpi.read"), true);
  assert.equal(facade.hasCapability("audit.export"), false);
  assert.equal(facade.canAccess("kpi"), true);
  assert.equal(facade.canAccess("overview"), true);

  // Pure validation: short reason rejected before calling port
  calls = [];
  const blockOk = await facade.setGlobalAccountBlocked(
    "test-token",
    "account-1",
    true,
    "abc", // Less than 5 chars!
  );
  assert.equal(blockOk, false);
  assert.equal(calls.length, 0); // Port was NOT called!
  assert.ok(facade.getSnapshot().error?.includes("al menos 5 caracteres"));

  // Concurrency check: invalid expected_updated_at rejected
  const updateOk = await facade.updateGlobalAccountProfile(
    "test-token",
    "account-1",
    {
      display_name: "Nuevo Nombre",
      expected_updated_at: "not-a-date",
      reason: "correccion_perfil",
    },
  );
  assert.equal(updateOk, false);
  assert.equal(calls.length, 0);
  assert.ok(facade.getSnapshot().error?.includes("expected_updated_at"));

  unsubscribe();
});

test("admin facade: gate 428 recent auth, 409 conflict, and 403 memory purge", async () => {
  const { AdminFacade } = await import(pathToFileURL(facadePath).href);
  const { privateBlobUrls } = await import(
    pathToFileURL(path.join(scratch, "private-blob-urls.mjs")).href
  );

  let errorToThrow = null;
  const mockPort = {
    async updateGlobalAccountProfile() {
      if (errorToThrow) throw errorToThrow;
    },
    async revokeGlobalAccountSessions() {
      if (errorToThrow) throw errorToThrow;
    },
    async getGlobalAdminOverview() {
      if (errorToThrow) throw errorToThrow;
      return { total_accounts: 10, by_state: {}, by_role: {} };
    },
  };

  const facade = new AdminFacade(mockPort);

  // Gate 428: Precondition Required (Recent Auth)
  errorToThrow = { status: 428, code: "precondition_required" };
  const ok428 = await facade.updateGlobalAccountProfile(
    "test-token",
    "account-1",
    {
      display_name: "Admin",
      expected_updated_at: "2026-10-25T14:30:00Z",
      reason: "correccion_perfil",
    },
  );
  assert.equal(ok428, false);
  assert.equal(facade.getSnapshot().recentAuthRequired, true);
  assert.ok(facade.getSnapshot().error?.includes("428"));

  facade.clearError();
  assert.equal(facade.getSnapshot().recentAuthRequired, false);
  assert.equal(facade.getSnapshot().error, null);

  // Gate 409: Concurrency Conflict
  errorToThrow = { status: 409, code: "concurrency_conflict" };
  const ok409 = await facade.updateGlobalAccountProfile(
    "test-token",
    "account-1",
    {
      display_name: "Admin",
      expected_updated_at: "2026-10-25T14:30:00Z",
      reason: "correccion_perfil",
    },
  );
  assert.equal(ok409, false);
  assert.equal(facade.getSnapshot().concurrencyConflict, true);
  assert.ok(facade.getSnapshot().error?.includes("409"));

  // Populate some private data first
  errorToThrow = null;
  await facade.loadOverview("test-token");
  assert.notEqual(facade.getSnapshot().overview, null);

  // Gate 403: Forbidden triggers immediate memory purge of private data
  const clearBefore = privateBlobUrls.clearCount;
  errorToThrow = { status: 403, code: "forbidden" };
  await facade.loadOverview("test-token");

  // Snapshot must have private data purged
  assert.equal(facade.getSnapshot().overview, null);
  assert.equal(facade.getSnapshot().globalAccounts.length, 0);
  assert.equal(facade.getSnapshot().verifications.length, 0);
  assert.ok(facade.getSnapshot().error?.includes("purgados de la memoria"));
  assert.ok(privateBlobUrls.clearCount > clearBefore);
});
