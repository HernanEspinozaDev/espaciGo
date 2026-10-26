import http from "node:http";
import https from "node:https";
import { readFileSync } from "node:fs";
import { randomBytes } from "node:crypto";

/** Synthetic upstream for testing the REAL Next BFF, never production auth. */
export async function startSessionUpstream(port = 0) {
  const sessions = new Map();
  const issued = [];
  const observations = [];
  const state = {
    ttlMs: 60_000,
    sessionFailure: false,
    logoutFailure: false,
    profile404: false,
    profileName: "Cuenta sintética",
    sessionDelayMs: 0,
  };
  const identity = {
    account_id: "770e8400-e29b-41d4-a716-446655440000",
    roles: ["arrendatario"],
    capabilities: [],
    restricted_mode: false,
  };
  const server = http.createServer(async (req, res) => {
    const pathname = new URL(req.url, "http://127.0.0.1").pathname;
    const token = (req.headers.authorization || "").replace(/^Bearer /, "");
    observations.push({
      method: req.method,
      pathname,
      cookieForwarded: Boolean(req.headers.cookie),
      bearerPresent: Boolean(token),
      forwardedPresent: Boolean(
        req.headers["x-forwarded-host"] || req.headers["x-forwarded-for"],
      ),
      idempotencyKey: req.headers["idempotency-key"] || null,
    });
    const reply = (status, body) => {
      res.writeHead(status, {
        "content-type": "application/json",
        "cache-control": "no-store",
      });
      res.end(status === 204 ? undefined : JSON.stringify(body));
    };
    const fail = (status, code) =>
      reply(status, {
        error: { code, message: "Synthetic upstream response" },
      });
    if (pathname === "/api/v1/auth/login" && req.method === "POST") {
      let text = "";
      for await (const chunk of req) {
        text += chunk;
        if (text.length > 4096) return fail(413, "too_large");
      }
      let body;
      try {
        body = JSON.parse(text);
      } catch {
        return fail(400, "invalid_request");
      }
      if (
        body.email !== "bff-test@example.invalid" ||
        body.password !== "SyntheticBff!123"
      )
        return fail(401, "unauthenticated");
      const secret = randomBytes(32).toString("base64url");
      const expiry = Date.now() + state.ttlMs;
      sessions.set(secret, { expiry, revoked: false });
      issued.push(secret);
      return reply(200, {
        ...identity,
        access_token: secret,
        token_type: "Bearer",
        expires_at: new Date(expiry).toISOString(),
        debug_field: "must-not-be-forwarded",
      });
    }
    const session = sessions.get(token);
    if (!session || session.revoked || Date.now() >= session.expiry)
      return fail(401, "unauthenticated");
    if (pathname === "/api/v1/auth/session") {
      if (state.sessionDelayMs > 0) {
        await new Promise((resolve) =>
          setTimeout(resolve, state.sessionDelayMs),
        );
      }
      if (state.sessionFailure) return fail(503, "temporarily_unavailable");
      return reply(200, identity);
    }
    if (pathname === "/api/v1/auth/logout" && req.method === "POST") {
      if (state.logoutFailure) return fail(503, "temporarily_unavailable");
      session.revoked = true;
      return reply(204);
    }
    if (
      pathname === "/api/v1/local/booking-trial/reservations" &&
      req.method === "POST"
    ) {
      for await (const chunk of req) {
        // Consume only the synthetic transport probe.
        void chunk;
      }
      return reply(201, { synthetic_transport_probe: true });
    }
    if (pathname === "/api/v1/profile" && req.method === "GET") {
      if (state.profile404) return fail(404, "profile_not_found");
      return reply(200, {
        display_name: state.profileName,
        phone: null,
        updated_at: "2026-10-26T00:00:00Z",
      });
    }
    if (pathname === "/api/v1/profile" && req.method === "PUT") {
      let text = "";
      for await (const chunk of req) {
        text += chunk;
        if (text.length > 4096) return fail(413, "too_large");
      }
      let body;
      try {
        body = JSON.parse(text);
      } catch {
        return fail(400, "invalid_request");
      }
      if (body.display_name !== undefined) {
        state.profileName = String(body.display_name);
      }
      return reply(200, {
        display_name: state.profileName,
        phone: body.phone ?? null,
        updated_at: new Date().toISOString(),
      });
    }
    if (pathname === "/api/v1/profile/photo")
      return fail(404, "photo_not_found");
    if (pathname === "/api/v1/payout-account")
      return fail(404, "payout_not_found");
    if (pathname === "/api/v1/rights-requests")
      return reply(200, { items: [] });
    if (pathname === "/api/v1/privacy/export/archive") {
      const bytes = Buffer.from("UEsFBgAAAAAAAAAAAAAAAAAAAAAAAA==", "base64");
      res.writeHead(200, {
        "content-type": "application/zip",
        "content-disposition": "attachment; filename=synthetic.zip",
        "content-length": String(bytes.length),
      });
      return res.end(bytes);
    }
    if (pathname === "/api/v1/privacy/export") {
      res.writeHead(200, {
        "content-type": "application/json",
        "content-disposition": "attachment; filename=synthetic.json",
      });
      return res.end(JSON.stringify({ synthetic: true }));
    }
    return fail(404, "not_found");
  });
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(port, "127.0.0.1", resolve);
  });
  return {
    url: `http://127.0.0.1:${server.address().port}/api/v1`,
    state,
    observations,
    secrets: () => [...issued],
    revokeAll: () => {
      for (const item of sessions.values()) item.revoked = true;
    },
    close: () =>
      new Promise((resolve) => {
        server.close(resolve);
        server.closeAllConnections();
      }),
  };
}

/** Test-owned TLS termination; target is always a fixed loopback Next port. */
export async function startSessionTlsProxy({
  port = 0,
  targetPort,
  certFile,
  keyFile,
}) {
  let target = targetPort;
  const server = https.createServer(
    { cert: readFileSync(certFile), key: readFileSync(keyFile) },
    (req, res) => {
      const upstream = http.request(
        {
          hostname: "127.0.0.1",
          port: target,
          path: req.url,
          method: req.method,
          headers: req.headers,
        },
        (remote) => {
          res.writeHead(remote.statusCode, remote.headers);
          remote.pipe(res);
        },
      );
      upstream.on("error", () => {
        if (!res.headersSent) res.writeHead(502);
        res.end("Local test proxy unavailable");
      });
      req.on("aborted", () => upstream.destroy());
      req.pipe(upstream);
    },
  );
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(port, "127.0.0.1", resolve);
  });
  return {
    url: `https://127.0.0.1:${server.address().port}`,
    switchTo: (next) => {
      if (!Number.isInteger(next) || next < 1 || next > 65535)
        throw new Error("Invalid local target port");
      target = next;
    },
    close: () =>
      new Promise((resolve) => {
        server.close(resolve);
        server.closeAllConnections();
      }),
  };
}
