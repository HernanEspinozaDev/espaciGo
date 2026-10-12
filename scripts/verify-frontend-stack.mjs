const frontendBase = required('FRONTEND_INTERNAL_URL');
const apiInternalBase = required('API_INTERNAL_BASE_URL').replace(/\/$/, '');
const apiPublicBase = required('API_PUBLIC_BASE_URL').replace(/\/$/, '');
const publicOrigin = required('FRONTEND_PUBLIC_ORIGIN');

const root = await fetch(frontendBase);
assert(root.status === 200, `Angular root returned HTTP ${root.status}`);
const html = await root.text();
assert(html.includes('<app-root'), 'Angular shell is not present in the HTML response');
report('Angular root is healthy over the Compose API network');

const configResponse = await fetch(new URL('/config.json', frontendBase));
assert(configResponse.status === 200, `Runtime config returned HTTP ${configResponse.status}`);
assert(configResponse.headers.get('cache-control')?.includes('no-store'), 'Runtime config must not be cached');
const config = await configResponse.json();
assert(config.apiBaseUrl === apiPublicBase, 'Browser API URL does not match the published API origin');
assert(!/[?#]/.test(config.apiBaseUrl), 'API URL unexpectedly includes query or fragment data');
report('Runtime config exposes the host-reachable API URL without credentials');

const readiness = await fetch(new URL('/health/ready', apiInternalBase), {
  headers: { Origin: publicOrigin },
});
assert(readiness.status === 200, `Go readiness returned HTTP ${readiness.status}`);
assert(readiness.headers.get('access-control-allow-origin') === publicOrigin, 'Go readiness CORS origin mismatch');
const readyBody = await readiness.json();
assert(readyBody.status === 'ready', 'Go readiness did not report ready');
report('Go readiness is reachable from the frontend network with browser CORS');

const preflight = await fetch(`${apiInternalBase}/auth/login`, {
  method: 'OPTIONS',
  headers: {
    Origin: publicOrigin,
    'Access-Control-Request-Method': 'POST',
    'Access-Control-Request-Headers': 'authorization,content-type',
  },
});
assert(preflight.status === 204, `Go CORS preflight returned HTTP ${preflight.status}`);
assert(preflight.headers.get('access-control-allow-origin') === publicOrigin, 'Go preflight origin mismatch');
assert(preflight.headers.get('access-control-allow-methods')?.includes('POST'), 'Go preflight omits POST');
assert(preflight.headers.get('access-control-allow-headers')?.toLowerCase().includes('authorization'), 'Go preflight omits Authorization');
report('Go accepts the browser login preflight from the published Angular origin');

function required(name) {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required environment variable ${name}`);
  return value;
}

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function report(message) {
  console.log(`PASS ${message}`);
}
