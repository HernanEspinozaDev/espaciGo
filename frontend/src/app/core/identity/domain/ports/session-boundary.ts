import type { SessionResponse } from '../../../contracts/models';

/** Session notifications available to adapters without importing the application implementation. */
export abstract class SessionBoundary {
  abstract readonly accessToken: () => string | null;
  abstract expireIfCurrent(token: string): boolean;
  abstract refreshSessionIfCurrent(token: string): Promise<SessionResponse | null>;
}
