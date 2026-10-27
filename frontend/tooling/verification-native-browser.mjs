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

    // Phase 5: Replay fragment once -> 422 invalid_token + Comprobar acceso CTA
    currentPhase = "REPLAY_VERIFY";
    await page.goto(
      `/verificar-correo#token_id=${encodeURIComponent(tokenId)}&token=${encodeURIComponent(token)}`,
      { waitUntil: "networkidle" },
    );
    await page.waitForFunction(() => window.location.hash === "", {
      timeout: 10000,
    });

    const [replayRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          new URL(res.url()).pathname === "/api/backend/auth/verification" &&
          res.status() === 422,
      ),
      page.getByRole("button", { name: "Verificar mi correo" }).click(),
    ]);
    assert.equal(replayRes.status(), 422);
    await page.waitForSelector("text=Comprobar acceso", { timeout: 10000 });

    // Phase 6: Authenticated login
    currentPhase = "LOGIN_SUCCESS";
    await page.goto("/acceso", { waitUntil: "networkidle" });
    await page.locator('input[name="email"]').fill(testEmail);
    await page
      .locator('input[name="password"], input[type="password"]')
      .first()
      .fill(testPassword);

    const [loginOkRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          res.url().includes("/auth/login") &&
          res.status() === 200,
      ),
      page.getByRole("button", { name: "Iniciar sesión" }).click(),
    ]);
    assert.equal(loginOkRes.status(), 200);
    await page.waitForURL("**/mi-cuenta*", { timeout: 15000 });

    const logoutBtn = page.getByRole("button", { name: "Cerrar sesión" });
    await logoutBtn.waitFor({ state: "visible", timeout: 10000 });

    const cookiesActive = await context.cookies();
    const sessionActive = cookiesActive.find(
      (c) => c.name === "__Host-espacigo-session",
    );
    const expiryActive = cookiesActive.find(
      (c) => c.name === "__Host-espacigo-session-expiry",
    );
    assert.ok(sessionActive && sessionActive.value, "session cookie required");
    assert.ok(
      expiryActive && expiryActive.value,
      "session expiry cookie required",
    );
    assert.equal(sessionActive.secure, true);
    assert.equal(sessionActive.httpOnly, true);
    assert.equal(expiryActive.secure, true);
    assert.equal(expiryActive.httpOnly, true);

    // Phase 7: Session persistence on reload
    currentPhase = "SESSION_PERSIST";
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
    await page
      .getByRole("button", { name: "Cerrar sesión" })
      .waitFor({ state: "visible", timeout: 10000 });

    // Phase 8: Logout via real UI
    currentPhase = "LOGOUT";
    const [logoutRes] = await Promise.all([
      page.waitForResponse(
        (res) =>
          res.request().method() === "POST" &&
          res.url().includes("/auth/logout"),
      ),
      page.getByRole("button", { name: "Cerrar sesión" }).click(),
    ]);
    assert.ok(logoutRes.status() === 200 || logoutRes.status() === 204);
    await page.waitForURL("**/acceso*", { timeout: 10000 });

    const cookiesPostLogout = await context.cookies();
    const sessionPostLogout = cookiesPostLogout.find(
      (c) => c.name === "__Host-espacigo-session",
    );
    const expiryPostLogout = cookiesPostLogout.find(
      (c) => c.name === "__Host-espacigo-session-expiry",
    );
    assert.ok(!sessionPostLogout || sessionPostLogout.value === "");
    assert.ok(!expiryPostLogout || expiryPostLogout.value === "");

    currentPhase = "COMPLETED";
  } catch (err) {
    console.error(`FAIL: phase=${currentPhase} error=${err?.name || "Error"}`);
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
      try {
        await stopChild(nextChild);
      } catch {}
    }
  }
}

main();
