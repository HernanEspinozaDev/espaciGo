import type {
  components,
  Readable,
  Writable,
} from "@/lib/api/generated/openapi";

export type LoginRequest = Writable<components["schemas"]["LoginRequest"]>;
type LoginResponse = Readable<components["schemas"]["LoginResponse"]>;

export type SessionRoles = LoginResponse["roles"];
export type SessionCapabilities = LoginResponse["capabilities"];

export interface PublicAuthSession {
  accountId: string;
  roles: SessionRoles;
  capabilities: SessionCapabilities;
  restrictedMode: boolean;
  expiresAt: string;
}

export type CurrentAuthSession = PublicAuthSession;

export interface AuthLogoutResult {
  cookieCleared: boolean;
  revocationConfirmed: boolean;
}

export interface AuthPort {
  login(input: LoginRequest, signal: AbortSignal): Promise<PublicAuthSession>;
  currentSession(signal: AbortSignal): Promise<PublicAuthSession>;
  logout(signal: AbortSignal): Promise<AuthLogoutResult>;
}
