import assert from "node:assert/strict";
import http from "node:http";
import net from "node:net";
import fs from "node:fs";
import path from "node:path";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";
import { startSessionTlsProxy } from "./bff-test-servers.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendDir = path.resolve(__dirname, "..");
const standaloneDir = path.join(frontendDir, ".next/standalone");
const standaloneServer = path.join(standaloneDir, "server.js");

if (!fs.existsSync(standaloneServer)) {
  console.error("FAIL: phase=STANDALONE_CHECK error=StandaloneNotFound");
  process.exit(1);
}

let currentPhase = "INIT";

async function stopChild(child) {
  if (!child || child.killed || child.exitCode !== null) return;
  await new Promise((resolve) => {
    let done = false;
    const onExit = () => {
      if (!done) {
        done = true;
        clearTimeout(killTimer);
        resolve();
      }
    };
    child.once("exit", onExit);
    child.kill("SIGTERM");
    const killTimer = setTimeout(() => {
      if (!done) {
        try {
          child.kill("SIGKILL");
        } catch {}
        resolve();
      }
    }, 1000);
  });
}

async function main() {
  let nextChild = null;
  let edge = null;
  let browser = null;
  let context = null;
  let page = null;

  try {
    currentPhase = "PREFLIGHT";
    const backendUrl = process.env.API_BACKEND_BASE_URL;
    const driverUrl = process.env.TEST_DRIVER_URL;
    const certFile = process.env.SESSION_TLS_CERT;
    const keyFile = process.env.SESSION_TLS_KEY;
    const nextPort = parseInt(process.env.NEXT_PORT || "4349", 10);

    assert.ok(backendUrl, "API_BACKEND_BASE_URL required");
    assert.ok(driverUrl, "TEST_DRIVER_URL required");
    assert.ok(
      certFile && fs.existsSync(certFile),
      "SESSION_TLS_CERT required and must exist",
    );
    assert.ok(
      keyFile && fs.existsSync(keyFile),
      "SESSION_TLS_KEY required and must exist",
    );
    assert.ok(Number.isInteger(nextPort) && nextPort > 0, "invalid NEXT_PORT");

    const parsedBackend = new URL(backendUrl);
    const parsedDriver = new URL(driverUrl);
    assert.equal(parsedBackend.protocol, "http:");
    assert.equal(parsedBackend.hostname, "127.0.0.1");
    assert.equal(parsedDriver.protocol, "http:");
    assert.equal(parsedDriver.hostname, "127.0.0.1");

    await new Promise((resolve, reject) => {
      const tester = net.createServer();
      tester.once("error", reject);
      tester.listen(nextPort, "127.0.0.1", () => {
        tester.close(resolve);
      });
    });

    currentPhase = "ASSET_SYNC";
    if (fs.existsSync(path.join(frontendDir, "public"))) {
      fs.cpSync(
        path.join(frontendDir, "public"),
        path.join(standaloneDir, "public"),
        {
          recursive: true,
        },
      );
    }
    if (fs.existsSync(path.join(frontendDir, ".next/static"))) {
      fs.cpSync(
        path.join(frontendDir, ".next/static"),
        path.join(standaloneDir, ".next/static"),
        { recursive: true },
      );
    }

    currentPhase = "SPAWN_NEXT";
    nextChild = spawn(process.execPath, [standaloneServer], {
      cwd: frontendDir,
      env: {
        ...process.env,
        NODE_ENV: "production",
        HOSTNAME: "127.0.0.1",
        PORT: String(nextPort),
        API_BACKEND_BASE_URL: backendUrl,
        NEXT_TELEMETRY_DISABLED: "1",
      },
      stdio: ["ignore", "pipe", "pipe"],
    });

    let stderrBounded = "";
    nextChild.stderr.on("data", (chunk) => {
      if (stderrBounded.length < 65536) {
        stderrBounded += chunk.toString("utf8");
      }
    });
    nextChild.stdout.resume();

    await new Promise((resolve, reject) => {
      let done = false;
      const onExit = (code, signal) => {
        if (!done) {
          done = true;
          clearInterval(timer);
          reject(new Error(`NextExitedEarly_${code}_${signal}`));
        }
      };
      nextChild.once("exit", onExit);

      const start = Date.now();
      const checkHealth = () => {
        const req = http.get(`http://127.0.0.1:${nextPort}/`, (res) => {
          res.resume();
          if (!done) {
            done = true;
            clearInterval(timer);
            nextChild.removeListener("exit", onExit);
            resolve();
          }
        });
        req.on("error", () => {
          if (Date.now() - start > 20000 && !done) {
            done = true;
            clearInterval(timer);
            nextChild.removeListener("exit", onExit);
            reject(new Error("NextStartupTimeout"));
          }
        });
      };
      const timer = setInterval(checkHealth, 250);
      checkHealth();
    });

    currentPhase = "TLS_PROXY";
    edge = await startSessionTlsProxy({
      targetPort: nextPort,
      certFile,
      keyFile,
    });
    const proxyUrl = edge.url;

    currentPhase = "BROWSER_LAUNCH";
    browser = await chromium.launch({
      headless: true,
    });
    context = await browser.newContext({
      ignoreHTTPSErrors: true,
      baseURL: proxyUrl,
    });
    page = await context.newPage();

    const runId = Math.random().toString(36).slice(2, 10);
    const testEmail = `verify-${runId}@ejemplo.invalid`;
    const testPassword = "Synthetic#123";
    const newPassword1 = "Nueva#456_pass";
    const newPassword2 = "Tercera#789_pass";

    // Phase 1: Register
    currentPhase = "REGISTER";
    await page.goto("/registro", { waitUntil: "networkidle" });
    await page.waitForSelector("#terms-and-conditions-consent", {
      timeout: 10000,
    });
    await page.locator('input[name="email"]').fill(testEmail);
    await page
      .locator(
        'input[name="register-password"], input[name="password"], input[type="password"]',
      )
      .first()
      .fill(testPassword);
    await page.locator("#terms-and-conditions-consent").check();
    await page.locator("#privacy-policy-consent").check();

    const [registerRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          res.url().includes("/auth/register") &&
          res.status() === 201,
      ),
      page.locator('button[type="submit"]').click(),
    ]);
    assert.equal(registerRes.status(), 201);
    await page.waitForURL("**/registro-exitoso*", { timeout: 10000 });

    // Phase 2: Login before verify (403 email unverified)
    currentPhase = "LOGIN_BLOCKED";
    await page.goto("/acceso", { waitUntil: "networkidle" });
    await page.locator('input[name="email"]').fill(testEmail);
    await page
      .locator('input[name="password"], input[type="password"]')
      .first()
      .fill(testPassword);

    const [loginFailRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          res.url().includes("/auth/login") &&
          res.status() === 403,
      ),
      page.getByRole("button", { name: "Iniciar sesión" }).click(),
    ]);
    assert.equal(loginFailRes.status(), 403);
    await page.waitForSelector(
      "text=Debes verificar tu correo antes de ingresar",
      { timeout: 10000 },
    );

    const cookiesPreVerify = await context.cookies();
    assert.equal(
      cookiesPreVerify.find((c) => c.name === "__Host-espacigo-session"),
      undefined,
    );
    assert.equal(
      cookiesPreVerify.find((c) => c.name === "__Host-espacigo-session-expiry"),
      undefined,
    );

    // Phase 3: Driver delivery memory fetch
    currentPhase = "MAIL_FETCH";
    const mailRes = await fetch(`${driverUrl}/__test/verification-mail`);
    assert.equal(mailRes.status, 200);
    const mailData = await mailRes.json();
    const tokenId = mailData.token_id;
    const token = mailData.token;
    assert.ok(tokenId && token);

    // Phase 4: Verify email via fragment URL
    currentPhase = "VERIFY";
    await page.goto(
      `/verificar-correo#token_id=${encodeURIComponent(tokenId)}&token=${encodeURIComponent(token)}`,
      { waitUntil: "networkidle" },
    );
    await page.waitForFunction(() => window.location.hash === "", {
      timeout: 10000,
    });

    const [verifyRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          new URL(res.url()).pathname === "/api/backend/auth/verification" &&
          res.status() === 204,
      ),
      page.getByRole("button", { name: "Verificar mi correo" }).click(),
    ]);
    assert.equal(verifyRes.status(), 204);
    await page.waitForURL("**/correo-verificado*", { timeout: 10000 });
    await page.waitForSelector("text=Entrar a EspaciGo", { timeout: 10000 });

    const cookiesPostVerify = await context.cookies();
    assert.equal(
      cookiesPostVerify.find((c) => c.name === "__Host-espacigo-session"),
      undefined,
    );
    assert.equal(
      cookiesPostVerify.find(
        (c) => c.name === "__Host-espacigo-session-expiry",
      ),
      undefined,
    );

    // Phase 5: Reopen old verification link -> status used, H1 'Verificación ya realizada', no 'Verificar mi correo' button
    currentPhase = "REOPEN_USED_VERIFY_NAVIGATE";
    await page.goto(
      `/verificar-correo#token_id=${encodeURIComponent(tokenId)}&token=${encodeURIComponent(token)}`,
      // Result readiness is its Go-confirmed heading, not unrelated link prefetch.
      { waitUntil: "domcontentloaded" },
    );
    currentPhase = "REOPEN_USED_VERIFY_HEADING";
    await page.waitForSelector("h1:has-text('Verificación ya realizada')", {
      timeout: 10000,
    });
    const verifyBtnCount = await page
      .getByRole("button", { name: "Verificar mi correo" })
      .count();
    assert.equal(
      verifyBtnCount,
      0,
      "No 'Verificar mi correo' button should be shown when link is used",
    );

    // Negative direct HTTP POST consumed verify via BFF to 422 with Origin header
    currentPhase = "REPLAY_VERIFY_DIRECT";
    const replayVerifyRes = await context.request.post(
      "/api/backend/auth/verification",
      {
        timeout: 5000,
        headers: {
          Origin: proxyUrl,
        },
        data: {
          token_id: tokenId,
          token: token,
        },
      },
    );
    assert.equal(
      replayVerifyRes.status(),
      422,
      "Replay verification directly to BFF must yield 422",
    );

    // Phase 6: Login after verify (200 OK + cookies issued)
    currentPhase = "LOGIN_SUCCESS";
    await page.goto("/acceso", { waitUntil: "networkidle" });
    await page.locator('input[name="email"]').fill(testEmail);
    await page
      .locator('input[name="password"], input[type="password"]')
      .first()
      .fill(testPassword);

    const [loginSuccessRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          res.url().includes("/auth/login") &&
          res.status() === 200,
      ),
      page.getByRole("button", { name: "Iniciar sesión" }).click(),
    ]);
    assert.equal(loginSuccessRes.status(), 200);
    await page.waitForURL("**/mi-cuenta*", { timeout: 10000 });

    const cookiesPostLogin = await context.cookies();
    const sessionCookie = cookiesPostLogin.find(
      (c) => c.name === "__Host-espacigo-session",
    );
    const expiryCookie = cookiesPostLogin.find(
      (c) => c.name === "__Host-espacigo-session-expiry",
    );
    assert.ok(sessionCookie && sessionCookie.value.length > 0);
    assert.ok(expiryCookie && expiryCookie.value.length > 0);
    assert.equal(sessionCookie.secure, true);
    assert.equal(sessionCookie.httpOnly, true);
    assert.equal(sessionCookie.sameSite, "Lax");
    assert.equal(expiryCookie.secure, true);
    assert.equal(expiryCookie.httpOnly, true);

    // Phase 7: Reload session verification
    currentPhase = "SESSION_RELOAD";
    const [sessionRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "GET" &&
          res.url().includes("/auth/session") &&
          res.status() === 200,
      ),
      page.reload({ waitUntil: "networkidle" }),
    ]);
    assert.equal(sessionRes.status(), 200);

    // Phase 8: Logout and cookie invalidation
    currentPhase = "LOGOUT";
    const [logoutRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          res.url().includes("/auth/logout") &&
          res.status() === 204,
      ),
      page.getByRole("button", { name: "Cerrar sesión" }).click(),
    ]);
    assert.equal(logoutRes.status(), 204);
    await page.waitForURL("**/acceso*", { timeout: 10000 });

    const cookiesPostLogout = await context.cookies();
    const expiredSessionCookie = cookiesPostLogout.find(
      (c) => c.name === "__Host-espacigo-session",
    );
    assert.ok(!expiredSessionCookie || expiredSessionCookie.value === "");

    // Phase 9: Retrieval Recovery via UI /recuperar-acceso
    currentPhase = "RECOVERY_REQUEST";
    await page.goto("/recuperar-acceso", { waitUntil: "networkidle" });
    await page.locator('#recovery-email, input[type="email"]').fill(testEmail);

    const [recoveryReqRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          new URL(res.url()).pathname ===
            "/api/backend/auth/password/recovery" &&
          res.status() === 202,
      ),
      page.getByRole("button", { name: "Solicitar instrucciones" }).click(),
    ]);
    assert.equal(recoveryReqRes.status(), 202);
    await page.waitForSelector("text=Solicitud recibida", { timeout: 10000 });

    // Phase 10: Private driver returns RecoveryDelivery
    currentPhase = "RECOVERY_MAIL_FETCH";
    const recoveryMailRes = await fetch(`${driverUrl}/__test/recovery-mail`);
    assert.equal(recoveryMailRes.status, 200);
    const recoveryMailData = await recoveryMailRes.json();
    const recTokenId = recoveryMailData.token_id;
    const recToken = recoveryMailData.token;
    assert.ok(recTokenId && recToken);

    // Phase 11: Navigate to canonical fragment recovery link -> form -> 204 -> Contraseña actualizada
    currentPhase = "RECOVERY_CONSUME";
    await page.goto(
      `/recuperar-acceso#token_id=${encodeURIComponent(recTokenId)}&token=${encodeURIComponent(recToken)}`,
      { waitUntil: "networkidle" },
    );

    await page.waitForSelector("label:has-text('Nueva contraseña')", {
      timeout: 10000,
    });
    await page.waitForSelector("label:has-text('Confirmar nueva contraseña')", {
      timeout: 10000,
    });

    await page.locator("#recovery-new-password").fill(newPassword1);
    await page.locator("#recovery-confirm-password").fill(newPassword1);

    const [recoveryConsumeRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          new URL(res.url()).pathname ===
            "/api/backend/auth/password/recovery/consume" &&
          res.status() === 204,
      ),
      page.getByRole("button", { name: "Actualizar contraseña" }).click(),
    ]);
    assert.equal(recoveryConsumeRes.status(), 204);
    await page.waitForSelector("h1:has-text('Contraseña actualizada')", {
      timeout: 10000,
    });

    // Phase 12: Reopen SAME recovery link -> inspection used, H1 'Enlace de recuperación ya utilizado', no fields, no button
    currentPhase = "REOPEN_USED_RECOVERY";
    await page.goto(
      `/recuperar-acceso#token_id=${encodeURIComponent(recTokenId)}&token=${encodeURIComponent(recToken)}`,
      { waitUntil: "domcontentloaded" },
    );
    await page.waitForSelector(
      "h1:has-text('Enlace de recuperación ya utilizado')",
      { timeout: 10000 },
    );
    assert.equal(await page.locator("#recovery-new-password").count(), 0);
    assert.equal(await page.locator("#recovery-confirm-password").count(), 0);
    assert.equal(
      await page.getByRole("button", { name: "Actualizar contraseña" }).count(),
      0,
    );

    // Phase 13: Direct BFF POST consume used token with different strong password -> 422 with Origin header
    currentPhase = "REPLAY_RECOVERY_DIRECT";
    const replayRecoveryRes = await context.request.post(
      "/api/backend/auth/password/recovery/consume",
      {
        timeout: 5000,
        headers: {
          Origin: proxyUrl,
        },
        data: {
          token_id: recTokenId,
          token: recToken,
          new_password: newPassword2,
          confirm_password: newPassword2,
        },
      },
    );
    assert.equal(
      replayRecoveryRes.status(),
      422,
      "Replay recovery consume directly to BFF must yield 422",
    );

    // Phase 14: Disposable attempt with old password fails (401), login with newPassword1 succeeds (200)
    currentPhase = "VERIFY_NEW_PASSWORD";
    await page.goto("/acceso", { waitUntil: "networkidle" });
    await page.locator('input[name="email"]').fill(testEmail);
    await page
      .locator('input[name="password"], input[type="password"]')
      .first()
      .fill(testPassword);

    const [oldPwdFailRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          res.url().includes("/auth/login") &&
          res.status() === 401,
      ),
      page.getByRole("button", { name: "Iniciar sesión" }).click(),
    ]);
    assert.equal(
      oldPwdFailRes.status(),
      401,
      "Old password must be rejected with 401",
    );

    await page
      .locator('input[name="password"], input[type="password"]')
      .first()
      .fill(newPassword1);

    const [newPwdLoginRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          res.url().includes("/auth/login") &&
          res.status() === 200,
      ),
      page.getByRole("button", { name: "Iniciar sesión" }).click(),
    ]);
    assert.equal(
      newPwdLoginRes.status(),
      200,
      "Login with updated password must succeed",
    );
    await page.waitForURL("**/mi-cuenta*", { timeout: 10000 });

    const recoveryLoginCookies = await context.cookies();
    const recoverySessionCookie = recoveryLoginCookies.find(
      (c) => c.name === "__Host-espacigo-session",
    );
    assert.ok(recoverySessionCookie && recoverySessionCookie.value.length > 0);

    // Session reload survives
    const [recoveryReloadRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "GET" &&
          res.url().includes("/auth/session") &&
          res.status() === 200,
      ),
      page.reload({ waitUntil: "networkidle" }),
    ]);
    assert.equal(recoveryReloadRes.status(), 200);

    // Final logout
    const [finalLogoutRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          res.url().includes("/auth/logout") &&
          res.status() === 204,
      ),
      page.getByRole("button", { name: "Cerrar sesión" }).click(),
    ]);
    assert.equal(finalLogoutRes.status(), 204);
    await page.waitForURL("**/acceso*", { timeout: 10000 });

    const finalCookies = await context.cookies();
    const finalSessionCookie = finalCookies.find(
      (c) => c.name === "__Host-espacigo-session",
    );
    assert.ok(!finalSessionCookie || finalSessionCookie.value === "");

    console.log(
      "PASS: native-browser-verification pipeline executed cleanly without mocks",
    );
    process.exitCode = 0;
  } catch (error) {
    const errorName = error instanceof Error ? error.name : "UnknownError";
    console.error(`FAIL: phase=${currentPhase} error=${errorName}`);
    process.exitCode = 1;
  } finally {
    if (page) {
      try {
        await page.close();
      } catch {}
    }
    if (context) {
      try {
        await context.close();
      } catch {}
    }
    if (browser) {
      try {
        await browser.close();
      } catch {}
    }
    if (edge) {
      try {
        await edge.close();
      } catch {}
    }
    if (nextChild) {
      await stopChild(nextChild);
    }
  }
}

main().catch((err) => {
  const errorName = err instanceof Error ? err.name : "UnknownError";
  console.error("FAIL: unhandled_exception", errorName);
  process.exit(1);
});
