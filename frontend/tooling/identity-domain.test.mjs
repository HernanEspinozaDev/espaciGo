import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-identity-domain-"),
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

await compile("src/lib/api/clp.ts", "clp.mjs");
await compile("src/lib/api/time.ts", "time.mjs");
await compile(
  "src/features/public-catalog/domain/query-state.ts",
  "query-state.mjs",
  [
    ["@/lib/api/clp", "./clp.mjs"],
    ["@/lib/api/time", "./time.mjs"],
  ],
);
const returnPath = await compile(
  "src/features/auth/domain/return-target.ts",
  "return-target.mjs",
  [["@/features/public-catalog/domain/query-state", "./query-state.mjs"]],
);
const credentialPath = await compile(
  "src/features/auth/domain/action-credential.ts",
  "action-credential.mjs",
);
const consentPath = await compile(
  "src/features/auth/domain/consent-documents.ts",
  "consent-documents.mjs",
);
const passwordPath = await compile(
  "src/features/auth/domain/password-policy.ts",
  "password-policy.mjs",
);
const retryPath = await compile(
  "src/features/auth/domain/retry-after.ts",
  "retry-after.mjs",
);
const copyPath = await compile(
  "src/features/auth/domain/onboarding-errors.ts",
  "onboarding-errors.mjs",
);
const facadePath = await compile(
  "src/features/auth/application/onboarding-facade.ts",
  "onboarding-facade.mjs",
);
const errorsPath = await compile("src/lib/api/errors.ts", "api-errors.mjs");
await writeFile(
  path.join(scratch, "private-blob-urls.mjs"),
  "export const privateBlobUrls = { clear() {} };",
);
const authFacadePath = await compile(
  "src/features/auth/application/auth-facade.ts",
  "auth-facade.mjs",
  [
    ['import "client-only";', ""],
    ["@/lib/api/errors", "./api-errors.mjs"],
    ["@/lib/api/private-blob-urls", "./private-blob-urls.mjs"],
  ],
);
const returnTarget = await import(pathToFileURL(returnPath));
const credentials = await import(pathToFileURL(credentialPath));
const consent = await import(pathToFileURL(consentPath));
const passwords = await import(pathToFileURL(passwordPath));
const retry = await import(pathToFileURL(retryPath));
const copy = await import(pathToFileURL(copyPath));
const { OnboardingFacade } = await import(pathToFileURL(facadePath));
const apiErrors = await import(pathToFileURL(errorsPath));
const { AuthFacade } = await import(pathToFileURL(authFacadePath));

const uuid = "550e8400-e29b-41d4-a716-446655440000";

function document(id, type) {
  return {
    id,
    code: type,
    type,
    sha256: "a".repeat(64),
    published_at: "2026-10-25T12:00:00Z",
    synthetic: true,
    document: {
      id,
      code: type,
      type,
      title:
        type === "terminos" ? "Términos sintéticos" : "Privacidad sintética",
      scope: "ensayo_local",
      status: "vigente",
      sections: [
        { heading: "Alcance", body: "Contenido sintético de ensayo." },
      ],
    },
  };
}

const terms = document(uuid, "terminos");
const privacy = document("660e8400-e29b-41d4-a716-446655440000", "privacidad");

test("return_to accepts only allowlisted same-origin routes", () => {
  for (const target of [
    "/",
    "/explorar",
    "/categorias",
    "/como-funciona",
    "/anfitriones",
    "/arriendos",
    "/dashboard",
    "/dashboard/general",
    "/dashboard/security",
    "/dashboard/activity",
    "/categorias/inmuebles",
    `/espacios/${uuid}`,
  ])
    assert.equal(returnTarget.resolveReturnTarget(target), target);
  assert.equal(
    returnTarget.resolveReturnTarget(
      "/explorar?q=  casa  &category_code=inmuebles&page_size=3",
    ),
    "/explorar?q=casa&category_code=inmuebles&page_size=3",
  );
});

