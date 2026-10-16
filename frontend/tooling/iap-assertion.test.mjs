import { generateKeyPairSync, sign } from 'node:crypto';
import test, { afterEach, describe } from 'node:test';
import assert from 'node:assert/strict';
import { clearIapKeyCacheForTests, verifyIapAssertion } from '../src/iap-assertion.mjs';

const audience = '/projects/123/locations/us-central1/services/espacigo-private';
const now = Date.parse('2026-10-15T12:00:00Z');
const { privateKey, publicKey } = generateKeyPairSync('ec', { namedCurve: 'P-256' });
const publicPem = publicKey.export({ type: 'spki', format: 'pem' });

afterEach(() => clearIapKeyCacheForTests());

describe('IAP assertion verification', () => {
  test('accepts a correctly signed assertion with the configured audience and time claims', async () => {
    const token = signedToken({ iss: 'https://cloud.google.com/iap', aud: audience, iat: now / 1000, exp: now / 1000 + 300 });
    assert.equal(await verifyIapAssertion(token, audience, { now, fetchImpl: keysResponse }), true);
  });

  for (const [name, claims] of [
    ['wrong issuer', { iss: 'https://attacker.invalid' }],
    ['wrong audience', { aud: '/projects/123/locations/us-central1/services/other' }],
    ['array audience', { aud: [audience] }],
    ['expired', { exp: now / 1000 - 60 }],
    ['future issued-at', { iat: now / 1000 + 60 }],
    ['overlong lifetime', { exp: now / 1000 + 661 }],
  ]) test(`rejects ${name} claims`, async () => {
    const token = signedToken({ iss: 'https://cloud.google.com/iap', aud: audience, iat: now / 1000, exp: now / 1000 + 300, ...claims });
    assert.equal(await verifyIapAssertion(token, audience, { now, fetchImpl: keysResponse }), false);
  });

  test('rejects altered signatures, wrong audiences, and unavailable key configuration', async () => {
    const claims = { iss: 'https://cloud.google.com/iap', aud: audience, iat: now / 1000, exp: now / 1000 + 300 };
    const token = signedToken(claims);
    const altered = token.split('.');
    altered[2] = Buffer.alloc(64, 7).toString('base64url');
    assert.equal(await verifyIapAssertion(altered.join('.'), audience, { now, fetchImpl: keysResponse }), false);
    assert.equal(await verifyIapAssertion(token, `${audience}/wrong`, { now, fetchImpl: keysResponse }), false);
    assert.equal(await verifyIapAssertion(signedToken(claims, { alg: 'HS256' }), audience, { now, fetchImpl: keysResponse }), false);
    assert.equal(await verifyIapAssertion(signedToken(claims, { kid: 'unknown-key' }), audience, { now, fetchImpl: keysResponse }), false);
    assert.equal(await verifyIapAssertion(token, audience, { now, fetchImpl: async () => new Response('{}'), forceRefresh: true }), false);
  });
});

function signedToken(claimOverrides, headerOverrides = {}) {
  const header = b64({ alg: 'ES256', kid: 'unit-test-key', ...headerOverrides });
  const claims = b64({ sub: 'test-user', ...claimOverrides });
  const payload = `${header}.${claims}`;
  const signature = sign('sha256', Buffer.from(payload), { key: privateKey, dsaEncoding: 'ieee-p1363' });
  return `${payload}.${signature.toString('base64url')}`;
}

function b64(value) {
  return Buffer.from(JSON.stringify(value)).toString('base64url');
}

function keysResponse() {
  return Promise.resolve(new Response(JSON.stringify({ 'unit-test-key': publicPem }), { status: 200 }));
}
