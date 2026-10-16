import assert from 'node:assert/strict';
import test from 'node:test';
import { apiRequestAllowed, clientApiHeaders, iapAssertionRequired, iapRequestValid, privateEntryAudienceConfigured, publicCacheControl, requestUrlWithinLimit, serviceAudienceMatches } from '../src/server-api-gateway.mjs';
import { clearCloudRunIdentityCacheForTests, cloudRunIdentityHeaders } from '../src/cloud-run-identity.mjs';

const spaceId = '5e7d9f2a-631a-4c82-9d26-791225d19412';
const imageId = '1e7d9f2a-631a-4c82-9d26-791225d19412';

test('public gateway permits only anonymous catalog GETs', () => {
  for (const path of [
    '/api/v1/public/spaces?page_size=25',
    `/api/v1/public/spaces/${spaceId}`,
    `/api/v1/public/spaces/${spaceId}/images/${imageId}`,
    '/api/v1/public/space-categories',
    '/api/v1/public/space-categories/oficina/filters',
    '/api/v1/public/locations',
  ]) {
    assert.equal(apiRequestAllowed('public', 'GET', path), true, path);
  }
  for (const [method, path, headers] of [
    ['POST', '/api/v1/public/spaces', {}],
    ['GET', '/api/v1/reservations', {}],
    ['GET', '/api/v1/auth/login', {}],
    ['GET', `/api/v1/public/spaces/${spaceId}/images/${imageId}`, { Cookie: 'session=x' }],
    ['GET', `/api/v1/public/spaces/${spaceId}`, { Authorization: 'Bearer token' }],
    ['GET', `/api/v1/public/spaces/${spaceId}`, { 'Content-Length': '12' }],
    ['GET', `/api/v1/public/spaces/${spaceId}`, { 'Transfer-Encoding': 'chunked' }],
    ['GET', `/api/v1/public/spaces/${spaceId}%2f..%2freservations`, {}],
    ['GET', '/api/v1/public/spaces/not-a-uuid', {}],
  ]) {
    assert.equal(apiRequestAllowed('public', method, path, headers), false, `${method} ${path}`);
  }
});

test('private and local gateways allow protected API operations but reject path confusion', () => {
  for (const mode of ['private', 'local']) {
    assert.equal(apiRequestAllowed(mode, 'POST', '/api/v1/reservations'), true);
    assert.equal(apiRequestAllowed(mode, 'PUT', `/api/v1/spaces/${spaceId}`), true);
    assert.equal(apiRequestAllowed(mode, 'GET', '/api/v1/public/spaces'), true);
    assert.equal(apiRequestAllowed(mode, 'GET', '/api/v1/../health/ready'), false);
    assert.equal(apiRequestAllowed(mode, 'CONNECT', '/api/v1/public/spaces'), false);
    assert.equal(apiRequestAllowed(mode, 'GET', '/health/ready'), false);
  }
  assert.equal(apiRequestAllowed('unknown', 'GET', '/api/v1/public/spaces'), false);
});

test('gateway keeps application bearer separate and never forwards client IAM or session cookies', () => {
  const client = {
    Authorization: 'Bearer espacigo-user-token',
    'X-Serverless-Authorization': 'Bearer attacker-token',
    'x-goog-iap-jwt-assertion': 'attacker-assertion',
    Cookie: 'session=private',
    Accept: 'application/json',
    'X-Request-Id': 'request-1',
  };
  const headers = clientApiHeaders('private', client);
  assert.equal(headers.get('authorization'), 'Bearer espacigo-user-token');
  assert.equal(headers.has('x-serverless-authorization'), false);
  assert.equal(headers.has('x-goog-iap-jwt-assertion'), false);
  assert.equal(headers.has('cookie'), false);
  assert.equal(headers.get('accept'), 'application/json');
  assert.equal(headers.get('x-request-id'), 'request-1');
  assert.equal(clientApiHeaders('public', client).has('authorization'), false);
});

