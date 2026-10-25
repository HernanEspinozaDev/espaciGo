import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-account-domain-"),
);

async function compile(sourcePath, outputName, replacements = []) {
  let source = await readFile(path.join(root, sourcePath), "utf8");
  for (const [before, after] of replacements)
    source = source.replaceAll(before, after);
  const output = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText;
  const destination = path.join(scratch, outputName);
  await writeFile(destination, output);
  return destination;
}

const profilePath = await compile(
  "src/features/account/domain/profile.ts",
  "profile.mjs",
);
const photoPath = await compile(
  "src/features/account/domain/photo.ts",
  "photo.mjs",
);
const payoutPath = await compile(
  "src/features/account/domain/payout-account.ts",
  "payout-account.mjs",
);
const privacyPath = await compile(
  "src/features/account/domain/privacy.ts",
  "privacy.mjs",
);
const verifPath = await compile(
  "src/features/verifications/domain/verification.ts",
  "verification.mjs",
);

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

test("account domain validates Chilean phone format and normalizes valid numbers", async () => {
  const {
    isValidChileanPhone,
    normalizeChileanPhone,
    validateProfileInput,
    normalizeProfileUpdatePayload,
  } = await import(pathToFileURL(profilePath).href);

  // Direct 9 digits
  assert.equal(isValidChileanPhone("912345678"), true);
  assert.equal(normalizeChileanPhone("912345678"), "912345678");

  // Formatted with spaces, dashes, parens
  assert.equal(isValidChileanPhone("9 1234 5678"), true);
  assert.equal(normalizeChileanPhone("9 1234 5678"), "912345678");
  assert.equal(normalizeChileanPhone("9-1234-5678"), "912345678");
  assert.equal(normalizeChileanPhone("(9) 1234 5678"), "912345678");

  // Chilean country code +56
  assert.equal(isValidChileanPhone("+56 9 1234 5678"), true);
  assert.equal(normalizeChileanPhone("+56 9 1234 5678"), "912345678");
  assert.equal(normalizeChileanPhone("+56912345678"), "912345678");
  assert.equal(normalizeChileanPhone("56912345678"), "912345678");

  // Chilean landline (9 digits total, e.g. Santiago 2...)
  assert.equal(isValidChileanPhone("221234567"), true);
  assert.equal(normalizeChileanPhone("221234567"), "221234567");

  // Invalid formats
  assert.equal(isValidChileanPhone("12345678"), false); // 8 digits
  assert.equal(isValidChileanPhone("1234567890"), false); // 10 digits
  assert.equal(isValidChileanPhone("91234abcd"), false); // alpha chars
  assert.equal(isValidChileanPhone("+54 9 1234 5678"), false); // Argentina
  assert.equal(normalizeChileanPhone("invalid"), null);

  // Optional: null or empty string returns null without error
  assert.equal(normalizeChileanPhone(null), null);
  assert.equal(normalizeChileanPhone(undefined), null);
  assert.equal(normalizeChileanPhone("   "), null);

  // Full profile validation with phone
  const validRes = validateProfileInput({
    displayName: "Ana Pérez",
    phone: "+56 9 8765 4321",
  });
  assert.equal(validRes.valid, true);
  assert.deepEqual(validRes.errors, {});

  const payload = normalizeProfileUpdatePayload({
    displayName: "  Ana Pérez  ",
    phone: "+56 9 8765 4321",
  });
  assert.deepEqual(payload, {
    display_name: "Ana Pérez",
    phone: "987654321",
  });

  // Without phone
  const withoutPhone = normalizeProfileUpdatePayload({
    displayName: "Carlos Muñoz",
    phone: "",
  });
  assert.deepEqual(withoutPhone, {
    display_name: "Carlos Muñoz",
  });
});

test("account domain validates profile name rules according to CU-07", async () => {
  const { isValidProfileName, validateProfileInput } = await import(
    pathToFileURL(profilePath).href
  );

  // Valid names: letters with accents and spaces
  assert.equal(isValidProfileName("Juan Pérez"), true);
  assert.equal(
    isValidProfileName("María del Carmen O'Higgins".replace("'", "")),
    true,
  );
  assert.equal(isValidProfileName("Sofía Ñuñoa"), true);

  // Invalid names
  assert.equal(isValidProfileName(""), false);
  assert.equal(isValidProfileName("   "), false);
  assert.equal(isValidProfileName("Juan123"), false);
  assert.equal(isValidProfileName("Admin <script>"), false);
  assert.equal(isValidProfileName("a".repeat(121)), false);

  const invalidRes = validateProfileInput({
    displayName: "12345",
    phone: "invalid",
  });
  assert.equal(invalidRes.valid, false);
  assert.ok(invalidRes.errors.displayName);
  assert.ok(invalidRes.errors.phone);
});

