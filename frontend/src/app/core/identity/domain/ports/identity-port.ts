import type {
  AccountCreated,
  LoginResponse,
  SessionResponse,
  TermsResponse,
} from '../../../contracts/models';
export type GeneratedResult = {
  data: unknown;
  error: unknown;
  response?: { status: number };
};

export abstract class IdentityPort {
  abstract terms(): Promise<TermsResponse>;
  abstract register(body: {
    email: string;
    password: string;
    use_preference: 'ofrecer' | 'arrendar';
    terms_version_ids: string[];
  }): Promise<AccountCreated>;
  abstract reissueVerification(email: string): Promise<void>;
  abstract verifyEmail(tokenId: string, token: string): Promise<void>;
  abstract recover(email: string): Promise<{ status: 'accepted'; message: string }>;
  abstract consumeRecovery(
    tokenId: string,
    token: string,
    newPassword: string,
    confirmPassword: string,
  ): Promise<void>;
  abstract login(email: string, password: string): Promise<LoginResponse>;
  abstract session(token: string): Promise<SessionResponse>;
  abstract logout(token: string): Promise<void>;
  abstract changePassword(
    token: string,
    currentPassword: string,
    newPassword: string,
    confirmPassword: string,
  ): Promise<void>;
}
