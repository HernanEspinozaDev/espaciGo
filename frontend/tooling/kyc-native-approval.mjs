import fs from "node:fs";
import { createHmac, randomUUID } from "node:crypto";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const ORIGIN = "https://127.0.0.1:4219";
const DECLARATION = "/api/backend/verifications/identity-declaration";
const INTENT = "/api/backend/verifications/didit/intent";
const FIXTURE = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../.local/testing/kyc-native/fixture.json",
);
const mode = process.env.KYC_NATIVE_APPROVAL;

function assert(ok, stage) {
  if (!ok) throw Object.assign(new Error(), { stage });
}
function fixture() {
  const parent = fs.lstatSync(dirname(FIXTURE));
  const stat = fs.lstatSync(FIXTURE);
  assert(
    parent.isDirectory() &&
      !parent.isSymbolicLink() &&
      (parent.mode & 0o777) === 0o700 &&
      parent.uid === process.getuid(),
    "fixture-parent-policy",
  );
  assert(
    stat.isFile() &&
      !stat.isSymbolicLink() &&
      (stat.mode & 0o777) === 0o600 &&
      stat.uid === process.getuid(),
    "fixture-file-policy",
  );
  const v = JSON.parse(fs.readFileSync(FIXTURE, "utf8"));
  assert(
    typeof v.email === "string" &&
      v.email &&
      typeof v.password === "string" &&
      v.password,
    "fixture-shape",
  );
  return v;
}
async function providerStats() {
  const r = await fetch("http://127.0.0.1:19091/__local_test/stats", {
    headers: { "x-api-key": "local-kyc-test-api-key" },
    redirect: "error",
    signal: AbortSignal.timeout(5000),
  });
  assert(r.status === 200, "provider-stats-status");
  const b = await r.json();
  assert(
    b &&
      Number.isInteger(b.creation_requests) &&
      Number.isInteger(b.active_sessions) &&
      Object.keys(b).length === 2,
    "provider-stats-shape",
  );
  return b;
}
function signedWebhook(sessionID, type, status) {
  const timestamp = Math.floor(Date.now() / 1000);
  const bytes = Buffer.from(
    JSON.stringify({
      event_id: randomUUID(),
      webhook_type: type,
      timestamp,
      session_id: sessionID,
      status,
    }),
  );
  return {
    bytes,
    headers: {
      "Content-Type": "application/json",
      "X-Timestamp": String(timestamp),
      "X-Signature": createHmac("sha256", "local-kyc-test-webhook-secret")
        .update(bytes)
        .digest("hex"),
    },
  };
}
async function sendWebhook(payload) {
  return fetch("http://127.0.0.1:18090/api/v1/webhooks/didit", {
    method: "POST",
    redirect: "error",
    signal: AbortSignal.timeout(10000),
    headers: payload.headers,
    body: payload.bytes,
  });
}
async function main() {
  assert(mode === "supersession" || mode === "withdrawal", "mode-required");
  assert(process.argv.length === 2, "unsupported-arguments");
  const account = fixture();
  const report = {
    mode,
    assertions: {},
    stages: {},
    unexpectedExternalRequests: 0,
    declarationSaves: 0,
    intentCreates: 0,
    statusReads: 0,
    piiInStorageOrUrl: false,
    cookiesClearedAfterLogout: false,
  };
  let browser,
    context,
    page,
    loggedIn = false,
    stage = "launch",
    attemptID = "",
    sessionID = "",
    observedIntent;
  const mark = (name, value) => {
    report.assertions[name] = Boolean(value);
    assert(value, name);
  };
  try {
    browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
    context = await browser.newContext({
      ignoreHTTPSErrors: true,
      serviceWorkers: "block",
      viewport: { width: 1365, height: 900 },
    });
    page = await context.newPage();
    page.setDefaultTimeout(15000);
    page.setDefaultNavigationTimeout(15000);
    await context.route("**/*", async (route) => {
      const req = route.request(),
        u = new URL(req.url());
      if (
        u.origin === ORIGIN &&
        u.pathname === INTENT &&
        req.method() === "POST"
      ) {
        report.intentCreates++;
        const response = await route.fetch({ maxRedirects: 0 });
        observedIntent = {
          status: response.status(),
          body: await response.json(),
        };
        return route.fulfill({ response });
      }
      if (
        u.origin === ORIGIN &&
        u.pathname === DECLARATION &&
        req.method() === "POST"
      )
        report.declarationSaves++;
      if (
        u.origin === ORIGIN &&
        u.pathname.startsWith(`${INTENT}/`) &&
        req.method() === "GET"
      )
        report.statusReads++;
      if (u.origin === ORIGIN) return route.continue();
      if (
        u.protocol === "https:" &&
        ["verify.didit.me", "verification.didit.me"].includes(u.hostname) &&
        /^\/session\/[0-9a-f-]{36}$/i.test(u.pathname)
      )
        return route.fulfill({
          status: 200,
          contentType: "text/html; charset=utf-8",
          body: '<!doctype html><html lang="es"><main><h1 id="fake-provider">Proveedor Didit simulado localmente</h1></main></html>',
        });
      report.unexpectedExternalRequests++;
      return route.abort("blockedbyclient");
    });
    stage = "login";
    let response = await page.goto(`${ORIGIN}/acceso?return_to=%2Fmi-cuenta`, {
      waitUntil: "domcontentloaded",
    });
    assert(response?.status() === 200, stage);
    await page.locator('input[name="email"]').fill(account.email);
    await page.locator('input[name="password"]').fill(account.password);
    const loginWait = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === "/api/backend/auth/login" &&
        r.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    report.stages.loginStatus = (await loginWait).status();
    assert(report.stages.loginStatus === 200, stage);
    loggedIn = true;
    await page.waitForURL(`${ORIGIN}/mi-cuenta`);
    const cookies = await context.cookies(ORIGIN);
    mark(
      "secure_session_cookies",
      ["__Host-espacigo-session", "__Host-espacigo-session-expiry"].every((n) =>
        cookies.some(
          (c) =>
            c.name === n &&
            c.secure &&
            c.httpOnly &&
            c.sameSite === "Lax" &&
            c.path === "/",
        ),
      ),
    );

    const statsBefore = await providerStats();
    stage = "initial-declaration";
    let initial = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === DECLARATION &&
        r.request().method() === "GET",
    );
    await page.goto(`${ORIGIN}/mi-cuenta/verificaciones`, {
      waitUntil: "domcontentloaded",
    });
    await page
      .getByRole("heading", { name: /Verificación de usuario/ })
      .first()
      .waitFor();
    mark("initial_declaration_404", (await initial).status() === 404);
    const save = async (lastName) => {
      await page.locator("#id-first-name").fill("Synthetic");
      await page.locator("#id-last-name").fill(lastName);
      await page.locator("#id-dob").fill("1990-01-02");
      await page.locator("#id-rut").fill("11.222.333-9");
      await page.locator('input[name="consent"]').check();
      const post = page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname === DECLARATION &&
          r.request().method() === "POST",
      );
      const get = page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname === DECLARATION &&
          r.request().method() === "GET" &&
          r.status() === 200,
      );
      await page
        .getByRole("button", { name: "Guardar datos de identidad" })
        .click();
      const [p, g] = await Promise.all([post, get]);
      const body = await g.json();
      assert(
        p.status() === 201 &&
          g.status() === 200 &&
          body.first_name === "Synthetic" &&
          body.last_name === lastName &&
          body.date_of_birth === "1990-01-02" &&
          body.rut === "11222333-9",
        "declaration-save-readback",
      );
      await page.getByText(/Datos guardados \(revisión #/).waitFor();
    };
    await save("Person");
    mark("single_initial_declaration_save", report.declarationSaves === 1);
    const statsSaved = await providerStats();
    mark(
      "provider_unchanged_after_save",
      statsSaved.creation_requests === statsBefore.creation_requests &&
        statsSaved.active_sessions === 0,
    );

    stage = "intent-start";
    await page.locator('input[name="didit_consent"]').check();
    const intentWait = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === INTENT && r.request().method() === "POST",
    );
    await page
      .getByRole("button", { name: "Verificar identidad con Didit" })
      .click();
    response = await intentWait;
    assert(
      response.status() === 201 &&
        report.intentCreates === 1 &&
        observedIntent?.body?.id,
      stage,
    );
    attemptID = observedIntent.body.id;
    const redirect = new URL(observedIntent.body.redirect_url);
    assert(
      redirect.protocol === "https:" &&
        ["verify.didit.me", "verification.didit.me"].includes(
          redirect.hostname,
        ) &&
        /^\/session\/[0-9a-f-]{36}$/i.test(redirect.pathname),
      "validated-redirect",
    );
    sessionID = redirect.pathname.split("/").at(-1);
    await page.waitForURL((u) =>
      ["verify.didit.me", "verification.didit.me"].includes(u.hostname),
    );
    await page.locator("#fake-provider").waitFor();
    const statsStarted = await providerStats();
    mark(
      "one_provider_creation_after_start",
      statsStarted.creation_requests - statsBefore.creation_requests === 1 &&
        statsStarted.active_sessions === 1,
    );

    stage = "approved-webhook";
    let hook = signedWebhook(sessionID, "status.updated", "Approved");
    response = await sendWebhook(hook);
    let body = await response.json();
    report.stages.approvalWebhookStatus = response.status;
    report.stages.approvalProcessingStatus = [
      "PROCESSED",
      "IGNORED",
      "FAILED",
    ].includes(body.processing_status)
      ? body.processing_status
      : "UNEXPECTED";
    mark(
      "approval_webhook_processed",
      response.status === 200 && body.processing_status === "PROCESSED",
    );
    const statsApproved = await providerStats();
    mark(
      "approved_remote_session_deleted",
      statsApproved.active_sessions === 0,
    );
    const ownPath = `${INTENT}/${encodeURIComponent(attemptID)}`;
    const readAttempt = async () => {
      const result = await page.evaluate(async (path) => {
        const response = await fetch(path, {
          credentials: "same-origin",
          cache: "no-store",
        });
        return {
          status: response.status,
          body: response.status === 200 ? await response.json() : null,
        };
      }, ownPath);
      assert(result.status === 200, "attempt-read-status");
      return result.body;
    };
    const callbackRead = async () => {
      const wait = page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname === ownPath &&
          r.request().method() === "GET",
      );
      await page.goto(
        `${ORIGIN}/verificaciones/retorno?attempt_id=${encodeURIComponent(attemptID)}`,
        { waitUntil: "domcontentloaded" },
      );
      await page
        .getByRole("heading", { name: "Resultado de identidad" })
        .waitFor();
      const response = await wait;
      assert(response.status() === 200, "callback-attempt-status");
      return response.json();
    };
    // Return through the actual UI callback so the browser observes server state, not webhook claims.
    stage = "approved-return";
    let result = await callbackRead();
    mark(
      "approved_sandbox_not_verified_badge",
      result.status === "APPROVED" && result.badge_result === "IN_REVIEW",
    );
    await page.getByText("En revisión", { exact: true }).waitFor();
    mark(
      "sandbox_visible_without_live_badge",
      (await page
        .getByText("Identidad verificada", { exact: true })
        .count()) === 0,
    );
    mark(
      "approved_callback_query_cleaned",
      page.url() === `${ORIGIN}/verificaciones/retorno`,
    );
    if (mode === "supersession") {
      const gone = await page.evaluate(
        async (path) =>
          (await fetch(path, { credentials: "same-origin", cache: "no-store" }))
            .status,
        DECLARATION,
      );
      mark("first_declaration_purged_404", gone === 404);
      const freshGet = page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname === DECLARATION &&
          r.request().method() === "GET",
      );
      await page.goto(`${ORIGIN}/mi-cuenta/verificaciones`, {
        waitUntil: "domcontentloaded",
      });
      await page
        .getByRole("heading", { name: /Verificación de usuario/ })
        .first()
        .waitFor();

      await page.locator("#id-first-name").waitFor();
      mark("purged_declaration_still_404", (await freshGet).status() === 404);
      await save("Person Revision");
      mark(
        "second_declaration_save_and_readback",
        report.declarationSaves === 2,
      );
      mark("exactly_two_declaration_saves", report.declarationSaves === 2);
      const old = await readAttempt();
      mark(
        "old_approved_attempt_not_verified",
        old.status === "APPROVED" && old.badge_result === "NOT_VERIFIED",
      );
      hook = signedWebhook(sessionID, "status.updated", "Approved");
      response = await sendWebhook(hook);
      body = await response.json();
      mark(
        "late_approved_processed_without_badge_resurrection",
        response.status === 200 &&
          body.processing_status === "PROCESSED" &&
          (await readAttempt()).badge_result === "NOT_VERIFIED",
      );
      const afterLateCallback = await callbackRead();
      await page
        .getByText("Identidad no verificada", { exact: true })
        .waitFor();
      mark(
        "callback_shows_superseded_attempt_not_verified",
        afterLateCallback.status === "APPROVED" &&
          afterLateCallback.badge_result === "NOT_VERIFIED",
      );
      mark(
        "callback_not_verified_badge_visible",
        await page
          .getByText("Identidad no verificada", { exact: true })
          .isVisible(),
      );
    } else {
      hook = signedWebhook(sessionID, "user.status.updated", "REVOKED");
      response = await sendWebhook(hook);
      body = await response.json();
      mark(
        "revoked_processed",
        response.status === 200 && body.processing_status === "PROCESSED",
      );
      response = await sendWebhook(hook);
      body = await response.json();
      mark(
        "revoked_replay_deduplicated",
        response.status === 200 && body.processing_status === "PROCESSED",
      );
      hook = signedWebhook(sessionID, "status.updated", "Approved");
      response = await sendWebhook(hook);
      body = await response.json();
      mark(
        "late_approval_ignored",
        response.status === 200 && body.processing_status === "IGNORED",
      );
      result = await readAttempt();
      mark(
        "rejected_durable",
        result.status === "REJECTED" && result.badge_result === "REJECTED",
      );
      const durable = await callbackRead();
      await page.getByText("Verificación rechazada", { exact: true }).waitFor();
      mark(
        "rejected_survives_callback",
        durable.status === "REJECTED" && durable.badge_result === "REJECTED",
      );
      mark(
        "rejected_badge_visible",
        await page
          .getByText("Verificación rechazada", { exact: true })
          .isVisible(),
      );
      const gone = await page.evaluate(
        async (path) =>
          (await fetch(path, { credentials: "same-origin", cache: "no-store" }))
            .status,
        DECLARATION,
      );
      mark("approved_declaration_purged_404", gone === 404);
      const statsEnd = await providerStats();
      mark(
        "withdrawal_remote_session_inactive",
        statsEnd.active_sessions === 0,
      );
    }
    const privateState = await page.evaluate(
      (markers) => {
        const vals = [
          ...Object.keys(localStorage).map((k) => localStorage.getItem(k)),
          ...Object.keys(sessionStorage).map((k) => sessionStorage.getItem(k)),
        ].join(" ");
        return markers.some(
          (v) => vals.includes(v) || location.href.includes(v),
        );
      },
      [
        "Synthetic",
        "Person",
        "Person Revision",
        "1990-01-02",
        "11222333-9",
        "11.222.333-9",
      ],
    );
    report.piiInStorageOrUrl = privateState;
    mark("no_pii_in_storage_or_url", !privateState);
    mark(
      "exact_save_and_intent_counts",
      report.declarationSaves === (mode === "supersession" ? 2 : 1) &&
        report.intentCreates === 1,
    );
    mark(
      "no_unexpected_external_requests",
      report.unexpectedExternalRequests === 0,
    );
  } catch (error) {
    report.failureStage = error?.stage ?? stage;
    report.errorName = error?.name ?? "Error";
  } finally {
    if (page && loggedIn && new URL(page.url()).origin !== ORIGIN) {
      try {
        await page.goto(`${ORIGIN}/mi-cuenta`, {
          waitUntil: "domcontentloaded",
        });
      } catch {
        report.failureStage ??= "logout-origin-recovery";
      }
    }
    if (page && loggedIn)
      try {
        report.stages.logoutStatus = await page.evaluate(
          async () =>
            (await fetch("/api/backend/auth/logout", { method: "POST" }))
              .status,
        );
        report.stages.postLogoutSessionStatus = await page.evaluate(
          async () =>
            (await fetch("/api/backend/auth/session", { cache: "no-store" }))
              .status,
        );
        report.cookiesClearedAfterLogout =
          (await context.cookies(ORIGIN)).filter((c) =>
            c.name.startsWith("__Host-espacigo-session"),
          ).length === 0;
        mark(
          "logout_clears_session",
          report.stages.logoutStatus === 204 &&
            report.stages.postLogoutSessionStatus === 401 &&
            report.cookiesClearedAfterLogout,
        );
      } catch {
        report.assertions.logout_clears_session = false;
        report.failureStage ??= "logout-verification";
      }
    if (browser)
      await browser.close().catch(() => {
        report.failureStage ??= "browser-close";
      });
  }
  console.log(JSON.stringify(report));
  if (report.failureStage) process.exitCode = 1;
}
if (
  process.env.KYC_NATIVE_APPROVAL !== "supersession" &&
  process.env.KYC_NATIVE_APPROVAL !== "withdrawal"
) {
  console.log(
    JSON.stringify({ failureStage: "mode-required", assertions: {} }),
  );
  process.exitCode = 2;
} else
  main().catch((error) => {
    console.log(
      JSON.stringify({
        failureStage: error?.stage ?? "launch",
        errorName: error?.name ?? "Error",
        assertions: {},
      }),
    );
    process.exitCode = 1;
  });
