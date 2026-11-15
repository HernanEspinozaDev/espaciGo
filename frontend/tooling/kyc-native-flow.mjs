import fs from "node:fs";
import { createHmac, randomUUID } from "node:crypto";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const origin = "https://127.0.0.1:4219";
const webhookMode = process.env.KYC_NATIVE_WEBHOOK === "1";
const idempotencyMode = process.env.KYC_NATIVE_IDEMPOTENCY === "1";
const identityVariantMode = process.env.KYC_NATIVE_UNICODE === "1";
const identity = identityVariantMode
  ? {
      firstName: "Ángela Ñora",
      lastName: "Pérez-Gómez",
      dateOfBirth: "1990-01-02",
      inputRut: "12.345.670-k",
      rut: "12345670-K",
      displayRut: "12.345.670-K",
    }
  : {
      firstName: "Synthetic",
      lastName: "Person",
      dateOfBirth: "1990-01-02",
      inputRut: "11.222.333-9",
      rut: "11222333-9",
      displayRut: "11.222.333-9",
    };
const identityMarkers = [
  identity.firstName,
  identity.lastName,
  identity.dateOfBirth,
  identity.rut,
  identity.displayRut,
];
if (process.env.KYC_NATIVE_FLOW !== "1" || process.argv.slice(2).length !== 0) {
  console.error("KYC native flow: explicit opt-in and no arguments required");
  process.exit(2);
}
function readFixture(name) {
  const dir = resolve(root, ".local/testing/kyc-native");
  const parent = fs.lstatSync(dir);
  if (
    !parent.isDirectory() ||
    parent.isSymbolicLink() ||
    (parent.mode & 0o777) !== 0o700 ||
    parent.uid !== process.getuid()
  )
    throw new Error("fixture-parent-policy");
  const path = resolve(dir, name);
  const stat = fs.lstatSync(path);
  if (
    !stat.isFile() ||
    stat.isSymbolicLink() ||
    (stat.mode & 0o777) !== 0o600 ||
    stat.uid !== process.getuid()
  )
    throw new Error("fixture-file-policy");
  return JSON.parse(fs.readFileSync(path, "utf8"));
}
let fixture, secondFixture;
try {
  fixture = readFixture("fixture.json");
  secondFixture = readFixture("fixture-2.json");
} catch {
  console.error("KYC native flow: private fixture validation failed");
  process.exit(1);
}
const result = {
  anonymousStatus: 0,
  loginStatus: 0,
  sessionCookieFlagsValid: false,
  declarationInitialStatus: 0,
  saveStatus: 0,
  readbackStatus: 0,
  reloadReadbackStatus: 0,
  noDiditConsentBlocked: false,
  noDiditCallBeforeConsent: false,
  diditIntentStatus: 0,
  diditIntentReadbackStatus: 0,
  diditIntentStatusValueAllowed: false,
  logoutStatus: 0,
  postLogoutSessionStatus: 0,
  secondLoginStatus: 0,
  crossAccountDeclarationStatus: 0,
  crossAccountContainsPII: true,
  crossAccountUIEmpty: false,
  crossAccountFieldsPresent: false,
  crossAccountFieldsEmpty: false,
  crossAccountConsentsUnchecked: false,
  crossAccountNoIdentityMarkers: false,
  secondLogoutStatus: 0,
  secondPostLogoutSessionStatus: 0,
  returnLoginStatus: 0,
  returnReadbackStatus: 0,
  returnReadbackMatches: false,
  returnReadbackRendered: false,
  finalLogoutStatus: 0,
  finalPostLogoutSessionStatus: 0,
  noSessionCookiesAfterLogout: false,
  diditRedirectIntercepted: false,
  diditRedirectHostAllowed: false,
  diditRedirectPathValid: false,
  unexpectedExternalRequests: 0,
  piiInBrowserStorageOrUrl: true,
  ownReturnQueryCleaned: false,
  ownReturnReadbackStatus: 0,
  ownReturnCleanBeforeReadback: false,
  ownReturnPendingBadge: false,
  ownReturnReviewBadge: false,
  webhookInvalidSignatureStatus: 0,
  webhookStaleStatus: 0,
  webhookUnknownSessionStatus: 0,
  webhookValidStatus: 0,
  webhookValidProcessingStatus: false,
  webhookReplayStatus: 0,
  webhookReplayProcessingStatus: false,
  webhookAuthoritativeInReview: false,
  ownReturnNoVerifiedBadge: false,
  crossAccountReturnStatus: 0,
  crossAccountReturnCleanBeforeReadback: false,
  crossAccountReturnSafeError: false,
  crossAccountReturnRetryStatus: 0,
  crossAccountReturnRetrySafe: false,
  noProviderOnSave: false,
  identityVariantEnabled: identityVariantMode,
  identityVariantReadbackVerified: false,
  noProviderWithoutConsent: false,
  providerCreatesAfterStart: -1,
  replayStatus: 0,
  replaySameAttempt: false,
  replaySameRedirect: false,
  providerCreatesAfterReplay: -1,
  activeResumeStatus: 0,
  activeResumeSameAttempt: false,
  activeResumeSameRedirect: false,
  providerCreatesAfterActiveResume: -1,
  rutConflictStatus: 0,
  rutConflictInputMatches: false,
  rutConflictMessage: false,
  rutConflictNoSavedSummary: false,
  rutConflictNoDiditAction: false,
  rutConflictReadbackStatus: 0,
  rutConflictReadbackEmpty: false,
  rutConflictProviderUnchanged: false,
};
const allowedDiditHosts = new Set(["verify.didit.me", "verification.didit.me"]);
let diditPostCount = 0;
let observedIntent = null;
let observedIntentRequest = null;
let providerCreationBaseline = -1;
let webhookSessionID = "";
let stage = "launch";
async function readProviderStats() {
  const response = await fetch("http://127.0.0.1:19091/__local_test/stats", {
    method: "GET",
    redirect: "error",
    signal: AbortSignal.timeout(5000),
    headers: { "x-api-key": "local-kyc-test-api-key" },
  });
  if (response.status !== 200) throw new Error("provider-stats-status");
  let body;
  try {
    body = await response.json();
  } catch {
    throw new Error("provider-stats-shape");
  }
  if (
    !body ||
    Array.isArray(body) ||
    Object.keys(body).length !== 2 ||
    !Object.hasOwn(body, "creation_requests") ||
    !Object.hasOwn(body, "active_sessions") ||
    !Number.isInteger(body.creation_requests) ||
    body.creation_requests < 0 ||
    !Number.isInteger(body.active_sessions) ||
    body.active_sessions < 0
  )
    throw new Error("provider-stats-shape");
  return body.creation_requests;
}
(async () => {
  const browser = await chromium.launch({
    headless: true,
    args: ["--no-sandbox"],
  });
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    serviceWorkers: "block",
  });
  const page = await context.newPage();
  page.setDefaultTimeout(20000);
  page.setDefaultNavigationTimeout(20000);
  await context.route("**/*", async (route) => {
    const url = new URL(route.request().url());
    if (
      url.origin === origin &&
      url.pathname === "/api/backend/verifications/didit/intent" &&
      route.request().method() === "POST"
    ) {
      const upstream = await route.fetch({ maxRedirects: 0 });
      observedIntent = {
        status: upstream.status(),
        body: await upstream.json(),
      };
      if (idempotencyMode)
        observedIntentRequest = {
          postData: route.request().postData(),
          idempotencyKey: route.request().headers()["idempotency-key"],
        };
      return route.fulfill({ response: upstream });
    }
    if (url.protocol === "blob:" && new URL(url.pathname).origin === origin)
      return route.continue();
    if (url.origin === origin) return route.continue();
    if (
      url.protocol === "https:" &&
      allowedDiditHosts.has(url.hostname) &&
      url.pathname.startsWith("/session/")
    ) {
      result.diditRedirectIntercepted = true;
      result.diditRedirectHostAllowed = allowedDiditHosts.has(url.hostname);
      result.diditRedirectPathValid = /^\/session\/[0-9a-f-]{36}$/i.test(
        url.pathname,
      );
      return route.fulfill({
        status: 200,
        contentType: "text/html; charset=utf-8",
        body: '<!doctype html><html lang="es"><main><h1 id="fake-provider">Proveedor Didit simulado localmente</h1></main></html>',
      });
    }
    result.unexpectedExternalRequests++;
    return route.abort("blockedbyclient");
  });
  page.on("response", (response) => {
    const url = new URL(response.url());
    if (
      url.origin === origin &&
      url.pathname === "/api/backend/verifications/didit/intent" &&
      response.request().method() === "POST"
    )
      diditPostCount++;
  });
  let loggedIn = false;
  let attemptID = "";
  try {
    stage = "anonymous-denial";
    result.anonymousStatus = (
      await page.request.get(
        `${origin}/api/backend/verifications/identity-declaration`,
      )
    ).status();
    stage = "login";
    await page.goto(`${origin}/acceso?return_to=%2Fmi-cuenta`, {
      waitUntil: "domcontentloaded",
    });
    await page.locator('input[name="email"]').fill(fixture.email);
    await page.locator('input[name="password"]').fill(fixture.password);
    const loginResponse = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === "/api/backend/auth/login" &&
        r.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    result.loginStatus = (await loginResponse).status();
    loggedIn = true;
    if (result.loginStatus !== 200) throw new Error("login");
    await page.waitForURL(`${origin}/mi-cuenta`);
    const authCookies = await context.cookies(origin);
    const sessionCookie = authCookies.find(
      (cookie) => cookie.name === "__Host-espacigo-session",
    );
    const expiryCookie = authCookies.find(
      (cookie) => cookie.name === "__Host-espacigo-session-expiry",
    );
    result.sessionCookieFlagsValid =
      !!sessionCookie &&
      !!expiryCookie &&
      sessionCookie.secure &&
      sessionCookie.httpOnly &&
      sessionCookie.sameSite === "Lax" &&
      sessionCookie.path === "/" &&
      expiryCookie.secure &&
      expiryCookie.httpOnly &&
      expiryCookie.sameSite === "Lax" &&
      expiryCookie.path === "/";
    if (!result.sessionCookieFlagsValid)
      throw new Error("session-cookie-flags");

    stage = "identity-form";
    const initialDeclaration = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname ===
          "/api/backend/verifications/identity-declaration" &&
        r.request().method() === "GET",
    );
    await page.goto(`${origin}/mi-cuenta/verificaciones`, {
      waitUntil: "domcontentloaded",
    });
    await page
      .getByRole("heading", { name: /Verificación de usuario/ })
      .first()
      .waitFor({ timeout: 20000 });
    await page
      .locator("#id-first-name")
      .waitFor({ state: "visible", timeout: 20000 });
    result.declarationInitialStatus = (await initialDeclaration).status();
    if (result.declarationInitialStatus !== 404)
      throw new Error("initial-declaration-state");
    if (idempotencyMode) providerCreationBaseline = await readProviderStats();

    stage = "save";
    await page.locator("#id-first-name").fill(identity.firstName);
    await page.locator("#id-last-name").fill(identity.lastName);
    await page.locator("#id-dob").fill(identity.dateOfBirth);
    await page.locator("#id-rut").fill(identity.inputRut);
    await page.locator('input[name="consent"]').check();
    const saveResponse = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname ===
          "/api/backend/verifications/identity-declaration" &&
        r.request().method() === "POST",
    );
    const saveReadback = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname ===
          "/api/backend/verifications/identity-declaration" &&
        r.request().method() === "GET" &&
        r.status() === 200,
    );
    await page
      .getByRole("button", { name: "Guardar datos de identidad" })
      .click();
    result.saveStatus = (await saveResponse).status();
    result.readbackStatus = (await saveReadback).status();
    if (result.saveStatus !== 201 || result.readbackStatus !== 200)
      throw new Error("save-readback");
    await page
      .getByText(/Datos guardados \(revisión #/)
      .waitFor({ timeout: 15000 });

    if (idempotencyMode) {
      result.noProviderOnSave =
        (await readProviderStats()) === providerCreationBaseline;
      if (!result.noProviderOnSave) throw new Error("provider-on-save");
    }

    stage = "reload-readback";
    const reloadReadback = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname ===
          "/api/backend/verifications/identity-declaration" &&
        r.request().method() === "GET" &&
        r.status() === 200,
    );
    await page.reload({ waitUntil: "domcontentloaded" });
    result.reloadReadbackStatus = (await reloadReadback).status();
    await page
      .getByText(/Datos guardados \(revisión #/)
      .waitFor({ timeout: 15000 });
    result.piiInBrowserStorageOrUrl = await page.evaluate((markers) => {
      const stored = [
        ...Object.keys(localStorage).map((k) => localStorage.getItem(k)),
        ...Object.keys(sessionStorage).map((k) => sessionStorage.getItem(k)),
      ].join(" ");
      return (
        markers.some((marker) => stored.includes(marker)) ||
        markers.some((marker) => location.href.includes(marker))
      );
    }, identityMarkers);

    stage = "didit-consent-gate";
    const consent = page.locator('input[name="didit_consent"]');
    if (await consent.isChecked()) throw new Error("didit-consent-preselected");
    const beforeIntentCalls = diditPostCount;
    await page
      .getByRole("button", { name: "Verificar identidad con Didit" })
      .click();
    await page
      .getByRole("alert")
      .filter({
        hasText:
          "Debes autorizar expresamente la verificación biométrica con Didit",
      })
      .waitFor({ timeout: 5000 });
    result.noDiditConsentBlocked = true;
    result.noDiditCallBeforeConsent = diditPostCount === beforeIntentCalls;
    if (idempotencyMode) {
      result.noProviderWithoutConsent =
        (await readProviderStats()) === providerCreationBaseline;
      if (!result.noProviderWithoutConsent)
        throw new Error("provider-without-consent");
    }

    stage = "didit-intent";
    await consent.check();
    const intentResponse = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname ===
          "/api/backend/verifications/didit/intent" &&
        r.request().method() === "POST",
    );
    await page
      .getByRole("button", { name: "Verificar identidad con Didit" })
      .click();
    await intentResponse;
    const intent = observedIntent;
    result.diditIntentStatus = intent.status;
    if (result.diditIntentStatus !== 201) throw new Error("didit-intent");
    const intentBody = intent.body;
    attemptID = intentBody.id;
    const redirectURL = new URL(intentBody.redirect_url);
    if (
      !/^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
        attemptID,
      )
    )
      throw new Error("attempt-id");
    if (
      !allowedDiditHosts.has(redirectURL.hostname) ||
      redirectURL.protocol !== "https:" ||
      !/^\/session\/[0-9a-f-]{36}$/i.test(redirectURL.pathname)
    )
      throw new Error("redirect-url");
    await page.waitForURL(
      (url) =>
        allowedDiditHosts.has(new URL(url).hostname) &&
        /^\/session\/[0-9a-f-]{36}$/i.test(new URL(url).pathname),
      { timeout: 15000 },
    );
    await page.locator("#fake-provider").waitFor({ timeout: 5000 });

    if (idempotencyMode) {
      result.providerCreatesAfterStart =
        (await readProviderStats()) - providerCreationBaseline;
      if (
        result.providerCreatesAfterStart !== 1 ||
        typeof observedIntentRequest?.postData !== "string" ||
        !observedIntentRequest.idempotencyKey
      )
        throw new Error("provider-create-after-start");
      stage = "intent-idempotency-replay";
      const replay = await page.request.post(
        `${origin}/api/backend/verifications/didit/intent`,
        {
          maxRedirects: 0,
          headers: {
            Origin: origin,
            "Content-Type": "application/json",
            "Idempotency-Key": observedIntentRequest.idempotencyKey,
          },
          data: observedIntentRequest.postData,
        },
      );
      result.replayStatus = replay.status();
      let replayBody = {};
      if (result.replayStatus === 201) {
        try {
          replayBody = await replay.json();
        } catch {}
      }
      result.replaySameAttempt = replayBody.id === attemptID;
      result.replaySameRedirect =
        replayBody.redirect_url === intentBody.redirect_url;
      result.providerCreatesAfterReplay =
        (await readProviderStats()) - providerCreationBaseline;
      if (
        result.replayStatus !== 201 ||
        !result.replaySameAttempt ||
        !result.replaySameRedirect ||
        result.providerCreatesAfterReplay !== 1
      )
        throw new Error("intent-idempotency-replay");
      stage = "intent-active-resume-new-key";
      const activeResume = await page.request.post(
        `${origin}/api/backend/verifications/didit/intent`,
        {
          maxRedirects: 0,
          headers: {
            Origin: origin,
            "Content-Type": "application/json",
            "Idempotency-Key": randomUUID(),
          },
          data: observedIntentRequest.postData,
        },
      );
      result.activeResumeStatus = activeResume.status();
      let activeResumeBody = {};
      if (result.activeResumeStatus === 201) {
        try {
          activeResumeBody = await activeResume.json();
        } catch {}
      }
      result.activeResumeSameAttempt = activeResumeBody.id === attemptID;
      result.activeResumeSameRedirect =
        activeResumeBody.redirect_url === intentBody.redirect_url;
      result.providerCreatesAfterActiveResume =
        (await readProviderStats()) - providerCreationBaseline;
      if (
        result.activeResumeStatus !== 201 ||
        !result.activeResumeSameAttempt ||
        !result.activeResumeSameRedirect ||
        result.providerCreatesAfterActiveResume !== 1
      )
        throw new Error("intent-active-resume-new-key");
    }

    stage = "intent-readback";
    await page.goto(`${origin}/mi-cuenta/verificaciones`, {
      waitUntil: "domcontentloaded",
    });
    await page
      .getByRole("heading", { name: /Verificación de usuario/ })
      .first()
      .waitFor({ timeout: 15000 });
    const intentReadback = await page.evaluate(async (id) => {
      const response = await fetch(
        `/api/backend/verifications/didit/intent/${encodeURIComponent(id)}`,
        { cache: "no-store" },
      );
      let body = {};
      try {
        body = await response.json();
      } catch {}
      return {
        status: response.status,
        idMatches: body.id === id,
        attemptStatus: body.status,
      };
    }, attemptID);
    result.diditIntentReadbackStatus = intentReadback.status;
    result.diditIntentStatusValueAllowed =
      intentReadback.idMatches &&
      ["CREATED", "PENDING_USER_INPUT", "PROCESSING"].includes(
        intentReadback.attemptStatus,
      );
    if (webhookMode) {
      stage = "webhook-invalid-signature";
      const sessionPath = redirectURL.pathname;
      webhookSessionID = sessionPath.split("/").at(-1);
      // Public synthetic fixture pinned by the dedicated local API profile.
      // Never import a real provider secret into the browser probe environment.
      const secret = "local-kyc-test-webhook-secret";
      // Deliberate loopback server-to-server webhook test; only this request bypasses the browser-origin router.
      const sendEvent = async ({
        eventID = randomUUID(),
        timestamp = Math.floor(Date.now() / 1000),
        sessionID = webhookSessionID,
        signature = true,
        headerTimestamp = timestamp,
      } = {}) => {
        const bytes = Buffer.from(
          JSON.stringify({
            event_id: eventID,
            webhook_type: "status.updated",
            timestamp,
            session_id: sessionID,
            status: "Approved",
          }),
        );
        const headers = {
          "Content-Type": "application/json",
          "X-Timestamp": String(headerTimestamp),
        };
        headers["X-Signature"] = signature
          ? createHmac("sha256", secret).update(bytes).digest("hex")
          : "00".repeat(32);
        return fetch("http://127.0.0.1:18090/api/v1/webhooks/didit", {
          method: "POST",
          redirect: "error",
          signal: AbortSignal.timeout(10000),
          headers,
          body: bytes,
        });
      };
      result.webhookInvalidSignatureStatus = (
        await sendEvent({ signature: false })
      ).status;
      if (result.webhookInvalidSignatureStatus !== 401)
        throw new Error("webhook-invalid-signature");
      stage = "webhook-stale";
      const stale = Math.floor(Date.now() / 1000) - 600;
      result.webhookStaleStatus = (
        await sendEvent({ timestamp: stale, headerTimestamp: stale })
      ).status;
      if (result.webhookStaleStatus !== 401) throw new Error("webhook-stale");
      stage = "webhook-unknown-session";
      result.webhookUnknownSessionStatus = (
        await sendEvent({ sessionID: randomUUID() })
      ).status;
      if (result.webhookUnknownSessionStatus !== 404)
        throw new Error("webhook-unknown-session");
      stage = "webhook-valid-replay";
      const eventID = randomUUID();
      const timestamp = Math.floor(Date.now() / 1000);
      const eventBytes = Buffer.from(
        JSON.stringify({
          event_id: eventID,
          webhook_type: "status.updated",
          timestamp,
          session_id: webhookSessionID,
          status: "Approved",
        }),
      );
      const signature = createHmac("sha256", secret)
        .update(eventBytes)
        .digest("hex");
      const sendSame = () =>
        fetch("http://127.0.0.1:18090/api/v1/webhooks/didit", {
          method: "POST",
          redirect: "error",
          signal: AbortSignal.timeout(10000),
          headers: {
            "Content-Type": "application/json",
            "X-Timestamp": String(timestamp),
            "X-Signature": signature,
          },
          body: eventBytes,
        });
      const valid = await sendSame();
      result.webhookValidStatus = valid.status;
      const validBody = await valid.json();
      result.webhookValidProcessingStatus =
        validBody.processing_status === "PROCESSED";
      if (valid.status !== 200 || !result.webhookValidProcessingStatus)
        throw new Error("webhook-valid");
      const replay = await sendSame();
      result.webhookReplayStatus = replay.status;
      result.webhookReplayProcessingStatus =
        (await replay.json()).processing_status === "PROCESSED";
      if (replay.status !== 200 || !result.webhookReplayProcessingStatus)
        throw new Error("webhook-replay");
      const authoritative = await page.evaluate(async (id) => {
        const response = await fetch(
          `/api/backend/verifications/didit/intent/${encodeURIComponent(id)}`,
          { cache: "no-store" },
        );
        const body = await response.json();
        return {
          status: response.status,
          attemptStatus: body.status,
          badgeResult: body.badge_result,
        };
      }, attemptID);
      result.webhookAuthoritativeInReview =
        authoritative.status === 200 &&
        authoritative.attemptStatus === "IN_REVIEW" &&
        ["IN_REVIEW", "NONE"].includes(authoritative.badgeResult);
      if (!result.webhookAuthoritativeInReview)
        throw new Error("webhook-authoritative-review");
    }

    stage = "own-return";
    const returnPath = `${origin}/verificaciones/retorno?attempt_id=${encodeURIComponent(attemptID)}&status=Approved&session_token=local-synthetic-callback`;
    let ownDispatchClean = false;
    const ownWait = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname ===
          `/api/backend/verifications/didit/intent/${attemptID}` &&
        response.request().method() === "GET",
    );
    page.on("request", (request) => {
      if (
        new URL(request.url()).pathname ===
          `/api/backend/verifications/didit/intent/${attemptID}` &&
        request.method() === "GET"
      )
        ownDispatchClean = new URL(page.url()).search === "";
    });
    await page.goto(returnPath, { waitUntil: "domcontentloaded" });
    await page
      .getByRole("heading", { name: "Resultado de identidad" })
      .waitFor();
    result.ownReturnReadbackStatus = (await ownWait).status();
    result.ownReturnCleanBeforeReadback = ownDispatchClean;
    if (webhookMode) {
      await page.getByText("En revisión", { exact: true }).waitFor();
      result.ownReturnReviewBadge = true;
    } else {
      await page.getByText("Verificación pendiente", { exact: true }).waitFor();
      result.ownReturnPendingBadge = true;
    }
    result.ownReturnQueryCleaned = new URL(page.url()).search === "";
    result.ownReturnNoVerifiedBadge =
      (await page
        .getByText("Identidad verificada", { exact: true })
        .count()) === 0;
    await page.goto(`${origin}/mi-cuenta/verificaciones`, {
      waitUntil: "domcontentloaded",
    });

    stage = "logout-a";
    result.logoutStatus = await page.evaluate(async () => {
      const response = await fetch("/api/backend/auth/logout", {
        method: "POST",
      });
      return response.status;
    });
    if (result.logoutStatus !== 204) throw new Error("logout-a");
    result.postLogoutSessionStatus = await page.evaluate(
      async () =>
        (await fetch("/api/backend/auth/session", { cache: "no-store" }))
          .status,
    );
    if (result.postLogoutSessionStatus !== 401)
      throw new Error("session-after-a-logout");
    loggedIn = false;

    stage = "login-b";
    await page.goto(`${origin}/acceso?return_to=%2Fmi-cuenta`, {
      waitUntil: "domcontentloaded",
    });
    await page.locator('input[name="email"]').fill(secondFixture.email);
    await page.locator('input[name="password"]').fill(secondFixture.password);
    const secondLoginResponse = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === "/api/backend/auth/login" &&
        r.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    result.secondLoginStatus = (await secondLoginResponse).status();
    loggedIn = true;
    if (result.secondLoginStatus !== 200) throw new Error("login-b");
    await page.waitForURL(`${origin}/mi-cuenta`);

    stage = "cross-account-read";
    await page.goto(`${origin}/mi-cuenta/verificaciones`, {
      waitUntil: "domcontentloaded",
    });
    await page
      .getByRole("heading", { name: /Verificación de usuario/ })
      .first()
      .waitFor({ timeout: 15000 });
    await page
      .locator("#id-first-name")
      .waitFor({ state: "visible", timeout: 15000 });
    const crossAccountState = await page.evaluate((markers) => {
      const first = document.querySelector("#id-first-name");
      const last = document.querySelector("#id-last-name");
      const dob = document.querySelector("#id-dob");
      const rut = document.querySelector("#id-rut");
      const declarationConsent = document.querySelector(
        'input[name="consent"]',
      );
      const diditConsent = document.querySelector(
        'input[name="didit_consent"]',
      );
      const pii = markers.some((marker) =>
        document.body.innerText.includes(marker),
      );
      return {
        fieldsPresent: !!first && !!last && !!dob && !!rut,
        fieldsEmpty:
          first?.value === "" &&
          last?.value === "" &&
          dob?.value === "" &&
          rut?.value === "",
        consentsUnchecked:
          !declarationConsent?.checked && !diditConsent?.checked,
        noIdentityMarkers: !pii,
      };
    }, identityMarkers);
    result.crossAccountFieldsPresent = crossAccountState.fieldsPresent;
    result.crossAccountFieldsEmpty = crossAccountState.fieldsEmpty;
    result.crossAccountConsentsUnchecked = crossAccountState.consentsUnchecked;
    result.crossAccountNoIdentityMarkers = crossAccountState.noIdentityMarkers;
    result.crossAccountUIEmpty =
      result.crossAccountFieldsPresent &&
      result.crossAccountFieldsEmpty &&
      result.crossAccountConsentsUnchecked &&
      result.crossAccountNoIdentityMarkers;
    if (!result.crossAccountUIEmpty) throw new Error("cross-account-ui-state");
    const crossAccount = await page.evaluate(async (markers) => {
      const response = await fetch(
        "/api/backend/verifications/identity-declaration",
        { cache: "no-store" },
      );
      const body = await response.text();
      return {
        status: response.status,
        containsPII: markers.some((marker) => body.includes(marker)),
      };
    }, identityMarkers);
    result.crossAccountDeclarationStatus = crossAccount.status;
    result.crossAccountContainsPII = crossAccount.containsPII;
    if (
      !result.crossAccountDeclarationStatus ||
      result.crossAccountDeclarationStatus !== 404 ||
      result.crossAccountContainsPII
    )
      throw new Error("cross-account-read");
    if (process.env.KYC_NATIVE_CONFLICT === "1") {
      stage = "rut-conflict";
      const conflictBaseline = await readProviderStats();
      const diditBaseline = diditPostCount;
      await page.locator("#id-first-name").fill(identity.firstName);
      await page.locator("#id-last-name").fill(identity.lastName);
      await page.locator("#id-dob").fill(identity.dateOfBirth);
      await page.locator("#id-rut").fill(identity.inputRut);
      await page.locator('input[name="consent"]').check();
      const conflictPost = page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname ===
            "/api/backend/verifications/identity-declaration" &&
          r.request().method() === "POST",
      );
      await page
        .getByRole("button", { name: "Guardar datos de identidad" })
        .click();
      const conflictResponse = await conflictPost;
      result.rutConflictStatus = conflictResponse.status();
      const conflictInput = conflictResponse.request().postDataJSON();
      result.rutConflictInputMatches =
        conflictInput?.rut === identity.rut &&
        conflictInput?.first_name === identity.firstName &&
        conflictInput?.last_name === identity.lastName &&
        conflictInput?.date_of_birth === identity.dateOfBirth &&
        conflictInput?.consent_given === true;
      const conflictAlert = page
        .getByRole("alert")
        .filter({ hasText: "No se pudo registrar este RUT" });
      await conflictAlert.waitFor({ timeout: 10000 });
      const conflictMessage = await conflictAlert.innerText();
      result.rutConflictMessage =
        conflictMessage ===
          "No se pudo registrar este RUT. Revisa los datos ingresados o solicita ayuda." &&
        !/inicia sesión|cuenta asociada/i.test(conflictMessage);
      result.rutConflictNoSavedSummary =
        (await page.getByText("Datos guardados (revisión #").count()) === 0;
      result.rutConflictNoDiditAction =
        (await page
          .getByRole("button", { name: "Verificar identidad con Didit" })
          .count()) === 0;
      const rejectedReadback = await page.evaluate(async (markers) => {
        const response = await fetch(
          "/api/backend/verifications/identity-declaration",
          { cache: "no-store" },
        );
        const body = await response.text();
        return {
          status: response.status,
          empty: !markers.some((marker) => body.includes(marker)),
        };
      }, identityMarkers);
      result.rutConflictReadbackStatus = rejectedReadback.status;
      result.rutConflictReadbackEmpty =
        rejectedReadback.status === 404 && rejectedReadback.empty;
      result.rutConflictProviderUnchanged =
        (await readProviderStats()) === conflictBaseline &&
        diditPostCount === diditBaseline;
      if (
        result.rutConflictStatus !== 409 ||
        !result.rutConflictInputMatches ||
        !result.rutConflictMessage ||
        !result.rutConflictNoSavedSummary ||
        !result.rutConflictNoDiditAction ||
        !result.rutConflictReadbackEmpty ||
        !result.rutConflictProviderUnchanged
      )
        throw new Error("rut-conflict");
    }
    stage = "cross-account-return";
    let crossDispatchClean = false;
    const crossPath = `/api/backend/verifications/didit/intent/${attemptID}`;
    const crossWait = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === crossPath &&
        response.request().method() === "GET",
    );
    page.on("request", (request) => {
      if (
        new URL(request.url()).pathname === crossPath &&
        request.method() === "GET"
      )
        crossDispatchClean = new URL(page.url()).search === "";
    });
    await page.goto(
      `${origin}/verificaciones/retorno?attempt_id=${encodeURIComponent(attemptID)}&status=Approved&session_token=local-synthetic-callback`,
      { waitUntil: "domcontentloaded" },
    );
    await page
      .getByRole("heading", { name: "Resultado de identidad" })
      .waitFor();
    result.crossAccountReturnStatus = (await crossWait).status();
    result.crossAccountReturnCleanBeforeReadback =
      crossDispatchClean && new URL(page.url()).search === "";
    await page
      .getByRole("alert")
      .filter({ hasText: "No pudimos consultar el estado" })
      .waitFor();
    await page
      .getByRole("button", { name: "Consultar estado nuevamente", exact: true })
      .waitFor();
    result.crossAccountReturnSafeError = true;
    const retryWait = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === crossPath &&
        response.request().method() === "GET",
    );
    await page
      .getByRole("button", { name: "Consultar estado nuevamente", exact: true })
      .click();
    result.crossAccountReturnRetryStatus = (await retryWait).status();
    await page
      .getByRole("alert")
      .filter({ hasText: "No pudimos consultar el estado" })
      .waitFor();
    const crossAccountReturnText = await page.locator("body").innerText();
    result.crossAccountReturnRetrySafe =
      result.crossAccountReturnRetryStatus === 404 &&
      (await page
        .getByText("Identidad verificada", { exact: true })
        .count()) === 0 &&
      (await page
        .getByText("Verificación rechazada", { exact: true })
        .count()) === 0 &&
      !identityMarkers.some((marker) =>
        crossAccountReturnText.includes(marker),
      );
    await page.goto(`${origin}/mi-cuenta/verificaciones`, {
      waitUntil: "domcontentloaded",
    });

    stage = "logout-b";
    result.secondLogoutStatus = await page.evaluate(
      async () =>
        (await fetch("/api/backend/auth/logout", { method: "POST" })).status,
    );
    if (result.secondLogoutStatus !== 204) throw new Error("logout-b");
    result.secondPostLogoutSessionStatus = await page.evaluate(
      async () =>
        (await fetch("/api/backend/auth/session", { cache: "no-store" }))
          .status,
    );
    if (result.secondPostLogoutSessionStatus !== 401)
      throw new Error("session-after-b-logout");
    loggedIn = false;

    stage = "login-a-return";
    await page.goto(`${origin}/acceso?return_to=%2Fmi-cuenta`, {
      waitUntil: "domcontentloaded",
    });
    await page.locator('input[name="email"]').fill(fixture.email);
    await page.locator('input[name="password"]').fill(fixture.password);
    const returnLoginResponse = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === "/api/backend/auth/login" &&
        r.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    result.returnLoginStatus = (await returnLoginResponse).status();
    loggedIn = true;
    if (result.returnLoginStatus !== 200) throw new Error("login-a-return");
    await page.waitForURL(`${origin}/mi-cuenta`);
    const returnReadback = await page.evaluate(async (expected) => {
      const response = await fetch(
        "/api/backend/verifications/identity-declaration",
        { cache: "no-store" },
      );
      let body = {};
      try {
        body = await response.json();
      } catch {}
      return {
        status: response.status,
        matches:
          body.first_name === expected.firstName &&
          body.last_name === expected.lastName &&
          body.date_of_birth === expected.dateOfBirth &&
          body.rut === expected.rut,
      };
    }, identity);
    result.returnReadbackStatus = returnReadback.status;
    result.returnReadbackMatches = returnReadback.matches;
    result.identityVariantReadbackVerified = returnReadback.matches;
    if (result.returnReadbackStatus !== 200 || !result.returnReadbackMatches)
      throw new Error("readback-a-return");
    const renderedReadback = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname ===
          "/api/backend/verifications/identity-declaration" &&
        r.request().method() === "GET" &&
        r.status() === 200,
    );
    await page.goto(`${origin}/mi-cuenta/verificaciones`, {
      waitUntil: "domcontentloaded",
    });
    await renderedReadback;
    await page
      .getByText(/Datos guardados \(revisión #/)
      .waitFor({ timeout: 15000 });
    result.returnReadbackRendered = await page.evaluate(
      (expected) =>
        document.body.innerText.includes(expected.firstName) &&
        document.body.innerText.includes(expected.lastName) &&
        document.body.innerText.includes(expected.displayRut),
      identity,
    );
    result.identityVariantReadbackVerified =
      result.identityVariantReadbackVerified && result.returnReadbackRendered;
    if (!result.returnReadbackRendered) throw new Error("readback-a-rendered");

    stage = "logout-a-final";
    result.finalLogoutStatus = await page.evaluate(
      async () =>
        (await fetch("/api/backend/auth/logout", { method: "POST" })).status,
    );
    if (result.finalLogoutStatus !== 204) throw new Error("logout-a-final");
    result.finalPostLogoutSessionStatus = await page.evaluate(
      async () =>
        (await fetch("/api/backend/auth/session", { cache: "no-store" }))
          .status,
    );
    if (result.finalPostLogoutSessionStatus !== 401)
      throw new Error("session-after-final-logout");
    result.noSessionCookiesAfterLogout = !(await context.cookies(origin)).some(
      (cookie) =>
        cookie.name === "__Host-espacigo-session" ||
        cookie.name === "__Host-espacigo-session-expiry",
    );
    if (!result.noSessionCookiesAfterLogout)
      throw new Error("session-cookie-after-final-logout");
    loggedIn = false;
  } finally {
    if (loggedIn) {
      try {
        if (new URL(page.url()).origin !== origin)
          await page.goto(`${origin}/mi-cuenta/verificaciones`, {
            waitUntil: "domcontentloaded",
          });
        const response = await page.evaluate(
          async () =>
            (await fetch("/api/backend/auth/logout", { method: "POST" }))
              .status,
        );
        result.finalLogoutStatus = response;
        result.finalPostLogoutSessionStatus = await page.evaluate(
          async () =>
            (await fetch("/api/backend/auth/session", { cache: "no-store" }))
              .status,
        );
        result.noSessionCookiesAfterLogout = !(
          await context.cookies(origin)
        ).some(
          (cookie) =>
            cookie.name === "__Host-espacigo-session" ||
            cookie.name === "__Host-espacigo-session-expiry",
        );
      } catch {}
    }
    await context.close();
    await browser.close();
  }
  console.log(JSON.stringify(result));
  const passed =
    result.anonymousStatus === 401 &&
    result.loginStatus === 200 &&
    result.sessionCookieFlagsValid &&
    result.declarationInitialStatus === 404 &&
    result.saveStatus === 201 &&
    result.readbackStatus === 200 &&
    result.reloadReadbackStatus === 200 &&
    result.noDiditConsentBlocked &&
    result.noDiditCallBeforeConsent &&
    result.diditIntentStatus === 201 &&
    result.diditIntentReadbackStatus === 200 &&
    result.diditIntentStatusValueAllowed &&
    result.logoutStatus === 204 &&
    result.postLogoutSessionStatus === 401 &&
    result.secondLoginStatus === 200 &&
    result.crossAccountDeclarationStatus === 404 &&
    !result.crossAccountContainsPII &&
    result.crossAccountUIEmpty &&
    result.secondLogoutStatus === 204 &&
    result.secondPostLogoutSessionStatus === 401 &&
    result.returnLoginStatus === 200 &&
    result.returnReadbackStatus === 200 &&
    result.returnReadbackMatches &&
    result.returnReadbackRendered &&
    result.ownReturnQueryCleaned &&
    result.ownReturnReadbackStatus === 200 &&
    result.ownReturnCleanBeforeReadback &&
    (webhookMode
      ? result.ownReturnReviewBadge
      : result.ownReturnPendingBadge) &&
    (!webhookMode ||
      (result.webhookInvalidSignatureStatus === 401 &&
        result.webhookStaleStatus === 401 &&
        result.webhookUnknownSessionStatus === 404 &&
        result.webhookValidStatus === 200 &&
        result.webhookValidProcessingStatus &&
        result.webhookReplayStatus === 200 &&
        result.webhookReplayProcessingStatus &&
        result.webhookAuthoritativeInReview)) &&
    result.ownReturnNoVerifiedBadge &&
    result.crossAccountReturnStatus === 404 &&
    result.crossAccountReturnCleanBeforeReadback &&
    result.crossAccountReturnSafeError &&
    result.crossAccountReturnRetryStatus === 404 &&
    result.crossAccountReturnRetrySafe &&
    result.finalLogoutStatus === 204 &&
    result.finalPostLogoutSessionStatus === 401 &&
    result.noSessionCookiesAfterLogout &&
    result.diditRedirectIntercepted &&
    result.diditRedirectHostAllowed &&
    result.diditRedirectPathValid &&
    result.unexpectedExternalRequests === 0 &&
    !result.piiInBrowserStorageOrUrl &&
    result.identityVariantReadbackVerified &&
    (process.env.KYC_NATIVE_CONFLICT !== "1" ||
      (result.rutConflictStatus === 409 &&
        result.rutConflictInputMatches &&
        result.rutConflictMessage &&
        result.rutConflictNoSavedSummary &&
        result.rutConflictNoDiditAction &&
        result.rutConflictReadbackStatus === 404 &&
        result.rutConflictReadbackEmpty &&
        result.rutConflictProviderUnchanged));
  const idempotencyPassed =
    !idempotencyMode ||
    (result.noProviderOnSave &&
      result.noProviderWithoutConsent &&
      result.providerCreatesAfterStart === 1 &&
      result.replayStatus === 201 &&
      result.replaySameAttempt &&
      result.replaySameRedirect &&
      result.providerCreatesAfterReplay === 1 &&
      result.activeResumeStatus === 201 &&
      result.activeResumeSameAttempt &&
      result.activeResumeSameRedirect &&
      result.providerCreatesAfterActiveResume === 1);
  if (!passed || !idempotencyPassed) process.exitCode = 1;
})().catch((error) => {
  console.log(JSON.stringify(result));
  const message = typeof error?.message === "string" ? error.message : "";
  const code = new Set([
    "attempt-id",
    "redirect-url",
    "didit-intent",
    "initial-declaration-state",
    "save-readback",
    "readback-a-rendered",
    "cross-account-ui-state",
  ]).has(message)
    ? message
    : /response body|ResponseBody|resource with given identifier|Protocol error/i.test(
          message,
        )
      ? "response-body-unavailable"
      : "unclassified";
  console.log(
    JSON.stringify({
      probe: "failed",
      stage,
      errorName: error?.name || "Error",
      errorCode: code,
    }),
  );
  process.exitCode = 1;
});
