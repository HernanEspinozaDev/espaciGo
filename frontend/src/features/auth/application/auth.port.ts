import type {
  components,
  Readable,
  Writable,
} from "@/lib/api/generated/openapi";

declare const accessTokenBrand: unique symbol;

export type LoginRequest = Writable<components["schemas"]["LoginRequest"]>;
type LoginResponse = Readable<components["schemas"]["LoginResponse"]>;
type SessionResponse = Readable<components["schemas"]["SessionResponse"]>;

export type AccessToken = string & { readonly [accessTokenBrand]: true };
export type SessionRoles = LoginResponse["roles"];
export type SessionCapabilities = LoginResponse["capabilities"];

export interface PublicAuthSession {
  accountId: string;
  roles: SessionRoles;
  capabilities: SessionCapabilities;
  restrictedMode: boolean;
  expiresAt: string;
}

export interface CurrentAuthSession {
  accountId: SessionResponse["account_id"];
  roles: SessionResponse["roles"];
  capabilities: SessionResponse["capabilities"];
  restrictedMode: boolean;
}

export interface AuthenticatedRequestContext {
  readonly accessToken: AccessToken;
  readonly generation: number;
  readonly signal: AbortSignal;
}

export interface AuthPort {
  login(
    input: LoginRequest,
    signal: AbortSignal,
  ): Promise<{
    accessToken: AccessToken;
    session: PublicAuthSession;
  }>;
  currentSession(
    accessToken: AccessToken,
    signal: AbortSignal,
  ): Promise<CurrentAuthSession>;
  logout(accessToken: AccessToken, signal: AbortSignal): Promise<void>;
}
