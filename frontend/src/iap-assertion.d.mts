export function verifyIapAssertion(
  token: string | undefined,
  audience: string,
  options?: { now?: number; clockSkewSeconds?: number; fetchImpl?: typeof fetch; forceRefresh?: boolean },
): Promise<boolean>;
export function clearIapKeyCacheForTests(): void;
