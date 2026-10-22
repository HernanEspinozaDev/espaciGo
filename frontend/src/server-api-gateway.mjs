import { verifyIapAssertion } from './iap-assertion.mjs';
import { isIP } from 'node:net';

const uuid = '[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}';
const publicPaths = [
  /^\/api\/v1\/public\/spaces$/,
  new RegExp(`^/api/v1/public/spaces/${uuid}$`),
  new RegExp(`^/api/v1/public/spaces/${uuid}/images/${uuid}$`),
  /^\/api\/v1\/public\/space-categories$/,
  /^\/api\/v1\/public\/space-categories\/[a-z0-9_-]+\/filters$/,
  /^\/api\/v1\/public\/locations$/,
];
const unauthenticatedIdentityPaths = new Map([
  ['/api/v1/auth/login', { method: 'POST', maxBytes: 4096 }],
  ['/api/v1/auth/register', { method: 'POST', maxBytes: 8192 }],
  ['/api/v1/auth/verification/reissue', { method: 'POST', maxBytes: 4096 }],
  ['/api/v1/auth/verification', { method: 'POST', maxBytes: 4096 }],
  ['/api/v1/auth/password/recovery', { method: 'POST', maxBytes: 4096 }],
  ['/api/v1/auth/password/recovery/consume', { method: 'POST', maxBytes: 8192 }],
]);
const publicIdentityTermsPath = '/api/v1/auth/terms';
const identityIpLimitedPaths = new Set([
  '/api/v1/auth/register',
  '/api/v1/auth/verification/reissue',
  '/api/v1/auth/verification',
  '/api/v1/auth/password/recovery',
  '/api/v1/auth/password/recovery/consume',
]);

export function publicIdentityNeedsClientIp(method, requestUrl) {
  try {
    return method.toUpperCase() === 'POST' &&
      identityIpLimitedPaths.has(new URL(requestUrl, 'http://frontend.invalid').pathname);
  } catch {
    return false;
  }
}

// The external Application Load Balancer appends the network peer and its
// forwarding-rule IP to X-Forwarded-For. Earlier values are client supplied,
// so only use the second-to-last value when Cloud Run ingress is load-balanced.
// Outside Cloud Run, use the direct socket peer and ignore forwarded headers.
export function publicRequestClientIp(headers = {}, remoteAddress, behindTrustedLoadBalancer = false) {
  if (behindTrustedLoadBalancer) {
    const forwarded = getHeader(headers, 'x-forwarded-for');
    if (typeof forwarded !== 'string') return null;
    const chain = forwarded.split(',').map((value) => value.trim());
    if (chain.length < 2 || !isIP(chain.at(-1)) || !isIP(chain.at(-2))) return null;
    return chain.at(-2);
  }
  if (typeof remoteAddress !== 'string') return null;
  const peer = remoteAddress.startsWith('::ffff:') ? remoteAddress.slice(7) : remoteAddress;
  return isIP(peer) ? peer : null;
}

export function apiRequestAllowed(mode, method, requestUrl, headers = {}, expectedOrigin = null) {
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
  if (hasHeader(headers, 'cookie') || hasHeader(headers, 'x-serverless-authorization') ||
      hasHeader(headers, 'x-goog-iap-jwt-assertion') || hasHeader(headers, 'transfer-encoding')) return false;
  const authorization = getHeader(headers, 'authorization');
  if (authorization !== undefined && !/^Bearer [A-Za-z0-9._~-]{16,4096}$/.test(authorization)) return false;
  const url = new URL(requestUrl, 'http://frontend.invalid');
  const verb = method.toUpperCase();
  if (pathname === publicIdentityTermsPath) {
    const contentLength = getHeader(headers, 'content-length');
    return verb === 'GET' && !authorization && !url.search &&
      (contentLength === undefined || contentLength === '0');
  }
  const identityPost = unauthenticatedIdentityPaths.get(pathname);
  if (identityPost) {
    if (verb !== identityPost.method || authorization) return false;
    const contentLength = getHeader(headers, 'content-length');
    const contentType = getHeader(headers, 'content-type') ?? '';
    const origin = getHeader(headers, 'origin');
    return !url.search && validSameOrigin(origin, expectedOrigin) &&
      /^application\/json(?:\s*;\s*charset\s*=\s*"?utf-8"?)?\s*$/i.test(contentType) &&
      typeof contentLength === 'string' && /^\d+$/.test(contentLength) &&
      Number(contentLength) > 0 && Number(contentLength) <= identityPost.maxBytes;
  }
  if (verb === 'GET' && !authorization) {
    const contentLength = getHeader(headers, 'content-length');
    return !hasHeader(headers, 'authorization') &&
      (contentLength === undefined || contentLength === '0') &&
      publicPaths.some((pattern) => pattern.test(pathname));
  }
  return Boolean(authorization);
}

export function publicIdentityPostLimit(method, requestUrl) {
  try {
    const route = unauthenticatedIdentityPaths.get(new URL(requestUrl, 'http://frontend.invalid').pathname);
    return method.toUpperCase() === 'POST' && route?.method === 'POST' ? route.maxBytes : null;
  } catch {
    return null;
  }
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
export function clientApiHeaders(mode, headers = {}, requestUrl = '') {
  const result = new Headers();
  if (mode === 'public' && (hasHeader(headers, 'cookie') || hasHeader(headers, 'x-serverless-authorization') ||
      hasHeader(headers, 'x-goog-iap-jwt-assertion'))) {
    return result;
  }
  const allowed = ['accept', 'accept-language', 'authorization', 'content-type', 'idempotency-key', 'if-match', 'if-none-match', 'origin', 'range', 'x-request-id'];
  for (const name of allowed) {
    if (mode === 'public' && publicIdentityPostLimit('POST', requestUrl) !== null &&
        !['accept', 'content-type'].includes(name)) continue;
    const value = getHeader(headers, name);
    if (typeof value === 'string') result.set(name, value);
  }
  return result;
}

function validSameOrigin(origin, expectedOrigin) {
  if (typeof origin !== 'string' || typeof expectedOrigin !== 'string') return false;
  try {
    return new URL(origin).origin === expectedOrigin && origin === expectedOrigin;
  } catch {
    return false;
  }
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
