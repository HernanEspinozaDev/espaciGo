import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import path from "node:path";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

const frontendDir = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
);
const standaloneServer = path.join(frontendDir, ".next/standalone/server.js");
const LOOPBACK = "127.0.0.1";
const PUBLIC_PORT = 4206;
const NEXT_PORT = 4209;
const API_BASE = "http://127.0.0.1:18086/api/v1";

export function validateTlsFiles(certFile, keyFile) {
  const root = path.resolve(
    frontendDir,
    "../.local/testing/wizard-improvements/tls",
  );
  for (const filename of [certFile, keyFile]) {
    assert.ok(filename, "TLS_CERT and TLS_KEY are required");
    const resolved = path.resolve(filename);
    assert.ok(
      resolved.startsWith(`${root}${path.sep}`),
      "TLS files must be under the wizard TLS directory",
    );
    let cursor = path.parse(resolved).root;
    for (const part of resolved.slice(cursor.length).split(path.sep)) {
      cursor = path.join(cursor, part);
      assert.ok(
        !fs.lstatSync(cursor).isSymbolicLink(),
        "TLS paths must not contain symlinks",
      );
    }
    const stat = fs.statSync(resolved);
    assert.ok(stat.isFile(), "TLS paths must be regular files");
    assert.equal(
      stat.uid,
      process.getuid?.(),
      "TLS file must belong to the current user",
    );
    if (resolved === path.resolve(keyFile))
      assert.equal(
        stat.mode & 0o777,
        0o600,
        "TLS private key must be mode 0600",
      );
  }
  for (const directory of [
    root,
    path.dirname(path.resolve(certFile)),
    path.dirname(path.resolve(keyFile)),
  ]) {
    const stat = fs.statSync(directory);
    assert.ok(
      stat.isDirectory() && (stat.mode & 0o777) === 0o700,
      "TLS parent directories must be mode 0700",
    );
    assert.equal(
      stat.uid,
      process.getuid?.(),
      "TLS directory must belong to current user",
    );
  }
  return { cert: fs.readFileSync(certFile), key: fs.readFileSync(keyFile) };
}

function assertPortFree(port) {
  return new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.once("error", reject);
    probe.listen(port, LOOPBACK, () => probe.close(resolve));
  });
}
function stopChild(child) {
  if (!child || child.exitCode !== null) return Promise.resolve();
  return new Promise((resolve) => {
    const timer = setTimeout(() => child.kill("SIGKILL"), 5000);
    child.once("exit", () => {
      clearTimeout(timer);
      resolve();
    });
    child.kill("SIGTERM");
  });
}
function waitNext(child) {
  return new Promise((resolve, reject) => {
    let settled = false;
    const deadline = setTimeout(
      () => finish(new Error("Next readiness timeout")),
      25000,
    );
    const finish = (error) => {
      if (settled) return;
      settled = true;
      clearTimeout(deadline);
      error ? reject(error) : resolve();
    };
    child.once("exit", (code, signal) =>
      finish(new Error(`Next exited during startup (${code ?? signal})`)),
    );
    const probe = () => {
      const req = http.get(`http://${LOOPBACK}:${NEXT_PORT}/`, (res) => {
        res.resume();
        if (res.statusCode >= 200 && res.statusCode < 500) finish();
        else if (!settled) setTimeout(probe, 200);
      });
      req.setTimeout(1000, () => req.destroy());
      req.on("error", () => {
        if (!settled) setTimeout(probe, 200);
      });
    };
    probe();
  });
}

async function main() {
  let child;
  let edge;
  try {
    assert.ok(
      fs.existsSync(standaloneServer),
      "Build the existing frontend standalone before running this harness",
    );
    const { certFile, keyFile } = process.env;
    const tls = validateTlsFiles(certFile, keyFile);
    await assertPortFree(PUBLIC_PORT);
    await assertPortFree(NEXT_PORT);
    child = spawn(process.execPath, [standaloneServer], {
      cwd: frontendDir,
      env: {
        PATH: process.env.PATH ?? "/usr/bin:/bin",
        HOME: process.env.HOME,
        ...(process.env.TMPDIR ? { TMPDIR: process.env.TMPDIR } : {}),
        NODE_ENV: "production",
        HOSTNAME: LOOPBACK,
        PORT: String(NEXT_PORT),
        API_BACKEND_BASE_URL: API_BASE,
        NEXT_TELEMETRY_DISABLED: "1",
      },
      stdio: "ignore",
    });
    await waitNext(child);
    edge = https.createServer(tls, (req, res) => {
      const headers = { ...req.headers };
      // Keep the browser's https Origin and Host intact; do not add forwarding headers.
      const upstream = http.request(
        {
          hostname: LOOPBACK,
          port: NEXT_PORT,
          path: req.url,
          method: req.method,
          headers,
        },
        (remote) => {
          res.writeHead(remote.statusCode, remote.headers);
          remote.pipe(res);
        },
      );
      upstream.on("error", () => {
        if (!res.headersSent) res.writeHead(502);
        res.end("Local Next harness unavailable");
      });
      req.on("aborted", () => upstream.destroy());
      req.pipe(upstream);
    });
    edge.on("clientError", (_error, socket) => socket.destroy());
    await new Promise((resolve, reject) => {
      edge.once("error", reject);
      edge.listen(PUBLIC_PORT, LOOPBACK, resolve);
    });
    console.log(
      `READY https://${LOOPBACK}:${PUBLIC_PORT} -> existing Next ${LOOPBACK}:${NEXT_PORT} -> API ${API_BASE}`,
    );
    await new Promise((resolve) => {
      let stopping = false;
      const shutdown = () => {
        if (stopping) return;
        stopping = true;
        edge.close(() => {});
        edge.closeAllConnections();
        stopChild(child).finally(resolve);
      };
      process.once("SIGINT", shutdown);
      process.once("SIGTERM", shutdown);
    });
  } catch (error) {
    console.error(
      `FAIL wizard-native-https: ${error instanceof Error ? error.message : "startup error"}`,
    );
    process.exitCode = 1;
  } finally {
    if (edge?.listening) {
      edge.closeAllConnections();
      await new Promise((resolve) => edge.close(resolve));
    }
    await stopChild(child);
  }
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
)
  main();
