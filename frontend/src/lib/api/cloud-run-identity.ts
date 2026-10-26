const tokenCache = new Map<string, { token: string; expiresAt: number }>();

export async function getCloudRunIdentityHeaders(
  audience: string | undefined,
  fetchImpl: typeof fetch = fetch,
): Promise<Record<string, string>> {
  if (!audience) return {};
  const cached = tokenCache.get(audience);
  if (cached && Date.now() < cached.expiresAt) {
    return { Authorization: `Bearer ${cached.token}` };
  }
  try {
    const metadataUrl = `http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=${encodeURIComponent(
      audience,
    )}`;
    const res = await fetchImpl(metadataUrl, {
      headers: { "Metadata-Flavor": "Google" },
      signal: AbortSignal.timeout(3000),
    });
    if (res.ok) {
      const token = await res.text();
      tokenCache.set(audience, { token, expiresAt: Date.now() + 50 * 60 * 1000 });
      return { Authorization: `Bearer ${token}` };
    }
  } catch {
    // Outside Cloud Run / local emulator fallback
  }
  return {};
}

export function clearCloudRunIdentityCacheForTests() {
  tokenCache.clear();
}
