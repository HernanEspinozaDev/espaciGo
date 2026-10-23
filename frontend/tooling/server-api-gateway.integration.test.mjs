import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { setTimeout as delay } from 'node:timers/promises';
import { after, test } from 'node:test';

const publicOrigin = 'https://spaces.example.test';
const forwardedRequests = [];
const apiServer = createServer((request, response) => {
  forwardedRequests.push({
    method: request.method,
    path: request.url,
    cookie: request.headers.cookie,
    authorization: request.headers.authorization,
    contentType: request.headers['content-type'],
  });
  request.resume();
  response.setHeader('Content-Type', 'application/json');
  response.setHeader('Set-Cookie', 'synthetic-upstream-cookie=must-not-leave; HttpOnly; Secure');
  response.end(JSON.stringify({ status: 'ok' }));
});

let frontendProcess;
let frontendBase;

after(async () => {
  frontendProcess?.kill('SIGTERM');
  if (frontendProcess) await Promise.race([once(frontendProcess, 'exit'), delay(3000)]);
  if (apiServer.listening) await new Promise((resolve) => apiServer.close(resolve));
});

test('public Express gateway strips the ambient IAP cookie and never returns upstream cookies', async () => {
  const apiPort = await listen(apiServer);
  const frontendPort = await unusedPort();
  frontendBase = `http://127.0.0.1:${frontendPort}`;
  frontendProcess = spawn(process.execPath, ['dist/frontend/server/server.mjs'], {
    cwd: new URL('..', import.meta.url).pathname,
    env: {
      ...process.env,
      PORT: String(frontendPort),
      FRONTEND_ENTRY_MODE: 'public',
      FRONTEND_PUBLIC_ORIGIN: publicOrigin,
      API_PUBLIC_BASE_URL: '/api/v1',
      API_INTERNAL_BASE_URL: `http://127.0.0.1:${apiPort}/api/v1`,
      API_SERVICE_AUDIENCE: '',
      IAP_JWT_AUDIENCE: '',
    },
    stdio: 'ignore',
  });

  await waitUntilReady(frontendBase, frontendProcess);

  const loginResponse = await fetch(`${frontendBase}/api/v1/auth/login`, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      Origin: publicOrigin,
      Cookie: 'GCP_IAP_UID=synthetic-cookie',
    },
    body: JSON.stringify({ email: 'synthetic@example.invalid', password: 'synthetic-only' }),
  });
  assert.equal(loginResponse.status, 200);
  assert.equal(loginResponse.headers.get('set-cookie'), null);
  assert.equal(forwardedRequests.length, 1);
  assert.deepEqual(forwardedRequests[0], {
    method: 'POST',
    path: '/api/v1/auth/login',
    cookie: undefined,
    authorization: undefined,
    contentType: 'application/json',
  });

  const sessionResponse = await fetch(`${frontendBase}/api/v1/auth/session`, {
    headers: {
      Accept: 'application/json',
      Authorization: 'Bearer synthetic-application-token-12345',
      Cookie: 'GCP_IAP_UID=synthetic-cookie',
    },
  });
  assert.equal(sessionResponse.status, 200);
  assert.equal(sessionResponse.headers.get('set-cookie'), null);
  assert.equal(forwardedRequests.length, 2);
  assert.deepEqual(forwardedRequests[1], {
    method: 'GET',
    path: '/api/v1/auth/session',
    cookie: undefined,
    authorization: 'Bearer synthetic-application-token-12345',
    contentType: undefined,
  });

  const beforeRejectedRequests = forwardedRequests.length;
  const arbitraryCookieResponse = await fetch(`${frontendBase}/api/v1/auth/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Origin: publicOrigin,
      Cookie: 'GCP_IAP_UID=synthetic-cookie; app_session=synthetic-cookie',
    },
    body: '{}',
  });
  assert.equal(arbitraryCookieResponse.status, 404);
  assert.equal(forwardedRequests.length, beforeRejectedRequests);

  const invalidOriginResponse = await fetch(`${frontendBase}/api/v1/auth/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Origin: 'https://attacker.example',
      Cookie: 'GCP_IAP_UID=synthetic-cookie',
    },
    body: '{}',
  });
  assert.equal(invalidOriginResponse.status, 404);
  assert.equal(forwardedRequests.length, beforeRejectedRequests);
});

async function listen(server) {
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  return server.address().port;
}

async function unusedPort() {
  const server = createServer();
  const port = await listen(server);
  await new Promise((resolve) => server.close(resolve));
  return port;
}

async function waitUntilReady(base, child) {
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) throw new Error('Frontend test server exited before readiness');
    try {
      const response = await fetch(`${base}/healthz`, { signal: AbortSignal.timeout(500) });
      if (response.ok) return;
    } catch {
      await delay(100);
    }
  }
  throw new Error('Frontend test server did not become ready');
}
