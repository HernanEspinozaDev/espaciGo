const tokenCache = new Map();

export async function cloudRunIdentityHeaders(audience, fetchImpl = fetch) {
  if (!audience) return {};
  const cached = tokenCache.get(audience);
  if (cached && cached.expiresAt > Date.now() + 60_000) {
    return { 'X-Serverless-Authorization': `Bearer ${cached.value}` };
  }
  const tokenUrl = new URL('http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity');
  tokenUrl.searchParams.set('audience', audience);
  tokenUrl.searchParams.set('format', 'full');
  const response = await fetchImpl(tokenUrl, {
    headers: { 'Metadata-Flavor': 'Google' },
    redirect: 'error',
    signal: AbortSignal.timeout(3000),
  });
  if (!response.ok) throw new Error('Cloud Run identity token unavailable');
  const value = (await response.text()).trim();
  if (value.length < 100 || value.length > 16_384) throw new Error('Cloud Run identity token invalid');
  tokenCache.set(audience, { value, expiresAt: Date.now() + 45 * 60_000 });
  return { 'X-Serverless-Authorization': `Bearer ${value}` };
}

export function clearCloudRunIdentityCacheForTests() {
  tokenCache.clear();
}
