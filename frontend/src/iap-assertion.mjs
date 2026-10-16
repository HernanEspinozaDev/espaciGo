import { createPublicKey, verify } from 'node:crypto';

const KEY_URL = 'https://www.gstatic.com/iap/verify/public_key';
let cachedKeys = null;
let cachedUntil = 0;

export async function verifyIapAssertion(token, audience, options = {}) {
  try {
    if (typeof token !== 'string' || token.length < 80 || token.length > 16_384 || !audience) return false;
    const parts = token.split('.');
    if (parts.length !== 3) return false;
    const header = parsePart(parts[0]);
    const claims = parsePart(parts[1]);
    if (header.alg !== 'ES256' || typeof header.kid !== 'string' || !header.kid) return false;
    if (claims.iss !== 'https://cloud.google.com/iap') return false;
    if (claims.aud !== audience) return false;
    if (typeof claims.exp !== 'number' || typeof claims.iat !== 'number') return false;
    const now = (options.now ?? Date.now()) / 1000;
    const skew = options.clockSkewSeconds ?? 30;
    if (claims.exp < now - skew || claims.iat > now + skew || claims.exp <= claims.iat || claims.exp - claims.iat > 660) return false;

    const fetchImpl = options.fetchImpl ?? fetch;
    let keys = await loadKeys(fetchImpl, options.forceRefresh === true);
    if (!keys[header.kid]) keys = await loadKeys(fetchImpl, true);
    const material = keys[header.kid];
    if (!material) return false;
    const key = typeof material === 'string'
      ? createPublicKey(material)
      : createPublicKey({ key: material, format: 'jwk' });
    const signature = Buffer.from(parts[2], 'base64url');
    if (signature.length !== 64) return false;
    return verify('sha256', Buffer.from(`${parts[0]}.${parts[1]}`), { key, dsaEncoding: 'ieee-p1363' }, signature);
  } catch {
    return false;
  }
}

async function loadKeys(fetchImpl, forceRefresh) {
  if (!forceRefresh && cachedKeys && cachedUntil > Date.now()) return cachedKeys;
  const response = await fetchImpl(KEY_URL, { redirect: 'error', signal: AbortSignal.timeout(3000) });
  if (!response.ok) throw new Error('IAP public keys unavailable');
  const body = await response.json();
  if (!body || typeof body !== 'object' || Array.isArray(body)) throw new Error('IAP public keys invalid');
  const keys = {};
  for (const [kid, value] of Object.entries(body)) {
    if (typeof value === 'string' && value.includes('BEGIN PUBLIC KEY')) keys[kid] = value;
    else if (value && typeof value === 'object' && value.kty === 'EC' && value.crv === 'P-256') keys[kid] = value;
  }
  if (!Object.keys(keys).length) throw new Error('IAP public keys invalid');
  cachedKeys = keys;
  cachedUntil = Date.now() + 5 * 60_000;
  return keys;
}

function parsePart(value) {
  const parsed = JSON.parse(Buffer.from(value, 'base64url').toString('utf8'));
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('invalid JWT');
  return parsed;
}

export function clearIapKeyCacheForTests() {
  cachedKeys = null;
  cachedUntil = 0;
}
