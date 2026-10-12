import { HttpErrorResponse, provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, provideRouter, Router } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { IdentityPage } from './identity-page';

describe('IdentityPage private forms', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('confirms login through the real IdentitySession and navigates to the account page', async () => {
    const api = {
      login: vi.fn().mockResolvedValue({
        account_id: 'account-login',
        roles: ['arrendatario'],
        access_token: 'login-bearer',
        token_type: 'Bearer',
        expires_at: '2026-10-12T00:00:00Z',
        restricted_mode: false,
      }),
      session: vi.fn().mockResolvedValue({
        account_id: 'account-login',
        roles: ['arrendatario'],
        restricted_mode: false,
      }),
      logout: vi.fn(),
      consumeRecovery: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [IdentityPage],
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideRouter([]),
        { provide: ActivatedRoute, useValue: { snapshot: { data: { identityMode: 'login' } } } },
        { provide: IdentityApi, useValue: api },
        PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage);
    fixture.detectChanges();
    const identity = TestBed.inject(IdentitySession);
    const router = TestBed.inject(Router);
    const navigate = vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
    const page = fixture.componentInstance as unknown as {
      loginModel: (() => { email: string; password: string }) & {
        set(value: { email: string; password: string }): void;
      };
      onLogin(event: SubmitEvent): Promise<void>;
    };
    page.loginModel.set({ email: 'person@example.invalid', password: 'Synthetic#123' });

    await page.onLogin(new Event('submit') as SubmitEvent);

    expect(api.login).toHaveBeenCalledWith('person@example.invalid', 'Synthetic#123');
    expect(identity.session()?.account_id).toBe('account-login');
    expect(identity.status()).toBe('authenticated');
    expect(navigate).toHaveBeenCalledWith('/mi-cuenta');
    fixture.destroy();
  });

  it('clears anonymous recovery credentials after Go accepts the current request', async () => {
    const api = {
      login: vi.fn(),
      session: vi.fn(),
      logout: vi.fn(),
      consumeRecovery: vi.fn().mockResolvedValue(undefined),
    };
    TestBed.configureTestingModule({
      imports: [IdentityPage],
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideRouter([]),
        { provide: ActivatedRoute, useValue: { snapshot: { data: { identityMode: 'recovery' } } } },
        { provide: IdentityApi, useValue: api },
        PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage);
    fixture.detectChanges();
    const identity = TestBed.inject(IdentitySession);
    const page = fixture.componentInstance as unknown as {
      recoveryModel: (() => {
        tokenId: string;
        token: string;
        newPassword: string;
        confirmPassword: string;
      }) & {
        set(value: {
          tokenId: string;
          token: string;
          newPassword: string;
          confirmPassword: string;
        }): void;
      };
      onConsumeRecovery(event: SubmitEvent): Promise<void>;
    };
    page.recoveryModel.set({
      tokenId: 'recovery-id',
      token: 'recovery-code',
      newPassword: 'New#12345',
      confirmPassword: 'New#12345',
    });

    await page.onConsumeRecovery(new Event('submit') as SubmitEvent);

    expect(api.consumeRecovery).toHaveBeenCalledWith(
      'recovery-id',
      'recovery-code',
      'New#12345',
      'New#12345',
    );
    expect(identity.status()).toBe('anonymous');
    expect(identity.accessToken()).toBeNull();
    expect(page.recoveryModel()).toEqual({
      tokenId: '',
      token: '',
      newPassword: '',
      confirmPassword: '',
    });
    fixture.destroy();
  });

  it('clears change-password inputs when logout cleanup runs even if Go is unavailable', async () => {
    const api = {
      login: vi.fn().mockResolvedValue({
        account_id: 'account-a',
        roles: ['arrendatario'],
        access_token: 'test-token',
        token_type: 'Bearer',
        expires_at: '2026-10-12T00:00:00Z',
        restricted_mode: false,
      }),
      session: vi.fn().mockResolvedValue({
        account_id: 'account-a',
        roles: ['arrendatario'],
        restricted_mode: false,
      }),
      logout: vi.fn().mockRejectedValue(new Error('offline')),
      consumeRecovery: vi.fn().mockResolvedValue(undefined),
    };
    TestBed.configureTestingModule({
      imports: [IdentityPage],
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideRouter([]),
        {
          provide: ActivatedRoute,
          useValue: { snapshot: { data: { identityMode: 'change-password' } } },
        },
        { provide: IdentityApi, useValue: api },
        PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage);
    fixture.detectChanges();
    const identity = TestBed.inject(IdentitySession);
    const page = fixture.componentInstance as unknown as {
      changeModel: (() => {
        currentPassword: string;
        newPassword: string;
        confirmPassword: string;
      }) & {
        set(value: { currentPassword: string; newPassword: string; confirmPassword: string }): void;
      };
    };
    await identity.signIn('person@example.invalid', 'Synthetic#123');
    page.changeModel.set({
      currentPassword: 'Old#12345',
      newPassword: 'New#12345',
      confirmPassword: 'New#12345',
    });

    await expect(identity.signOut()).resolves.toBe(true);

    expect(page.changeModel()).toEqual({
      currentPassword: '',
      newPassword: '',
      confirmPassword: '',
    });
    fixture.destroy();
  });

  it('clears the active session when the recovery form completes successfully', async () => {
    const api = {
      login: vi.fn().mockResolvedValue({
        account_id: 'account-a',
        roles: ['arrendatario'],
        access_token: 'test-token',
        token_type: 'Bearer',
        expires_at: '2026-10-12T00:00:00Z',
        restricted_mode: false,
      }),
      session: vi
        .fn()
        .mockResolvedValueOnce({
          account_id: 'account-a',
          roles: ['arrendatario'],
          restricted_mode: false,
        })
        .mockRejectedValueOnce(new HttpErrorResponse({ status: 401 })),
      logout: vi.fn(),
      consumeRecovery: vi.fn().mockResolvedValue(undefined),
    };
    TestBed.configureTestingModule({
      imports: [IdentityPage],
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideRouter([]),
        { provide: ActivatedRoute, useValue: { snapshot: { data: { identityMode: 'recovery' } } } },
        { provide: IdentityApi, useValue: api },
        PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage);
    fixture.detectChanges();
    const identity = TestBed.inject(IdentitySession);
    const page = fixture.componentInstance as unknown as {
      recoveryModel: (() => {
        tokenId: string;
        token: string;
        newPassword: string;
        confirmPassword: string;
      }) & {
        set(value: {
          tokenId: string;
          token: string;
          newPassword: string;
          confirmPassword: string;
        }): void;
      };
      onConsumeRecovery(event: SubmitEvent): Promise<void>;
    };
    await identity.signIn('person@example.invalid', 'Synthetic#123');
    page.recoveryModel.set({
      tokenId: 'recovery-id',
      token: 'recovery-code',
      newPassword: 'New#12345',
      confirmPassword: 'New#12345',
    });

    await page.onConsumeRecovery(new Event('submit') as SubmitEvent);

    expect(api.consumeRecovery).toHaveBeenCalledWith(
      'recovery-id',
      'recovery-code',
      'New#12345',
      'New#12345',
    );
    expect(identity.session()).toBeNull();
    expect(identity.accessToken()).toBeNull();
    expect(page.recoveryModel()).toEqual({
      tokenId: '',
      token: '',
      newPassword: '',
      confirmPassword: '',
    });
    fixture.destroy();
  });

  it('discards a recovery-request message that returns after logout cleanup', async () => {
    let finishRecovery!: (value: { status: 'accepted'; message: string }) => void;
    const pendingRecovery = new Promise<{ status: 'accepted'; message: string }>(
      (resolve) => (finishRecovery = resolve),
    );
    const api = {
      login: vi.fn().mockResolvedValue({
        account_id: 'account-a',
        roles: ['arrendatario'],
        access_token: 'test-token',
        token_type: 'Bearer',
        expires_at: '2026-10-12T00:00:00Z',
        restricted_mode: false,
      }),
      session: vi.fn().mockResolvedValue({
        account_id: 'account-a',
        roles: ['arrendatario'],
        restricted_mode: false,
      }),
      logout: vi.fn().mockResolvedValue(undefined),
      recover: vi.fn(() => pendingRecovery),
      consumeRecovery: vi.fn().mockResolvedValue(undefined),
    };
    TestBed.configureTestingModule({
      imports: [IdentityPage],
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideRouter([]),
        { provide: ActivatedRoute, useValue: { snapshot: { data: { identityMode: 'recovery' } } } },
        { provide: IdentityApi, useValue: api },
        PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage);
    fixture.detectChanges();
    const identity = TestBed.inject(IdentitySession);
    const page = fixture.componentInstance as unknown as {
      emailModel: (() => { email: string }) & { set(value: { email: string }): void };
      onRecover(event: SubmitEvent): Promise<void>;
      successMessage: () => string;
    };
    await identity.signIn('person@example.invalid', 'Synthetic#123');
    page.emailModel.set({ email: 'person@example.invalid' });

    const recoveryAction = page.onRecover(new Event('submit') as SubmitEvent);
    expect(api.recover).toHaveBeenCalledOnce();
    await identity.signOut();
    finishRecovery({ status: 'accepted', message: 'Solicitud aceptada.' });
    await recoveryAction;

    expect(page.successMessage()).toBe('');
    expect(identity.session()).toBeNull();
    fixture.destroy();
  });
});