test("return_to rejects open redirects, traversal, auth/API paths, fragments, and unknown query", () => {
  for (const target of [
    "https://outside.invalid/",
    "//outside.invalid/",
    "\\\\outside.invalid",
    "/\\\\outside.invalid",
    "/../dashboard",
    "/dashboard/../login",
    "/api/backend/auth/session",
    "/acceso",
    "/login",
    "/logout",
    "/explorar?return_to=https://outside.invalid",
    "/explorar?token=secret",
    "/dashboard?unknown=1",
    "/explorar#fragment",
    "/%2f%2foutside.invalid",
    "/espacios/not-a-uuid",
    "/categorias/InvalidCode",
    "/explorar?cursor=a&cursor=b",
    "/explorar?page_size=99",
    "/explorar?region_code=1",
    `/espacios/${uuid.toUpperCase()}`,
  ])
    assert.equal(returnTarget.resolveReturnTarget(target), "/", target);
  assert.equal(
    returnTarget.resolveReturnParam(["/dashboard", "/arriendos"]),
    "/",
  );
  assert.equal(returnTarget.resolveReturnParam(["/dashboard"]), "/dashboard");
});

test("action fragment accepts only one token pair and never returns rejected values", () => {
  const token = "synthetic-test-action-token";
  const parsed = credentials.parseActionFragment(
    `#token=${token}&token_id=${uuid}`,
  );
  assert.equal(parsed.ok, true);
  if (parsed.ok) {
    assert.equal(parsed.credential.tokenId, uuid);
    assert.equal(parsed.credential.token, token);
  }
  for (const invalid of [
    `#token=${token}&token_id=${uuid}&extra=x`,
    `#token=${token}&token=${token}&token_id=${uuid}`,
    `#token=${token}&token_id=not-a-uuid`,
    `#token=${token}%0A&token_id=${uuid}`,
    `#token=${token}&token_id=${uuid.toUpperCase()}`,
    `#token=${token}&token_id=${uuid}&x=1`,
    "?token=not-a-fragment",
  ]) {
    const result = credentials.parseActionFragment(invalid);
    assert.deepEqual(result, { ok: false });
    assert.equal(JSON.stringify(result).includes(token), false);
  }
});

test("consent documents require exactly one complete version of each type", () => {
  const accepted = consent.validateConsentDocuments({
    items: [terms, privacy],
  });
  assert.equal(accepted?.terms.id, uuid);
  assert.equal(accepted?.privacy.id, privacy.id);
  assert.equal(consent.validateConsentDocuments({ items: [terms] }), null);
  assert.equal(
    consent.validateConsentDocuments({ items: [terms, privacy, privacy] }),
    null,
  );
  assert.equal(
    consent.validateConsentDocuments({
      items: [terms, { ...privacy, id: uuid }],
    }),
    null,
  );
  assert.equal(
    consent.validateConsentDocuments({
      items: [
        terms,
        { ...privacy, document: { ...privacy.document, sections: [] } },
      ],
    }),
    null,
  );
  assert.equal(
    consent.validateConsentDocuments({
      items: [terms, { ...privacy, synthetic: false }],
    }),
    null,
  );
  for (const invalidPrivacy of [
    { ...privacy, sha256: "not-a-sha" },
    { ...privacy, published_at: "not-a-timestamp" },
    { ...privacy, code: "other" },
    {
      ...privacy,
      document: { ...privacy.document, status: "" },
    },
  ]) {
    assert.equal(
      consent.validateConsentDocuments({ items: [terms, invalidPrivacy] }),
      null,
    );
  }
});

test("password policy counts Unicode code points and UTF-8 bytes without requiring lowercase", () => {
  assert.equal(passwords.inspectPassword("ABC123!!").valid, true);
  assert.equal(passwords.inspectPassword("A1!short").valid, true);
  assert.equal(passwords.inspectPassword("A1!é").valid, false);
  assert.equal(passwords.inspectPassword("A1!é".repeat(2)).valid, true);
  assert.equal(
    passwords.inspectPassword(`A1!${"é".repeat(34)}x`).utf8Bytes,
    72,
  );
  assert.equal(passwords.inspectPassword(`A1!${"é".repeat(34)}x`).valid, true);
  assert.equal(passwords.inspectPassword(`A1!${"é".repeat(35)}`).utf8Bytes, 73);
  assert.equal(passwords.inspectPassword(`A1!${"é".repeat(35)}`).valid, false);
});

