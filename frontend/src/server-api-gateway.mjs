import { verifyIapAssertion } from './iap-assertion.mjs';

const uuid = '[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}';
const publicPaths = [
  /^\/api\/v1\/public\/spaces$/,
  new RegExp(`^/api/v1/public/spaces/${uuid}$`),
  new RegExp(`^/api/v1/public/spaces/${uuid}/images/${uuid}$`),
  /^\/api\/v1\/public\/space-categories$/,
  /^\/api\/v1\/public\/space-categories\/[a-z0-9_-]+\/filters$/,
  /^\/api\/v1\/public\/locations$/,
];

export function apiRequestAllowed(mode, method, requestUrl, headers = {}) {
  if (!['public', 'private', 'local'].includes(mode)) return false;
  if (!['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE'].includes(method.toUpperCase())) return false;
  let pathname;
  try {
    pathname = new URL(requestUrl, 'http://frontend.invalid').pathname;
  } catch {
    return false;
  }
  if (!pathname.startsWith('/api/v1/') && pathname !== '/api/v1') return false;
  if (pathname.includes('%') || pathname.includes('\\') || pathname.includes('//')) return false;
  if (pathname.split('/').some((part) => part === '.' || part === '..')) return false;
  if (mode !== 'public') return true;
  if (method.toUpperCase() !== 'GET') return false;
  if (hasHeader(headers, 'authorization') || hasHeader(headers, 'cookie')) return false;
  if (hasHeader(headers, 'transfer-encoding')) return false;
  const contentLength = getHeader(headers, 'content-length');
  if (contentLength !== undefined && contentLength !== '0') return false;
  return publicPaths.some((pattern) => pattern.test(pathname));
}

export function requestUrlWithinLimit(mode, requestUrl) {
  if (!['public', 'private', 'local'].includes(mode) || typeof requestUrl !== 'string') return false;
  const limitBytes = mode === 'public' ? 4096 : 16_384;
  return new TextEncoder().encode(requestUrl).byteLength <= limitBytes;
}

export function serviceAudienceMatches(apiBase, audience) {
  try {
    const target = new URL(apiBase);
    const parsedAudience = new URL(audience);
    return target.protocol === 'https:' && !target.username && !target.password &&
      parsedAudience.protocol === 'https:' && !parsedAudience.username && !parsedAudience.password &&
      parsedAudience.origin === target.origin && parsedAudience.pathname === '/' &&
      !parsedAudience.search && !parsedAudience.hash;
  } catch {
    return false;
  }
}

export function iapAssertionRequired(mode, pathname) {
  if (mode !== 'private') return false;
  return pathname !== '/healthz' && pathname !== '/health/ready';
}

export function privateEntryAudienceConfigured(mode, audience) {
  if (mode !== 'private') return true;
  return typeof audience === 'string' &&
    /^\/projects\/[1-9]\d*\/locations\/[a-z0-9-]+\/services\/[a-z][a-z0-9-]{0,62}$/.test(audience);
}

export async function iapRequestValid(mode, pathname, assertion, audience, verify = verifyIapAssertion) {
  if (!iapAssertionRequired(mode, pathname)) return true;
  if (typeof assertion !== 'string' || !audience) return false;
  return verify(assertion, audience);
}

// Only forward application credentials. Cloud Run service identity is always
// added by the server after this function; a browser-supplied IAM header is
// deliberately ignored.
export function clientApiHeaders(mode, headers = {}) {
  const result = new Headers();
  if (mode === 'public' && (hasHeader(headers, 'authorization') || hasHeader(headers, 'cookie'))) {
    return result;
  }
  const allowed = ['accept', 'accept-language', 'authorization', 'content-type', 'idempotency-key', 'if-match', 'if-none-match', 'origin', 'range', 'x-request-id'];
  for (const name of allowed) {
    const value = getHeader(headers, name);
    if (typeof value === 'string') result.set(name, value);
  }
  return result;
}

export function publicCacheControl(mode, upstreamValue) {
  if (mode !== 'public' || typeof upstreamValue !== 'string') return 'private, no-store';
  const match = /^public\s*,\s*max-age=(\d+)$/i.exec(upstreamValue.trim());
  if (!match) return 'private, no-store';
  const seconds = Number(match[1]);
  return Number.isSafeInteger(seconds) && seconds > 0 && seconds <= 300
    ? `public, max-age=${seconds}`
    : 'private, no-store';
}

function hasHeader(headers, name) {
  if (headers instanceof Headers) return headers.has(name);
  return Object.keys(headers).some((key) => key.toLowerCase() === name);
}

function getHeader(headers, name) {
  if (headers instanceof Headers) return headers.get(name) ?? undefined;
  const key = Object.keys(headers).find((candidate) => candidate.toLowerCase() === name);
  const value = key ? headers[key] : undefined;
  return Array.isArray(value) ? value[0] : value;
}