test("verification domain governs states, eligibility, and retry conditions", async () => {
  const { VERIFICATION_STATES, canRetryVerification, isVerificationEligible } =
    await import(pathToFileURL(verifPath).href);

  assert.deepEqual(VERIFICATION_STATES, [
    "en_revision",
    "aprobada",
    "rechazada",
    "revocada",
  ]);

  // Retry allowed only for rechazada
  assert.equal(canRetryVerification({ state: "rechazada" }), true);
  assert.equal(canRetryVerification({ state: "en_revision" }), false);
  assert.equal(canRetryVerification({ state: "aprobada" }), false);
  assert.equal(canRetryVerification({ state: "revocada" }), false);
  assert.equal(canRetryVerification(null), false);

  // Eligibility evaluation
  const eligibilityList = [
    {
      type: "kyc",
      eligible: true,
      verification_id: "v-1",
      granted_at: "2026-10-20T10:00:00Z",
      revoked_at: null,
      revocation_reason_code: "",
    },
    {
      type: "kyb",
      eligible: false,
      verification_id: "v-2",
      granted_at: null,
      revoked_at: "2026-10-21T10:00:00Z",
      revocation_reason_code: "documento_vencido",
    },
  ];

  assert.equal(isVerificationEligible(eligibilityList, "kyc"), true);
  assert.equal(isVerificationEligible(eligibilityList, "kyb"), false);
  assert.equal(isVerificationEligible([], "kyc"), false);
  assert.equal(isVerificationEligible(null, "kyc"), false);
});

test("verification and account domain normalize synthetic payloads", async () => {
  const {
    SYNTHETIC_EVIDENCE_FIXTURE_CODE,
    normalizeVerificationCreatePayload,
    normalizeVerificationRetryPayload,
    normalizeSyntheticEvidenceCreatePayload,
  } = await import(pathToFileURL(verifPath).href);

  const { SYNTHETIC_PHOTO_FIXTURE_CODE, isSyntheticPhoto, isValidPhotoSize } =
    await import(pathToFileURL(photoPath).href);

  const {
    SYNTHETIC_PAYOUT_STATUS,
    isSyntheticPayoutAccountActive,
    normalizeSyntheticPayoutAccount,
  } = await import(pathToFileURL(payoutPath).href);

  // Synthetic evidence payload
  assert.equal(SYNTHETIC_EVIDENCE_FIXTURE_CODE, "synthetic-png-v1");
  assert.deepEqual(normalizeSyntheticEvidenceCreatePayload(), {
    fixture_code: "synthetic-png-v1",
  });

  // Verification create payload
  assert.deepEqual(normalizeVerificationCreatePayload("kyc"), {
    type: "kyc",
  });
  assert.deepEqual(normalizeVerificationCreatePayload("kyb"), {
    type: "kyb",
  });
  assert.throws(() => normalizeVerificationCreatePayload("invalido"));

  // Verification retry payload
  assert.deepEqual(
    normalizeVerificationRetryPayload("fixture_vigente_actualizado"),
    { correction_code: "fixture_vigente_actualizado" },
  );
  assert.throws(() => normalizeVerificationRetryPayload("codigo_inexistente"));

  // Synthetic photo fixture
  assert.equal(SYNTHETIC_PHOTO_FIXTURE_CODE, "synthetic-png-v1");
  assert.equal(
    isSyntheticPhoto({
      id: "p-1",
      fixtureCode: "synthetic-png-v1",
      mimeType: "image/png",
      sha256: "a".repeat(64),
      sizeBytes: 1024,
      createdAt: "2026-10-20T10:00:00Z",
      storageKind: "synthetic_fixed",
    }),
    true,
  );
  assert.equal(isValidPhotoSize(1024), true);
  assert.equal(isValidPhotoSize(2 * 1024 * 1024), true);
  assert.equal(isValidPhotoSize(2 * 1024 * 1024 + 1), false);
  assert.equal(isValidPhotoSize(0), false);

  // Synthetic payout account
  assert.equal(SYNTHETIC_PAYOUT_STATUS, "activa_fake");
  const payout = normalizeSyntheticPayoutAccount({
    reference_id: "pay-fake-1234",
    status: "activa_fake",
    notice: "Referencia local ficticia",
  });
  assert.ok(payout);
  assert.equal(payout.referenceId, "pay-fake-1234");
  assert.equal(payout.status, "activa_fake");
  assert.equal(isSyntheticPayoutAccountActive(payout), true);
  assert.equal(isSyntheticPayoutAccountActive(null), false);
});

test("privacy domain identifies pending suppression requests safely", async () => {
  const { isSuppressionPending, hasResolvedSuppression } = await import(
    pathToFileURL(privacyPath).href
  );

  const requests = [
    {
      id: "r-1",
      kind: "acceso",
      state: "resuelta",
      created_at: "2026-10-10T00:00:00Z",
    },
    {
      id: "r-2",
      kind: "supresion",
      state: "en_revision",
      created_at: "2026-10-15T00:00:00Z",
    },
  ];

  assert.equal(isSuppressionPending(requests), true);
  assert.equal(hasResolvedSuppression(requests), false);

  const resolved = [
    {
      id: "r-3",
      kind: "supresion",
      state: "resuelta",
      created_at: "2026-10-16T00:00:00Z",
    },
  ];
  assert.equal(isSuppressionPending(resolved), false);
  assert.equal(hasResolvedSuppression(resolved), true);
});
