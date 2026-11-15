import fs from "node:fs";
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

function assert(condition, stage) {
  if (!condition) throw Object.assign(new Error(), { stage });
}

function fixture() {
  const parentPath = dirname(FIXTURE);
  const parent = fs.lstatSync(parentPath);
  assert(
    parent.isDirectory() &&
      !parent.isSymbolicLink() &&
      (parent.mode & 0o777) === 0o700 &&
      parent.uid === process.getuid(),
    "fixture-parent-policy",
  );
  const stat = fs.lstatSync(FIXTURE);
  assert(
    stat.isFile() &&
      !stat.isSymbolicLink() &&
      (stat.mode & 0o777) === 0o600 &&
      stat.uid === process.getuid(),
    "fixture-file-policy",
  );
  const value = JSON.parse(fs.readFileSync(FIXTURE, "utf8"));
  assert(
    typeof value.email === "string" &&
      value.email.length > 0 &&
      typeof value.password === "string" &&
      value.password.length > 0,
    "fixture-shape",
  );
  return value;
}

async function providerCreations() {
  const response = await fetch("http://127.0.0.1:19091/__local_test/stats", {
    method: "GET",
    redirect: "error",
    signal: AbortSignal.timeout(5000),
    headers: { "x-api-key": "local-kyc-test-api-key" },
  });
  assert(response.status === 200, "provider-stats-status");
  const body = await response.json();
  assert(
    body &&
      !Array.isArray(body) &&
      Number.isInteger(body.creation_requests) &&
      body.creation_requests >= 0 &&
      Number.isInteger(body.active_sessions) &&
      body.active_sessions >= 0 &&
      Object.keys(body).length === 2,
    "provider-stats-shape",
  );
  return body.creation_requests;
}

