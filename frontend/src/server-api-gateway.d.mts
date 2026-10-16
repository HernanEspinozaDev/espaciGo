export function apiRequestAllowed(
  mode: string,
  method: string,
  requestUrl: string,
  headers?: Headers | Record<string, string | string[] | undefined>,
): boolean;
export function clientApiHeaders(
  mode: string,
  headers?: Headers | Record<string, string | string[] | undefined>,
): Headers;
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
