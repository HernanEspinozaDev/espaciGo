import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const repoRoot = path.resolve(import.meta.dirname, "..");
const rulesSourcePath = path.join(
  repoRoot,
  "src/features/bookings/domain/booking-rules.ts",
);

const scratch = await mkdtemp(path.join(os.tmpdir(), "bookings-domain-test-"));
const rulesTranspiledPath = path.join(scratch, "booking-rules.mjs");

const rawCode = await readFile(rulesSourcePath, "utf8");
const transpiled = ts.transpileModule(rawCode, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;

await writeFile(rulesTranspiledPath, transpiled, "utf8");

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

test("booking rules validates UUID and generates valid Idempotency-Key", async () => {
  const { isValidUUID, generateIdempotencyKey } = await import(
    pathToFileURL(rulesTranspiledPath).href
  );

  assert.equal(isValidUUID("123e4567-e89b-12d3-a456-426614174000"), true);
  assert.equal(isValidUUID("d3b07384-d113-4632-a6f9-03a89a05b38a"), true);
  assert.equal(isValidUUID("not-a-uuid"), false);
  assert.equal(isValidUUID(""), false);
  assert.equal(isValidUUID(null), false);
  assert.equal(isValidUUID(undefined), false);

  const generated = generateIdempotencyKey();
  assert.equal(typeof generated, "string");
  assert.equal(isValidUUID(generated), true);
});

test("booking rules validates CLP currency amounts and non-negative integers", async () => {
  const { isValidCLPAmount, isValidNonNegativeCLPAmount } = await import(
    pathToFileURL(rulesTranspiledPath).href
  );

  assert.equal(isValidCLPAmount(50000), true);
  assert.equal(isValidCLPAmount(1), true);
  assert.equal(isValidCLPAmount(0), false);
  assert.equal(isValidCLPAmount(-500), false);
  assert.equal(isValidCLPAmount(1500.5), false);
  assert.equal(isValidCLPAmount("50000"), false);
  assert.equal(isValidCLPAmount(NaN), false);

  assert.equal(isValidNonNegativeCLPAmount(0), true);
  assert.equal(isValidNonNegativeCLPAmount(50000), true);
  assert.equal(isValidNonNegativeCLPAmount(-1), false);
  assert.equal(isValidNonNegativeCLPAmount(0.5), false);
});

test("booking rules validates rate units and allowed duration units", async () => {
  const { isValidRateUnit, isAllowedDurationUnits } = await import(
    pathToFileURL(rulesTranspiledPath).href
  );

  assert.equal(isValidRateUnit("hora"), true);
  assert.equal(isValidRateUnit("dia"), true);
  assert.equal(isValidRateUnit("mes"), true);
  assert.equal(isValidRateUnit("semana"), false);

  assert.equal(isAllowedDurationUnits("hora", 1), true);
  assert.equal(isAllowedDurationUnits("hora", 2), true);
  assert.equal(isAllowedDurationUnits("hora", 4), true);
  assert.equal(isAllowedDurationUnits("hora", 3), false);
  assert.equal(isAllowedDurationUnits("dia", 1), true);
  assert.equal(isAllowedDurationUnits("dia", 2), true);
  assert.equal(isAllowedDurationUnits("dia", 3), true);
  assert.equal(isAllowedDurationUnits("dia", 4), false);
  assert.equal(isAllowedDurationUnits("mes", 1), true);
  assert.equal(isAllowedDurationUnits("mes", 2), false);
});

test("booking rules enforces terminal states and read-only behavior", async () => {
  const { isTerminalState, isStateReadOnly } = await import(
    pathToFileURL(rulesTranspiledPath).href
  );

  assert.equal(isTerminalState("cerrada"), true);
  assert.equal(isTerminalState("cancelada_por_firma"), true);
  assert.equal(isTerminalState("cancelada_por_pago"), true);
  assert.equal(isTerminalState("rechazada_arrendador"), true);
  assert.equal(isTerminalState("vencida_pago"), true);
  assert.equal(isTerminalState("vencida_host"), true);
  assert.equal(isTerminalState("cancelada_arrendatario"), true);
  assert.equal(isTerminalState("finalizada"), false);
  assert.equal(isTerminalState("en_curso"), false);
  assert.equal(isTerminalState("pendiente_de_pago"), false);

  assert.equal(isStateReadOnly("cerrada"), true);
  assert.equal(isStateReadOnly("cancelada_arrendatario"), true);
  assert.equal(isStateReadOnly("en_curso"), false);
});

test("booking rules enforces valid state transitions and detects illegal moves", async () => {
  const { canTransitionState } = await import(
    pathToFileURL(rulesTranspiledPath).href
  );

  assert.equal(canTransitionState("pendiente_de_pago", "pagada"), true);
  assert.equal(
    canTransitionState("pendiente_de_pago", "cancelada_por_pago"),
    true,
  );
  assert.equal(canTransitionState("pendiente_de_pago", "en_curso"), false);

  assert.equal(canTransitionState("pagada", "aprobada_host"), true);
  assert.equal(canTransitionState("pagada", "rechazada_arrendador"), true);
  assert.equal(canTransitionState("pagada", "vencida_host"), true);
  assert.equal(canTransitionState("pagada", "cerrada"), false);

  assert.equal(canTransitionState("aprobada_host", "firma_parcial"), true);
  assert.equal(canTransitionState("aprobada_host", "lista_para_checkin"), true);
  assert.equal(
    canTransitionState("aprobada_host", "cancelada_por_firma"),
    true,
  );

  assert.equal(canTransitionState("lista_para_checkin", "en_curso"), true);
  assert.equal(canTransitionState("en_curso", "finalizada"), true);
  assert.equal(canTransitionState("en_curso", "en_disputa"), true);
  assert.equal(canTransitionState("finalizada", "cerrada"), true);

  // Terminal state has no transitions
  assert.equal(canTransitionState("cerrada", "en_curso"), false);
  assert.equal(
    canTransitionState("cancelada_arrendatario", "pendiente_de_pago"),
    false,
  );
});

test("booking rules deduplicates entities by id maintaining ordering", async () => {
  const { deduplicateById } = await import(
    pathToFileURL(rulesTranspiledPath).href
  );

  const items = [
    { id: "1", text: "primer mensaje" },
    { id: "2", text: "segundo mensaje" },
    { id: "1", text: "primer mensaje duplicado ignorado" },
    { id: "3", text: "tercer mensaje" },
  ];

  const deduplicated = deduplicateById(items);
  assert.equal(deduplicated.length, 3);
  assert.equal(deduplicated[0].id, "1");
  assert.equal(deduplicated[0].text, "primer mensaje");
  assert.equal(deduplicated[1].id, "2");
  assert.equal(deduplicated[2].id, "3");
});
