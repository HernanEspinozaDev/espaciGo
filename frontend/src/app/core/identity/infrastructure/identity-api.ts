import { Injectable, inject } from '@angular/core';
import { M01Local } from '../../../../generated/api/sdk.gen';
import type {
  AccountCreated,
  LoginResponse,
  SessionResponse,
  TermsResponse,
} from '../../contracts/models';
import { GeneratedResult, IdentityPort } from '../domain/ports/identity-port';
import { IdentityApiError, type IdentityFieldError } from '../identity-errors';

@Injectable({ providedIn: 'root' })
export class IdentityApi extends IdentityPort {
  private readonly api = inject(M01Local);

  async terms(): Promise<TermsResponse> {
    return unwrap(await this.api.listLocalTerms(), 'body');
  }

  async register(body: {
    email: string;
    password: string;
    terms_version_ids: [string, string];
  }): Promise<AccountCreated> {
    return unwrap(await this.api.registerAccount({ body }), 'body');
  }

  async reissueVerification(email: string): Promise<void> {
    const result = await this.api.reissueVerification({
      body: { email },
    });
    unwrap(result, 'empty');
  }

  async verifyEmail(tokenId: string, token: string): Promise<void> {
    const result = await this.api.verifyEmail({
      body: { token_id: tokenId, token },
    });
    unwrap(result, 'empty');
  }

  async recover(email: string): Promise<{ status: 'accepted'; message: string }> {
    return unwrap(
      await this.api.requestPasswordRecovery({
        body: { email },
      }),
      'body',
    );
  }

  async consumeRecovery(
    tokenId: string,
    token: string,
    newPassword: string,
    confirmPassword: string,
  ): Promise<void> {
    const result = await this.api.consumePasswordRecovery({
      body: {
        token_id: tokenId,
        token,
        new_password: newPassword,
        confirm_password: confirmPassword,
      },
    });
    unwrap(result, 'empty');
  }

  async login(email: string, password: string): Promise<LoginResponse> {
    return unwrap(
      await this.api.login({
        body: { email, password },
      }),
      'body',
    );
  }

  async session(token: string): Promise<SessionResponse> {
    return unwrap(
      await this.api.currentSession({ headers: { Authorization: `Bearer ${token}` } }),
      'body',
    );
  }

  async logout(token: string): Promise<void> {
    unwrap(await this.api.logout({ headers: { Authorization: `Bearer ${token}` } }), 'empty');
  }

  async changePassword(
    token: string,
    currentPassword: string,
    newPassword: string,
    confirmPassword: string,
  ): Promise<void> {
    const result = await this.api.changePassword({
      headers: { Authorization: `Bearer ${token}` },
      body: {
        current_password: currentPassword,
        new_password: newPassword,
        confirm_password: confirmPassword,
      },
    });
    unwrap(result, 'empty');
  }
}

function unwrap<T>(result: GeneratedResult, kind: 'body' | 'empty'): T {
  if (result.error) {
    const body =
      typeof result.error === 'object' && result.error !== null && 'error' in result.error
        ? result.error.error
        : null;
    const code =
      typeof body === 'object' && body !== null && 'code' in body && typeof body.code === 'string'
        ? body.code
        : undefined;
    const details: IdentityFieldError[] = [];
    if (
      typeof body === 'object' &&
      body !== null &&
      'details' in body &&
      Array.isArray(body.details)
    ) {
      for (const detail of body.details.slice(0, 10)) {
        if (
          typeof detail === 'object' &&
          detail !== null &&
          typeof detail.field === 'string' &&
          typeof detail.code === 'string'
        ) {
          details.push({ field: detail.field, code: detail.code });
        }
      }
    }
    throw new IdentityApiError(result.response?.status ?? 0, code, details);
  }
  if (kind === 'body' && result.data === undefined) throw new IdentityApiError(0);
  return result.data as T;
}

export { type GeneratedResult } from '../domain/ports/identity-port';