async function main() {
  assert(process.env.KYC_NATIVE_RETURN_FAILURES === "1", "opt-in-required");
  assert(process.argv.slice(2).length === 0, "unsupported-arguments");
  const account = fixture();
  const output = {
    stages: {},
    providerCreationsAfterSave: -1,
    providerCreationsAfterStart: -1,
    providerCreationsAfterRecovery: -1,
    unexpectedExternalRequests: 0,
    ownStatusReads: 0,
    piiInStorageOrUrl: false,
    cookiesClearedAfterLogout: false,
  };
  let browser;
  let context;
  let page;
  let loggedIn = false;
  let currentStage = "launch";
  let attemptID = "";
  let providerBaseline = -1;
  let declarationSaves = 0;
  let intentCreates = 0;
  let dispatchClean = false;
  let observedIntent = null;
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
      const request = route.request();
      const url = new URL(request.url());
      if (
        url.origin === ORIGIN &&
        url.pathname === INTENT &&
        request.method() === "POST"
      ) {
        intentCreates += 1;
        const response = await route.fetch({ maxRedirects: 0 });
        observedIntent = {
          status: response.status(),
          body: await response.json(),
        };
        return route.fulfill({ response });
      }
      if (
        url.origin === ORIGIN &&
        url.pathname === DECLARATION &&
        request.method() === "POST"
      )
        declarationSaves += 1;
      if (
        url.origin === ORIGIN &&
        url.pathname.startsWith(`${INTENT}/`) &&
        request.method() === "GET"
      )
        output.ownStatusReads += 1;
      if (url.origin === ORIGIN) return route.continue();
      if (url.protocol === "blob:" && url.origin === ORIGIN)
        return route.continue();
      if (
        url.protocol === "https:" &&
        ["verify.didit.me", "verification.didit.me"].includes(url.hostname) &&
        /^\/session\/[0-9a-f-]{36}$/i.test(url.pathname)
      ) {
        return route.fulfill({
          status: 200,
          contentType: "text/html; charset=utf-8",
          body: '<!doctype html><html lang="es"><main><h1 id="fake-provider">Proveedor Didit simulado localmente</h1></main></html>',
        });
      }
      output.unexpectedExternalRequests += 1;
      return route.abort("blockedbyclient");
    });

    currentStage = "login";
    let response = await page.goto(`${ORIGIN}/acceso?return_to=%2Fmi-cuenta`, {
      waitUntil: "domcontentloaded",
    });
    assert(response?.status() === 200, currentStage);
    await page.locator('input[name="email"]').fill(account.email);
    await page.locator('input[name="password"]').fill(account.password);
    const loginWait = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === "/api/backend/auth/login" &&
        r.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    output.stages.loginStatus = (await loginWait).status();
    assert(output.stages.loginStatus === 200, currentStage);
    loggedIn = true;
    await page.waitForURL(`${ORIGIN}/mi-cuenta`, {
      waitUntil: "domcontentloaded",
    });
    const cookies = await context.cookies(ORIGIN);
    assert(
      cookies.some(
        (c) =>
          c.name === "__Host-espacigo-session" &&
          c.secure &&
          c.httpOnly &&
          c.sameSite === "Lax" &&
          c.path === "/",
      ),
      "session-cookie",
    );
    assert(
      cookies.some(
        (c) =>
          c.name === "__Host-espacigo-session-expiry" &&
          c.secure &&
          c.httpOnly &&
          c.sameSite === "Lax" &&
          c.path === "/",
      ),
      "expiry-cookie",
    );

    currentStage = "identity-save";
    providerBaseline = await providerCreations();
    const initial = page.waitForResponse(
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
    assert((await initial).status() === 404, "identity-initial-state");
    await page.locator("#id-first-name").fill("Synthetic");
    await page.locator("#id-last-name").fill("Person");
    await page.locator("#id-dob").fill("1990-01-02");
    await page.locator("#id-rut").fill("11.222.333-9");
    await page.locator('input[name="consent"]').check();
    const saveWait = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === DECLARATION &&
        r.request().method() === "POST",
    );
    const saveRead = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === DECLARATION &&
        r.request().method() === "GET" &&
        r.status() === 200,
    );
    await page
      .getByRole("button", { name: "Guardar datos de identidad" })
      .click();
    output.stages.identitySaveStatus = (await saveWait).status();
    output.stages.identityReadbackStatus = (await saveRead).status();
    assert(
      output.stages.identitySaveStatus === 201 &&
        output.stages.identityReadbackStatus === 200 &&
        declarationSaves === 1,
      currentStage,
    );
    await page.getByText(/Datos guardados \(revisión #/).waitFor();
    output.providerCreationsAfterSave =
      (await providerCreations()) - providerBaseline;
    assert(output.providerCreationsAfterSave === 0, "provider-created-on-save");

    currentStage = "didit-start";
    await page.locator('input[name="didit_consent"]').check();
    let intentResponse;
    const intentWait = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === INTENT && r.request().method() === "POST",
    );
    await page
      .getByRole("button", { name: "Verificar identidad con Didit" })
      .click();
    intentResponse = await intentWait;
    output.stages.diditIntentStatus = intentResponse.status();
    assert(
      output.stages.diditIntentStatus === 201 && intentCreates === 1,
      currentStage,
    );
    const intentBody = observedIntent?.body;
    assert(
      intentBody &&
        typeof intentBody.id === "string" &&
        /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
          intentBody.id,
        ),
      "intent-shape",
    );
    attemptID = intentBody.id;
    await page.waitForURL(
      (url) =>
        ["verify.didit.me", "verification.didit.me"].includes(url.hostname),
      { timeout: 15000 },
    );
    await page.locator("#fake-provider").waitFor();
    output.providerCreationsAfterStart =
      (await providerCreations()) - providerBaseline;
    assert(output.providerCreationsAfterStart === 1, "provider-create-count");

    currentStage = "return-first-failure";
    const ownPath = `${INTENT}/${encodeURIComponent(attemptID)}`;
    page.on("request", (request) => {
      const url = new URL(request.url());
      if (
        url.origin === ORIGIN &&
        url.pathname === ownPath &&
        request.method() === "GET"
      )
        dispatchClean = new URL(page.url()).search === "";
    });
    const failedRead = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === ownPath && r.request().method() === "GET",
    );
    await page.goto(
      `${ORIGIN}/verificaciones/retorno?attempt_id=${encodeURIComponent(attemptID)}&status=Approved&session_token=synthetic-forgery`,
      { waitUntil: "domcontentloaded" },
    );
    await page
      .getByRole("heading", { name: "Resultado de identidad" })
      .waitFor();
    output.stages.firstStatusRead = (await failedRead).status();
    assert(
      output.stages.firstStatusRead === 502 &&
        dispatchClean &&
        new URL(page.url()).search === "",
      currentStage,
    );
    await page
      .getByRole("alert")
      .filter({ hasText: "No pudimos consultar el estado" })
      .waitFor();
    assert(
      (await page
        .getByText("Identidad verificada", { exact: true })
        .count()) === 0 &&
        (await page
          .getByText("Verificación rechazada", { exact: true })
          .count()) === 0,
      "failure-badge",
    );
    await page.waitForTimeout(2500);
    assert(output.ownStatusReads === 1, "automatic-retry");

    currentStage = "explicit-retry-failed-state";
    const retryRead = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === ownPath && r.request().method() === "GET",
    );
    await page
      .getByRole("button", { name: "Consultar estado nuevamente", exact: true })
      .click();
    const retryResponse = await retryRead;
    output.stages.retryStatus = retryResponse.status();
    const failedBody = await retryResponse.json().catch(() => null);
    assert(
      output.stages.retryStatus === 200 &&
        failedBody?.status === "FAILED" &&
        failedBody?.badge_result === "NOT_VERIFIED",
      currentStage,
    );
    await page
      .getByText("Fallo técnico en la comprobación", { exact: true })
      .waitFor();
    await page.getByText(/no implica un rechazo de tu identidad/i).waitFor();
    assert(
      (await page
        .getByText("Identidad verificada", { exact: true })
        .count()) === 0 &&
        (await page
          .getByText("Verificación rechazada", { exact: true })
          .count()) === 0 &&
        !new URL(page.url()).search &&
        (await page
          .getByRole("alert")
          .filter({ hasText: "No pudimos consultar el estado" })
          .count()) === 0,
      "failed-terminal-ui",
    );

    currentStage = "durable-failed-reload";
    const durableRead = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === ownPath && r.request().method() === "GET",
    );
    await page.goto(
      `${ORIGIN}/verificaciones/retorno?attempt_id=${encodeURIComponent(attemptID)}`,
      { waitUntil: "domcontentloaded" },
    );
    const durableResponse = await durableRead;
    output.stages.reloadStatus = durableResponse.status();
    const durableBody = await durableResponse.json();
    assert(
      output.stages.reloadStatus === 200 &&
        durableBody.status === "FAILED" &&
        durableBody.badge_result === "NOT_VERIFIED",
      currentStage,
    );
    await page
      .getByText("Fallo técnico en la comprobación", { exact: true })
      .waitFor();
    const privateState = await page.evaluate(
      (markers) => {
        const values = [
          ...Object.keys(localStorage).map((k) => localStorage.getItem(k)),
          ...Object.keys(sessionStorage).map((k) => sessionStorage.getItem(k)),
        ].join(" ");
        return (
          markers.some((v) => values.includes(v)) ||
          markers.some((v) => location.href.includes(v))
        );
      },
      ["Synthetic", "Person", "1990-01-02", "11222333-9", "11.222.333-9"],
    );
    output.piiInStorageOrUrl = privateState;
    output.providerCreationsAfterRecovery =
      (await providerCreations()) - providerBaseline;
    assert(
      !privateState &&
        intentCreates === 1 &&
        declarationSaves === 1 &&
        output.providerCreationsAfterRecovery === 1 &&
        output.unexpectedExternalRequests === 0 &&
        output.ownStatusReads === 3 &&
        dispatchClean &&
        new URL(page.url()).search === "",
      "privacy-or-mutation-count",
    );
  } catch (error) {
    output.failureStage = error?.stage ?? currentStage;
    output.errorName = error?.name ?? "Error";
  } finally {
    if (page && loggedIn) {
      try {
        const logout = await page.evaluate(
          async () =>
            (await fetch("/api/backend/auth/logout", { method: "POST" }))
              .status,
        );
        output.stages.logoutStatus = logout;
        const session = await page.evaluate(
          async () =>
            (await fetch("/api/backend/auth/session", { cache: "no-store" }))
              .status,
        );
        output.stages.postLogoutSessionStatus = session;
        output.cookiesClearedAfterLogout =
          (await context.cookies(ORIGIN)).filter((c) =>
            c.name.startsWith("__Host-espacigo-session"),
          ).length === 0;
        if (
          logout !== 204 ||
          session !== 401 ||
          !output.cookiesClearedAfterLogout
        )
          output.failureStage ??= "logout-verification";
      } catch {
        output.failureStage ??= "logout-verification";
      }
    }
    if (browser)
      await browser.close().catch(() => {
        output.failureStage ??= "browser-close";
      });
  }
  console.log(JSON.stringify(output));
  if (output.failureStage) process.exitCode = 1;
}

main().catch((error) => {
  console.log(
    JSON.stringify({
      failureStage: error?.stage ?? "launch",
      errorName: error?.name ?? "Error",
    }),
  );
  process.exitCode = 1;
});