test('shared cache is allowed only for explicitly public responses with a short bounded TTL', () => {
  assert.equal(publicCacheControl('public', 'public, max-age=60'), 'public, max-age=60');
  assert.equal(publicCacheControl('public', 'public, max-age=301'), 'private, no-store');
  assert.equal(publicCacheControl('public', 'private, max-age=60'), 'private, no-store');
  assert.equal(publicCacheControl('private', 'public, max-age=60'), 'private, no-store');
  assert.equal(publicCacheControl('local', undefined), 'private, no-store');
});

test('public and private gateway URLs have finite byte limits', () => {
  assert.equal(requestUrlWithinLimit('public', `/api/v1/public/spaces?cursor=${'a'.repeat(4050)}`), true);
  assert.equal(requestUrlWithinLimit('public', `/api/v1/public/spaces?cursor=${'a'.repeat(4100)}`), false);
  assert.equal(requestUrlWithinLimit('private', `/api/v1/documents?cursor=${'a'.repeat(16_300)}`), true);
  assert.equal(requestUrlWithinLimit('private', `/api/v1/documents?cursor=${'a'.repeat(16_400)}`), false);
});

test('Cloud Run API audience must be the HTTPS service origin without a path', () => {
  assert.equal(serviceAudienceMatches('https://api-abc.run.app/api/v1', 'https://api-abc.run.app'), true);
  assert.equal(serviceAudienceMatches('https://api-abc.run.app/api/v1', 'https://other.run.app'), false);
  assert.equal(serviceAudienceMatches('https://api-abc.run.app/api/v1', 'https://api-abc.run.app/api/v1'), false);
  assert.equal(serviceAudienceMatches('http://api-abc.run.app/api/v1', 'http://api-abc.run.app'), false);
  assert.equal(serviceAudienceMatches('https://api-abc.run.app/api/v1', 'https://user@api-abc.run.app'), false);
});

test('private entry requires IAP assertion except for non-sensitive platform probes', () => {
  assert.equal(iapAssertionRequired('private', '/reservas'), true);
  assert.equal(iapAssertionRequired('private', '/healthz'), false);
  assert.equal(iapAssertionRequired('private', '/health/ready'), false);
  assert.equal(iapAssertionRequired('public', '/reservas'), false);
  assert.equal(privateEntryAudienceConfigured('private', '/projects/123456/locations/southamerica-west1/services/espacigo-private'), true);
  assert.equal(privateEntryAudienceConfigured('private', ''), false);
  assert.equal(privateEntryAudienceConfigured('private', 'https://service.run.app'), false);
  assert.equal(privateEntryAudienceConfigured('public', ''), true);
});

test('private entry validates the signed assertion and exact configured audience', async () => {
  const validator = async (token, audience) => token === 'valid-signed-token' && audience === '/projects/123/locations/us-central1/services/private';
  const audience = '/projects/123/locations/us-central1/services/private';
  assert.equal(await iapRequestValid('private', '/reservas', undefined, audience, validator), false);
  assert.equal(await iapRequestValid('private', '/reservas', 'forged', audience, validator), false);
  assert.equal(await iapRequestValid('private', '/reservas', 'valid-signed-token', audience, validator), true);
  assert.equal(await iapRequestValid('private', '/reservas', 'valid-signed-token', `${audience}/wrong`, validator), false);
  assert.equal(await iapRequestValid('private', '/health/ready', undefined, audience, validator), true);
});

test('server identity uses the metadata audience and caches the returned token', async () => {
  clearCloudRunIdentityCacheForTests();
  const token = 'synthetic-cloud-run-id-token-'.padEnd(120, 'x');
  const audience = 'https://api.example.test';
  let calls = 0;
  const fetchImpl = async (url, init) => {
    calls += 1;
    assert.equal(new URL(url).searchParams.get('audience'), audience);
    assert.equal(new URL(url).searchParams.get('format'), 'full');
    assert.equal(new Headers(init.headers).get('Metadata-Flavor'), 'Google');
    return new Response(token, { status: 200 });
  };
  assert.deepEqual(await cloudRunIdentityHeaders(audience, fetchImpl), { 'X-Serverless-Authorization': `Bearer ${token}` });
  assert.deepEqual(await cloudRunIdentityHeaders(audience, fetchImpl), { 'X-Serverless-Authorization': `Bearer ${token}` });
  assert.equal(calls, 1);
  clearCloudRunIdentityCacheForTests();
});
