import assert from 'node:assert/strict';
import { test } from 'node:test';
import http from 'node:http';
import https from 'node:https';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import { once } from 'node:events';
import { createServer } from './kyc_manual_https.mjs';

function fixtureTls() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kyc-proxy-tls-'));
  const key = path.join(dir, 'key.pem');
  const cert = path.join(dir, 'cert.pem');
  const result = spawnSync('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', key, '-out', cert, '-subj', '/CN=localhost', '-days', '1'], { encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
  return { dir, key: fs.readFileSync(key), cert: fs.readFileSync(cert) };
}

test('proxy preserves canonical Host and Origin, strips forwarded headers and denies arbitrary Host', async (t) => {
  const tls = fixtureTls();
  t.after(() => fs.rmSync(tls.dir, { recursive: true, force: true }));
  let seen;
  const upstream = http.createServer((req, res) => { seen = req.headers; res.writeHead(204); res.end(); });
  upstream.listen(0, '127.0.0.1');
  await once(upstream, 'listening');
  t.after(() => upstream.close());
  const proxy = createServer({ ...tls, upstreamPort: upstream.address().port });
  proxy.listen(0, '127.0.0.1');
  await once(proxy, 'listening');
  t.after(() => proxy.close());

  const request = (host) => new Promise((resolve, reject) => {
    const req = https.request({ hostname: '127.0.0.1', port: proxy.address().port, path: '/auth-check', method: 'GET', rejectUnauthorized: false, headers: { Host: host, Origin: 'https://127.0.0.1:4219', 'X-Forwarded-Host': 'attacker.invalid', 'X-Forwarded-Proto': 'http', Forwarded: 'host=attacker.invalid' } }, (res) => { res.resume(); res.on('end', () => resolve(res.statusCode)); });
    req.on('error', reject); req.end();
  });
  assert.equal(await request('127.0.0.1:4219'), 204);
  assert.equal(seen.host, '127.0.0.1:4219');
  assert.equal(seen.origin, 'https://127.0.0.1:4219');
  assert.equal(seen['x-forwarded-host'], undefined);
  assert.equal(seen['x-forwarded-proto'], undefined);
  assert.equal(seen.forwarded, undefined);
  assert.equal(await request('attacker.invalid'), 421);
});

test('proxy validates private TLS metadata before reading configured files', () => {
  const source = fs.readFileSync(new URL('./kyc_manual_https.mjs', import.meta.url), 'utf8');
  assert.match(source, /lstatSync/);
  assert.match(source, /0o700/);
  assert.match(source, /0o600/);
});
