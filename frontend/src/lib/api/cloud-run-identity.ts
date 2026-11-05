const tokenCache = new Map<string, { token: string; expiresAt: number }>();

export function getTargetAudience(baseUrlOrigin?: string): string | undefined {
  return process.env.API_SERVICE_AUDIENCE?.trim() || baseUrlOrigin;
}

export async function getCloudRunIdentityHeaders(
  audience: string | undefined,
  fetchImpl: typeof fetch = fetch,
): Promise<Record<string, string>> {
  if (!audience) return {};
  const cached = tokenCache.get(audience);
  if (cached && Date.now() < cached.expiresAt) {
    return {
      Authorization: `Bearer ${cached.token}`,
      "X-Serverless-Authorization": `Bearer ${cached.token}`,
    };
  }
  try {
    const metadataUrl = new URL(
      "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity",
    );
    metadataUrl.searchParams.set("audience", audience);
    metadataUrl.searchParams.set("format", "full");
    const res = await fetchImpl(metadataUrl.toString(), {
      headers: { "Metadata-Flavor": "Google" },
      signal: AbortSignal.timeout(3000),
    });
    if (res.ok) {
      const token = (await res.text()).trim();
      if (token.length >= 100 && token.length <= 16384) {
        tokenCache.set(audience, {
          token,
          expiresAt: Date.now() + 45 * 60 * 1000,
        });
        return {
          Authorization: `Bearer ${token}`,
          "X-Serverless-Authorization": `Bearer ${token}`,
        };
      }
    }
  } catch {
    // Outside Cloud Run / local emulator fallback
  }
  return {};
}

function isLoopbackHost(hostname: string): boolean {
  const h = hostname.toLowerCase();
  return (
    h === "localhost" || h === "[::1]" || h === "::1" || h.startsWith("127.")
  );
}

export async function getCloudRunIdentityHeadersForBackend(
  backend: URL,
  fetchImpl: typeof fetch = fetch,
): Promise<Record<string, string>> {
  if (isLoopbackHost(backend.hostname)) {
    return {};
  }
  return getCloudRunIdentityHeaders(
    getTargetAudience(backend.origin),
    fetchImpl,
  );
}

export function clearCloudRunIdentityCacheForTests() {
  tokenCache.clear();
}
