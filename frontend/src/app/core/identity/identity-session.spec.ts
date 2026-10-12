import { HttpErrorResponse } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { LoginResponse, SessionResponse } from '../../../generated/api/types.gen';
import { client } from '../../../generated/api/client.gen';
import { IdentityApi } from './identity-api';
import { IdentitySession, StaleIdentityRequest } from './identity-session';
import { PrivateSessionState } from './private-session-state';

const makeLogin = (accountId: string, token: string): LoginResponse => ({
  account_id: accountId,
  roles: ['arrendatario'],
  access_token: token,
  token_type: 'Bearer',
  expires_at: '2026-10-12T00:00:00Z',
  restricted_mode: false,
});

const makeSession = (accountId: string, restricted = false): SessionResponse => ({
  account_id: accountId,
  roles: ['arrendatario'],
  restricted_mode: restricted,
});

describe('IdentitySession', () => {
  afterEach(() => {
    client.setConfig({ auth: undefined });
    TestBed.resetTestingModule();
  });

  it('uses roles and restricted mode from Go session, not the onboarding preference', async () => {
    const api = {
      login: vi.fn().mockResolvedValue(makeLogin('account-1', 'opaque-token')),
      session: vi.fn().mockResolvedValue(makeSession('account-1', true)),
      logout: vi.fn(),
    };
    const identity = createIdentity(api);

    const session = await identity.signIn('person@example.invalid', 'Synthetic#123');

    expect(session).toEqual(makeSession('account-1', true));
    expect(identity.session()?.roles).toEqual(['arrendatario']);
    expect(identity.session()?.restricted_mode).toBe(true);
    expect(identity.accessToken()).toBe('opaque-token');
    expect(identity.status()).toBe('authenticated');
  });

  it('discards a delayed login response from an earlier generation', async () => {
    let finishFirst!: (value: LoginResponse) => void;
    const firstLogin = new Promise<LoginResponse>((resolve) => (finishFirst = resolve));
    const api = {
      login: vi.fn((email: string) =>
        email === 'first@example.invalid'
          ? firstLogin
          : Promise.resolve(makeLogin('account-2', 'new-token')),
      ),
      session: vi.fn().mockResolvedValue(makeSession('account-2')),
      logout: vi.fn(),
    };
    const identity = createIdentity(api);

    const staleAttempt = identity.signIn('first@example.invalid', 'Synthetic#123');
    const currentAttempt = identity.signIn('second@example.invalid', 'Synthetic#456');
    await expect(currentAttempt).resolves.toEqual(makeSession('account-2'));
    finishFirst(makeLogin('account-1', 'old-token'));
    await expect(staleAttempt).rejects.toBeInstanceOf(StaleIdentityRequest);

    expect(identity.session()?.account_id).toBe('account-2');
    expect(identity.accessToken()).toBe('new-token');
  });

  it('clears session state and private state before requesting Go logout', async () => {
    const api = {
      login: vi.fn().mockResolvedValue(makeLogin('account-1', 'logout-token')),
      session: vi.fn().mockResolvedValue(makeSession('account-1')),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    const identity = createIdentity(api);
    const privateState = TestBed.inject(PrivateSessionState);
    let cache = 'private';
    privateState.register(() => (cache = 'cleared'));
    await identity.signIn('person@example.invalid', 'Synthetic#123');

    await identity.signOut();

    expect(api.logout).toHaveBeenCalledWith('logout-token');
    expect(identity.accessToken()).toBeNull();
    expect(identity.session()).toBeNull();
    expect(cache).toBe('cleared');
    expect(identity.status()).toBe('anonymous');
  });

  it('invalidates an active session after successful password recovery', async () => {
    const api = {
      login: vi.fn().mockResolvedValue(makeLogin('account-1', 'active-token')),
      session: vi
        .fn()
        .mockResolvedValueOnce(makeSession('account-1'))
        .mockRejectedValueOnce(new HttpErrorResponse({ status: 401 })),
      logout: vi.fn(),
      consumeRecovery: vi.fn().mockResolvedValue(undefined),
    };
    const identity = createIdentity(api);
    let cleared = false;
    TestBed.inject(PrivateSessionState).register(() => (cleared = true));
    await identity.signIn('person@example.invalid', 'Synthetic#123');

    await identity.consumeRecovery('recovery-id', 'recovery-code', 'Changed#789', 'Changed#789');

    expect(api.consumeRecovery).toHaveBeenCalledWith(
      'recovery-id',
      'recovery-code',
      'Changed#789',
      'Changed#789',
    );
    expect(identity.accessToken()).toBeNull();
    expect(identity.session()).toBeNull();
    expect(identity.status()).toBe('anonymous');
    expect(cleared).toBe(true);
    expect(identity.notice()).toContain('revocó la sesión vigente');
  });

  it('preserves a newer account after an older account recovery completes late', async () => {
    let finishRecovery!: () => void;
    const pendingRecovery = new Promise<void>((resolve) => (finishRecovery = resolve));
    const api = {
      login: vi
        .fn()
        .mockResolvedValueOnce(makeLogin('account-a', 'bearer-a'))
        .mockResolvedValueOnce(makeLogin('account-b', 'bearer-b')),
      session: vi
        .fn()
        .mockResolvedValueOnce(makeSession('account-a'))
        .mockResolvedValueOnce(makeSession('account-b'))
        .mockResolvedValueOnce(makeSession('account-b', true)),
      logout: vi.fn().mockResolvedValue(undefined),
      consumeRecovery: vi.fn(() => pendingRecovery),
    };
    const identity = createIdentity(api);
    await identity.signIn('a@example.invalid', 'Synthetic#123');

    const recovery = identity.consumeRecovery(
      'recovery-a',
      'recovery-code-a',
      'Changed#789',
      'Changed#789',
    );
    await identity.signOut();
    await identity.signIn('b@example.invalid', 'Synthetic#456');
    const sessionB = identity.session();
    const tokenB = identity.accessToken();
    finishRecovery();
    await recovery;

    expect(api.session).toHaveBeenCalledTimes(3);
    expect(identity.accessToken()).toBe(tokenB);
    expect(identity.session()).toEqual(makeSession('account-b', true));
    expect(identity.session()?.account_id).toBe('account-b');
    expect(identity.notice()).toBe('');
    expect(sessionB?.account_id).toBe('account-b');
  });

  it('does not publish or navigate a stale logout after a newer login succeeds', async () => {
    let finishLogout!: () => void;
    const pendingLogout = new Promise<void>((resolve) => (finishLogout = resolve));
    const api = {
      login: vi
        .fn()
        .mockResolvedValueOnce(makeLogin('account-1', 'old-token'))
        .mockResolvedValueOnce(makeLogin('account-2', 'new-token')),
      session: vi
        .fn()
        .mockResolvedValueOnce(makeSession('account-1'))
        .mockResolvedValueOnce(makeSession('account-2')),
      logout: vi.fn(() => pendingLogout),
    };
    const identity = createIdentity(api);
    await identity.signIn('first@example.invalid', 'Synthetic#123');

    const staleLogout = identity.signOut();
    await identity.signIn('second@example.invalid', 'Synthetic#456');
    finishLogout();

    await expect(staleLogout).resolves.toBe(false);
    expect(identity.session()?.account_id).toBe('account-2');
    expect(identity.accessToken()).toBe('new-token');
    expect(identity.notice()).toBe('');
  });

  it('keeps local cleanup and reports an unconfirmed logout when Go is unavailable', async () => {
    const api = {
      login: vi.fn().mockResolvedValue(makeLogin('account-1', 'logout-token')),
      session: vi.fn().mockResolvedValue(makeSession('account-1')),
      logout: vi.fn().mockRejectedValue(new Error('network offline')),
    };
    const identity = createIdentity(api);
    await identity.signIn('person@example.invalid', 'Synthetic#123');

    await expect(identity.signOut()).resolves.toBe(true);

    expect(identity.accessToken()).toBeNull();
    expect(identity.session()).toBeNull();
    expect(identity.notice()).toContain('Go no confirmó');
  });

  it('clears a session after a current-session 401', async () => {
    const api = {
      login: vi.fn().mockResolvedValue(makeLogin('account-1', 'expired-token')),
      session: vi.fn().mockRejectedValue(new HttpErrorResponse({ status: 401 })),
      logout: vi.fn(),
    };
    const identity = createIdentity(api);

    await expect(identity.signIn('person@example.invalid', 'Synthetic#123')).rejects.toBeDefined();
    expect(identity.accessToken()).toBeNull();
    expect(identity.session()).toBeNull();
  });
});

function createIdentity(api: object): IdentitySession {
  TestBed.configureTestingModule({
    providers: [provideRouter([]), { provide: IdentityApi, useValue: api }, PrivateSessionState],
  });
  return TestBed.inject(IdentitySession);
}
