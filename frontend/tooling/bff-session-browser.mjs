import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";
import net from "node:net";
import {
  startSessionUpstream,
  startSessionTlsProxy,
} from "./bff-test-servers.mjs";

const root = fileURLToPath(new URL("../", import.meta.url));
const out = path.resolve(root, "../.local/testing/ui-auth-phase2");
const native = process.env.SESSION_BFF_NATIVE_URL;
const ports = [4330, 4331];
const results = [];
const children = [];
const logs = [];
const sensitive = [];
const networkSurfaces = [];
const pendingReads = [];
let fixture, edge, context;
fs.mkdirSync(out, { recursive: true, mode: 0o700 });
fs.chmodSync(out, 0o700);
const check = (ok, text) => assert.ok(ok, `SAFE: ${text}`);
async function run(name, task) {
  try {
    await task();
    results.push({ name, passed: true });
    console.log(`PASS ${name}`);
  } catch (err) {
    results.push({
      name,
      passed: false,
      reason: err.message?.startsWith("SAFE:")
        ? err.message
        : "Browser or transport action failed; private details withheld",
    });
    console.log(`FAIL ${name}`);
  }
}
async function ready(port) {
  for (let i = 0; i < 100; i++) {
    try {
      const r = await fetch(`http://127.0.0.1:${port}/api/health`);
      if (r.ok) return;
    } catch {
      /* owned process is starting */
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error("SAFE: owned Next process did not become ready");
}
async function startNext(upstream) {
  check(
    fs.existsSync(path.join(root, ".next/standalone/server.js")),
    "build standalone first",
  );
  fs.cpSync(
    path.join(root, "public"),
    path.join(root, ".next/standalone/public"),
    { recursive: true },
  );
  fs.cpSync(
    path.join(root, ".next/static"),
    path.join(root, ".next/standalone/.next/static"),
    { recursive: true },
  );
  for (const port of ports) {
    await new Promise((resolve, reject) => {
      const reservation = net.createServer();
      reservation.once("error", reject);
      reservation.listen(port, "127.0.0.1", () => reservation.close(resolve));
    });
    const env = {
      PATH: process.env.PATH,
      HOME: process.env.HOME,
      TMPDIR: process.env.TMPDIR,
      NODE_ENV: "production",
      HOSTNAME: "127.0.0.1",
      PORT: String(port),
      API_BACKEND_BASE_URL: upstream,
    };
    const child = spawn(
      process.execPath,
      [path.join(root, ".next/standalone/server.js")],
      { cwd: root, env, stdio: ["ignore", "pipe", "pipe"] },
    );
    children.push(child);
    child.stdout.on("data", (chunk) => logs.push(chunk.toString()));
    child.stderr.on("data", (chunk) => logs.push(chunk.toString()));
    await ready(port);
  }
}
async function main() {
  let email = "bff-test@example.invalid",
    password = "SyntheticBff!123";
  if (native) {
    check(
      new URL(native).origin === "http://127.0.0.1:18084",
      "native target must be the verified isolated Go harness",
    );
    const base =
      "/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/.local/testing";
    const policy = JSON.parse(
      fs.readFileSync(path.join(base, "test-account-policy.json"), "utf8"),
    );
    const inventory = JSON.parse(
      fs.readFileSync(path.join(base, "phase3-account-inventory.json"), "utf8"),
    );
    check(
      inventory.status === "verified" && inventory.email.endsWith(".invalid"),
      "existing synthetic native inventory is required",
    );
    email = inventory.email;
    password = policy.account_policy.password;
  } else fixture = await startSessionUpstream(18085);
  sensitive.push(email, password);
  await startNext(native || fixture.url);
  check(
    Boolean(process.env.SESSION_TLS_CERT && process.env.SESSION_TLS_KEY),
    "private localhost TLS files are required",
  );
  edge = await startSessionTlsProxy({
    port: 18443,
    targetPort: ports[0],
    certFile: process.env.SESSION_TLS_CERT,
    keyFile: process.env.SESSION_TLS_KEY,
  });
  const profile = fs.mkdtempSync(
    path.join(out, native ? "native-browser-" : "controlled-browser-"),
  );
  fs.chmodSync(profile, 0o700);
  const browserErrors = [];
  let normalBearer = false;
  const launch = async () => {
    const ctx = await chromium.launchPersistentContext(profile, {
      headless: true,
      ignoreHTTPSErrors: true,
      viewport: { width: 1365, height: 900 },
    });
    await ctx.route("**/*", (route) =>
      new URL(route.request().url()).origin === edge.url
        ? route.continue()
        : route.abort(),
    );
    ctx.setDefaultTimeout(8000);
    ctx.setDefaultNavigationTimeout(15000);
    for (const existing of ctx.pages())
      existing.on("console", (msg) => {
        if (msg.type() === "error") browserErrors.push(msg.text());
      });
    ctx.on("page", (page) =>
      page.on("console", (msg) => {
        if (msg.type() === "error") browserErrors.push(msg.text());
      }),
    );
    ctx.on("request", (req) => {
      networkSurfaces.push(req.url());
      if (req.headers().authorization) normalBearer = true;
    });
    ctx.on("response", (response) => {
      if (
        /javascript|json|text\/html/.test(
          response.headers()["content-type"] || "",
        )
      ) {
        pendingReads.push(
          response
            .text()
            .then((text) => networkSurfaces.push(text))
            .catch(() => {}),
        );
      }
    });
    return ctx;
  };
  context = await launch();
  let page = context.pages()[0] || (await context.newPage());
  const login = async () => {
    await page.goto(`${edge.url}/acceso`);
    await page.getByLabel("Correo electrónico").fill(email);
    await page.getByLabel("Contraseña", { exact: true }).fill(password);
    const responsePromise = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === "/api/backend/auth/login" &&
        r.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    const response = await responsePromise;
    check(response.status() === 200, "login must succeed");
    const body = await response.json();
    check(
      Object.keys(body).sort().join(",") ===
        "account_id,capabilities,expires_at,restricted_mode,roles",
      "BFF login must whitelist its public DTO",
    );
    await page.waitForURL((url) => url.pathname === "/mi-cuenta");
    await page.getByLabel("Nombre visible").waitFor();
    return body;
  };
  const api = async (method, pathname, data, headers = {}) =>
    context.request.fetch(`${edge.url}/api/backend${pathname}`, {
      method,
      data,
      headers: { Origin: edge.url, ...headers },
      failOnStatusCode: false,
    });
  const authCookies = async () =>
    (await context.cookies(edge.url)).filter((c) =>
      c.name.startsWith("__Host-espacigo-session"),
    );
  let initialExpiry;
  await run("login BFF confirmado, DTO seguro y migración legacy", async () => {
    await page.goto(`${edge.url}/acceso`);
    await page.evaluate(() =>
      sessionStorage.setItem(
        "espacigo_auth_session",
        JSON.stringify({
          accessToken: "synthetic-legacy-not-a-credential",
          session: { roles: ["administrador_global"] },
        }),
      ),
    );
    const body = await login();
    initialExpiry = body.expires_at;
    const cookies = await authCookies();
    check(cookies.length === 2, "two persistent host cookies are required");
    check(
      cookies.every(
        (c) =>
          c.httpOnly &&
          c.secure &&
          c.sameSite === "Lax" &&
          c.path === "/" &&
          c.domain === "127.0.0.1" &&
          c.expires > Date.now() / 1000 &&
          c.expires <= Date.parse(body.expires_at) / 1000,
      ),
      "cookie flags and expiry must match the contract",
    );
    const credential = cookies.find(
      (c) => c.name === "__Host-espacigo-session",
    );
    check(Boolean(credential), "opaque credential cookie required");
    sensitive.push(credential.value);
    const surfaces = await page.evaluate(() => ({
      cookie: document.cookie,
      local: Object.entries(localStorage),
      session: Object.entries(sessionStorage),
      html: document.documentElement.outerHTML,
    }));
    check(
      !surfaces.session.some(([key]) => key === "espacigo_auth_session"),
      "legacy session key must be removed",
    );
    check(
      ![password, credential.value].some(
        (value) => value && JSON.stringify(surfaces).includes(value),
      ),
      "credentials must not appear in JS-readable surfaces",
    );
    check(
      !normalBearer,
      "browser must not send Authorization during normal use",
    );
  });
  if (!results[0]?.passed) throw new Error("Initial BFF proof failed");
  await run("recarga y pestaña nueva revalidan la misma sesión", async () => {
    await page.reload();
    await page.getByLabel("Nombre visible").waitFor();
    const peer = await context.newPage();
    await peer.goto(`${edge.url}/mi-cuenta`);
    await peer.getByLabel("Nombre visible").waitFor();
    await peer.close();
    const response = await api("GET", "/auth/session");
    check(response.status() === 200, "restored session must confirm");
    check(
      Date.parse((await response.json()).expires_at) ===
        Date.parse(initialExpiry),
      "session checks must not renew expiry",
    );
    check(
      response.headers()["cache-control"]?.includes("no-store"),
      "session must not be shared-cacheable",
    );
  });
  await run(
    "cookie persiste al cerrar y reabrir Chromium aislado",
    async () => {
      await context.close();
      context = await launch();
      page = await context.newPage();
      check(
        (await authCookies()).length === 2,
        "cookie must survive browser restart",
      );
      await page.goto(`${edge.url}/mi-cuenta`);
      await page.getByLabel("Nombre visible").waitFor();
    },
  );
  await run(
    "continuidad entre dos procesos Next sin sesión en memoria del servidor",
    async () => {
      edge.switchTo(ports[1]);
      await page.reload();
      await page.getByLabel("Nombre visible").waitFor();
      const response = await api("GET", "/auth/session");
      check(
        response.status() === 200,
        "second Next process must accept the same Go credential",
      );
      check(
        Date.parse((await response.json()).expires_at) ===
          Date.parse(initialExpiry),
        "cross-instance continuity must not renew expiry",
      );
    },
  );
  await run(
    "CSRF sin Origin, Origin ajeno y forwarded spoof no borran sesión",
    async () => {
      for (const headers of [
        {},
        { Origin: "null" },
        {
          Origin: "https://outside.invalid",
          "X-Forwarded-Host": "outside.invalid",
        },
      ]) {
        const response = await context.request.post(
          `${edge.url}/api/backend/auth/logout`,
          { headers },
        );
        check(response.status() === 403, "invalid Origin must be rejected");
      }
      check(
        (await authCookies()).length === 2,
        "CSRF rejection must not erase a valid session",
      );
    },
  );
  if (fixture) {
    await run(
      "gateway conserva streaming e idempotencia sin reenviar Cookie o forwarded",
      async () => {
        const stream = await api("GET", "/privacy/export/archive");
        check(
          stream.status() === 200 &&
            stream.headers()["content-type"]?.includes("application/zip"),
          "private binary stream must remain binary",
        );
        check(
          (await stream.body()).length > 0 &&
            stream.headers()["content-disposition"]?.includes("synthetic.zip"),
          "binary bytes and disposition must survive",
        );
        const key = "770e8400-e29b-41d4-a716-446655440001";
        const mutation = await api(
          "POST",
          "/local/booking-trial/reservations",
          { synthetic_transport_probe: true },
          { "Idempotency-Key": key },
        );
        check(
          mutation.status() === 201,
          "allowlisted mutation must reach the synthetic upstream",
        );
        check(
          fixture.observations.some(
            (r) =>
              r.pathname === "/api/v1/local/booking-trial/reservations" &&
              r.idempotencyKey === key &&
              r.bearerPresent,
          ),
          "idempotency and Go bearer must remain server-side",
        );
        check(
          fixture.observations.every(
            (r) => !r.cookieForwarded && !r.forwardedPresent,
          ),
          "raw cookies and forwarded headers must never reach Go",
        );
      },
    );
    await run(
      "503 de sesión conserva cookies para revalidación posterior",
      async () => {
        fixture.state.sessionFailure = true;
        try {
          check(
            (await api("GET", "/auth/session")).status() === 503,
            "upstream outage must not become invalid credentials",
          );
          check(
            (await authCookies()).length === 2,
            "outage must not erase cookies",
          );
        } finally {
          fixture.state.sessionFailure = false;
        }
      },
    );
    await run(
      "Authorization del navegador se rechaza sin credencial alternativa",
      async () => {
        const response = await api("GET", "/profile", undefined, {
          Authorization: "Bearer synthetic-injected-not-a-credential",
        });
        check(
          response.status() >= 400 && response.status() < 500,
          "browser bearer injection must be rejected",
        );
      },
    );
    await run(
      "logout sin red purga otras pestañas y marcador impide restauración",
      async () => {
        const peer = await context.newPage();
        await peer.goto(`${edge.url}/mi-cuenta`);
        await peer.getByLabel("Nombre visible").waitFor();
        await page.goto(`${edge.url}/mi-cuenta`);
        await page.getByRole("button", { name: "Cerrar sesión" }).waitFor();
        await context.setOffline(true);
        try {
          await page.getByRole("button", { name: "Cerrar sesión" }).click();
          await peer.waitForFunction(
            () => localStorage.getItem("espacigo_logout_pending") === "1",
          );
          await peer
            .getByLabel("Nombre visible")
            .waitFor({ state: "detached" });
        } finally {
          await context.setOffline(false);
        }
        await peer.evaluate(() => window.dispatchEvent(new Event("focus")));
        await peer.waitForFunction(
          () => localStorage.getItem("espacigo_logout_pending") === null,
        );
        check(
          (await api("GET", "/auth/session")).status() === 401,
          "pending logout must clear the BFF cookie before restoration",
        );
        await peer.close();
      },
    );
    await run(
      "logout con Go caído borra cookie y no afirma revocación remota",
      async () => {
        await login();
        fixture.state.logoutFailure = true;
        try {
          const response = await api("POST", "/auth/logout");
          check(
            response.status() === 502 &&
              response.headers()["x-espacigo-session-cleared"] === "1",
            "partial logout must explicitly identify local cookie cleanup",
          );
          check(
            (await authCookies()).length === 0,
            "remote failure must still remove local cookies",
          );
          check(
            (await response.json()).error?.code === "remote_logout_unconfirmed",
            "remote failure must not be reported as successful revocation",
          );
        } finally {
          fixture.state.logoutFailure = false;
        }
      },
    );
    await run(
      "revocación Go vence incluso con metadata de expiración manipulada",
      async () => {
        await login();
        const metadata = (await authCookies()).find(
          (c) => c.name === "__Host-espacigo-session-expiry",
        );
        await context.addCookies([
          {
            ...metadata,
            value: new Date(Date.now() + 3_600_000).toISOString(),
          },
        ]);
        fixture.revokeAll();
        check(
          (await api("GET", "/auth/session")).status() === 401,
          "Go revocation must remain authoritative",
        );
        check(
          (await authCookies()).length === 0,
          "revoked session must clear both cookies",
        );
      },
    );
    await run(
      "expiración absoluta elimina cookie sin renovaciones",
      async () => {
        fixture.state.ttlMs = 2500;
        const response = await api("POST", "/auth/login", { email, password });
        check(
          response.status() === 200,
          "short synthetic lifetime login must succeed",
        );
        await new Promise((resolve) => setTimeout(resolve, 3000));
        check(
          (await api("GET", "/auth/session")).status() === 401,
          "expired cookie must not restore a session",
        );
        check(
          (await authCookies()).length === 0,
          "expired credentials must not remain in cookies",
        );
        fixture.state.ttlMs = 60_000;
      },
    );
    await login();
  }
  await run("logout normal revoca Go y elimina ambas cookies", async () => {
    const secret = (await authCookies()).find(
      (c) => c.name === "__Host-espacigo-session",
    )?.value;
    check(Boolean(secret), "valid cookie is required before logout");
    const response = await api("POST", "/auth/logout");
    check(response.status() === 204, "normal remote logout must confirm");
    check(
      (await authCookies()).length === 0,
      "normal logout must remove both cookies",
    );
    check(
      (await api("GET", "/auth/session")).status() === 401,
      "session must remain anonymous after logout",
    );
    if (native) {
      const direct = await fetch(`${native}/auth/session`, {
        headers: { Authorization: `Bearer ${decodeURIComponent(secret)}` },
      });
      check(
        direct.status === 401,
        "real Go must reject the previously issued credential after BFF logout",
      );
    }
  });
  await run(
    "tokens ausentes de JSON, HTML, JS, URLs, consola y logs Next",
    async () => {
      await Promise.allSettled(pendingReads);
      const values = [...sensitive, ...(fixture?.secrets() || [])];
      const text = [...logs, ...browserErrors].join("\n");
      check(
        !values.some((value) => value && text.includes(value)),
        "logs must never contain credentials",
      );
      const network = networkSurfaces.join("\n");
      const credentials = [
        password,
        ...sensitive.slice(2),
        ...(fixture?.secrets() || []),
      ];
      check(
        !credentials.some((value) => value && network.includes(value)),
        "response surfaces and URLs must never expose credentials",
      );
    },
  );
}
async function cleanup() {
  await context?.close().catch(() => {});
  await edge?.close().catch(() => {});
  await fixture?.close().catch(() => {});
  await Promise.all(
    children.map(
      (child) =>
        new Promise((resolve) => {
          if (child.exitCode !== null) return resolve();
          child.once("exit", resolve);
          child.kill("SIGTERM");
          const timer = setTimeout(() => {
            child.kill("SIGKILL");
            resolve();
          }, 5000);
          timer.unref();
        }),
    ),
  );
  const passed = results.filter((r) => r.passed).length;
  const report = {
    mode: native
      ? "real-go-native-authorized-db"
      : "controlled-upstream-real-next-bff",
    passed,
    failed: results.length - passed,
    results,
    setupFailed: Boolean(process.exitCode),
  };
  fs.writeFileSync(
    path.join(
      out,
      native ? "native-browser-report.json" : "controlled-browser-report.json",
    ),
    `${JSON.stringify(report, null, 2)}\n`,
    { mode: 0o600 },
  );
  console.log(
    JSON.stringify({
      mode: report.mode,
      passed: report.passed,
      failed: report.failed,
      setupFailed: report.setupFailed,
    }),
  );
  if (!results.length || report.failed || report.setupFailed)
    process.exitCode = 1;
}
main()
  .catch(() => {
    console.error("Phase-2 browser proof failed; private details withheld");
    process.exitCode = 1;
  })
  .finally(cleanup);
