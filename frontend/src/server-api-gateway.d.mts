export function apiRequestAllowed(
  mode: string,
  method: string,
  requestUrl: string,
  headers?: Headers | Record<string, string | string[] | undefined>,
  expectedOrigin?: string | null,
): boolean;
export function clientApiHeaders(
  mode: string,
  headers?: Headers | Record<string, string | string[] | undefined>,
  requestUrl?: string,
): Headers;
export function publicIdentityPostLimit(method: string, requestUrl: string): number | null;
export function publicIdentityNeedsClientIp(method: string, requestUrl: string): boolean;
export function publicRequestClientIp(
  headers: Headers | Record<string, string | string[] | undefined>,
  remoteAddress: string | undefined,
  behindTrustedLoadBalancer?: boolean,
): string | null;
export function publicCacheControl(mode: string, upstreamValue?: string): string;
export function requestUrlWithinLimit(mode: string, requestUrl: string): boolean;
export function serviceAudienceMatches(apiBase: string, audience: string): boolean;
export function iapAssertionRequired(mode: string, pathname: string): boolean;
export function privateEntryAudienceConfigured(mode: string, audience: string): boolean;
export function iapRequestValid(
  mode: string,
  pathname: string,
  assertion: string | undefined,
  audience: string,
  verify?: (assertion: string, audience: string) => Promise<boolean>,
): Promise<boolean>;