test("Retry-After accepts seconds and countdown is deadline-based", () => {
  assert.equal(retry.retryAfterSeconds("30"), 30);
  assert.equal(retry.retryAfterSeconds("0"), null);
  assert.equal(retry.retryAfterSeconds("1.5"), null);
  assert.equal(retry.retryAfterSeconds("Wed, 21 Oct 2015 07:28:00 GMT"), null);
  assert.equal(retry.remainingRetrySeconds(20_000, 1_500), 19);
  assert.equal(retry.remainingRetrySeconds(1_000, 1_500), 0);
});

test("auth errors cover the actual email_unverified and recovery codes safely", () => {
  assert.equal(
    copy.onboardingErrorCopy("email_unverified"),
    "Verifica tu correo antes de iniciar sesión.",
  );
  assert.equal(
    copy.onboardingErrorCopy("invalid_token"),
    copy.onboardingErrorCopy("invalid_token"),
  );
  assert.equal(
    copy.onboardingErrorCopy("password_confirmation_mismatch"),
    "Las contraseñas no coinciden.",
  );
  assert.equal(
    copy.onboardingErrorCopy("password_unchanged"),
    "Elige una contraseña distinta de la actual.",
  );
  assert.equal(
    copy.onboardingErrorCopy("password_recently_used"),
    "Elige una contraseña que no hayas usado recientemente.",
  );
  assert.equal(copy.recoveryAcceptedCopy(), copy.recoveryAcceptedCopy());
  assert.equal(copy.reissueAcceptedCopy().includes("correo"), true);
  const error = new apiErrors.ApiError({
    status: 401,
    code: "email_unverified",
    requestId: null,
    retryAfter: null,
    fields: [],
  });
  assert.equal(
    apiErrors.authErrorMessage(error),
    "Verifica tu correo antes de iniciar sesión.",
  );
});

test("facade discards a stale result after cancel", async () => {
  let finish;
  const port = {
    terms: () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
    register: async () => 201,
    reissueVerification: async () => {},
    verifyEmail: async () => {},
    requestRecovery: async () => {},
    consumeRecovery: async () => {},
  };
  const facade = new OnboardingFacade(port);
  const pending = facade.terms();
  facade.cancel();
  finish({ items: [] });
  await assert.rejects(pending, { name: "AbortError" });
});

test("auth session is confirmed by the session port and expires without retaining usable auth", async () => {
  const expiresAt = new Date(Date.now() + 100).toISOString();
  const accessToken = "synthetic-memory-only-session-token";
  let notifications = 0;
  let authenticatedRequestCalled = false;
  const auth = new AuthFacade({
    login: async () => ({
      accessToken,
      session: {
        accountId: uuid,
        roles: [],
        capabilities: [],
        restrictedMode: false,
        expiresAt,
      },
    }),
    currentSession: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
    }),
    logout: async () => {},
  });
  auth.subscribe(() => {
    notifications += 1;
  });

  assert.equal(
    await auth.login({ email: "test@example.invalid", password: "ABC123!!" }),
    true,
  );
  assert.equal(auth.getSnapshot().status, "authenticated");
  await new Promise((resolve) => setTimeout(resolve, 175));
  assert.equal(auth.getSnapshot().status, "anonymous");
  assert.ok(notifications >= 3);
  await assert.rejects(
    auth.runAuthenticated(async () => {
      authenticatedRequestCalled = true;
      return { response: new Response(null, { status: 200 }), value: true };
    }),
    (error) => error instanceof apiErrors.ApiError && error.status === 401,
  );
  assert.equal(authenticatedRequestCalled, false);
});

test.after(async () => rm(scratch, { recursive: true, force: true }));
