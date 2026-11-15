import https from 'node:https';
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const privateRoot = path.join(root, '.local/testing/kyc-manual');
const certPath = path.join(privateRoot, 'localhost-cert.pem');
const keyPath = path.join(privateRoot, 'localhost-key.pem');

function validatePrivate(p, directory = false) {
  const s = fs.lstatSync(p);
  const expected = directory ? 0o700 : 0o600;
  if ((directory ? !s.isDirectory() : !s.isFile()) || (s.mode & 0o777) !== expected || s.uid !== process.getuid()) throw new Error('private TLS material metadata invalid');
}
function createServer(options = {}) {
  let cert = options.cert;
  let key = options.key;
  if (!cert || !key) {
    validatePrivate(privateRoot, true); validatePrivate(certPath); validatePrivate(keyPath);
    cert = fs.readFileSync(certPath); key = fs.readFileSync(keyPath);
  }
  const upstreamPort = options.upstreamPort ?? 4201;
  const host = options.host ?? '127.0.0.1';
  const port = options.port ?? 4219;
  return https.createServer({ cert, key, requestTimeout: 15000, headersTimeout: 10000 }, (req, res) => {
    if (req.headers.host !== '127.0.0.1:4219') {
      res.writeHead(421, { 'content-type': 'text/plain; charset=utf-8', connection: 'close' });
      res.end('Misdirected Request');
      return;
    }
    const headers = { ...req.headers };
    for (const h of Object.keys(headers)) if (/^x-forwarded-/i.test(h) || /^forwarded$/i.test(h)) delete headers[h];
    headers.host = '127.0.0.1:4219';
    const upstream = http.request({ hostname: '127.0.0.1', port: upstreamPort, method: req.method, path: req.url, headers, timeout: 10000 }, (up) => {
      res.writeHead(up.statusCode || 502, up.headers); up.pipe(res);
    });
    upstream.on('timeout', () => upstream.destroy(new Error('timeout')));
    upstream.on('error', () => { if (!res.headersSent) { res.writeHead(502, { 'content-type':'text/plain; charset=utf-8', 'connection':'close' }); res.end('Upstream unavailable'); } else res.destroy(); });
    req.on('aborted', () => upstream.destroy()); req.pipe(upstream);
  });
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  let server;
  let stopping = false;
  const stop = () => {
    if (stopping) return;
    stopping = true;
    const timeout = setTimeout(() => process.exit(1), 3000);
    timeout.unref();
    if (server) server.close(() => { clearTimeout(timeout); process.exit(0); });
    else process.exit(0);
  };
  try {
    server = createServer();
    server.on('error', () => { console.error('HTTPS proxy unavailable'); stop(); });
    server.listen(4219, '127.0.0.1');
    process.on('SIGTERM', stop); process.on('SIGINT', stop);
  } catch { console.error('HTTPS proxy not ready: private TLS material unavailable or invalid'); process.exit(1); }
}
export { createServer };
