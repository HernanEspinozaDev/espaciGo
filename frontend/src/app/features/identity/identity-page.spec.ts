import { HttpErrorResponse, provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter, Router } from '@angular/router';
import type { UrlTree } from '@angular/router';
import axe from 'axe-core';
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
        capabilities: [],
        restricted_mode: false,
      }),
      session: vi.fn().mockResolvedValue({
        account_id: 'account-login',
        roles: ['arrendatario'],
        capabilities: [],
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

  it('does not mix a confirmed session with navigation failure or stale password validation', async () => {
    const api = {
      login: vi.fn().mockResolvedValue({
        account_id: 'account-login',
        roles: ['arrendatario'],
        access_token: 'login-bearer',
        token_type: 'Bearer',
        expires_at: '2026-10-12T00:00:00Z',
        capabilities: [],
        restricted_mode: false,
      }),
      session: vi.fn().mockResolvedValue({
        account_id: 'account-login',
        roles: ['arrendatario'],
        capabilities: [],
        restricted_mode: false,
      }),
      logout: vi.fn(),
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
    const router = TestBed.inject(Router);
    vi.spyOn(router, 'navigateByUrl').mockRejectedValue(new Error('route loading failed'));
    const page = fixture.componentInstance as unknown as {
      loginModel: { set(value: { email: string; password: string }): void };
      onLogin(event: SubmitEvent): Promise<void>;
      loginConfirmed(): boolean;
      errorMessage(): string;
      successMessage(): string;
    };
    page.loginModel.set({ email: 'person@example.invalid', password: 'Synthetic#123' });

    await page.onLogin(new Event('submit') as SubmitEvent);
    fixture.detectChanges();

    expect(api.login).toHaveBeenCalledOnce();
    expect(page.loginConfirmed()).toBe(true);
    expect(page.errorMessage()).toContain('Go confirmó tu sesión');
    expect(page.successMessage()).toBe('');
    expect(fixture.nativeElement.querySelector('[role="status"]')).toBeNull();
    expect(fixture.nativeElement.querySelector('#login-password-error')).toBeNull();
    expect(fixture.nativeElement.querySelector('[role="alert"]')?.textContent).toContain(
      'Go confirmó tu sesión',
    );
    fixture.destroy();
  });

  it('validates email and password before sending a login request', async () => {
    const api = { login: vi.fn(), session: vi.fn(), logout: vi.fn() };
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
    const page = fixture.componentInstance as unknown as {
      loginModel: { set(value: { email: string; password: string }): void };
      onLogin(event: SubmitEvent): Promise<void>;
    };
    page.loginModel.set({ email: 'correo-invalido', password: '' });

    await page.onLogin(new Event('submit') as SubmitEvent);
    fixture.detectChanges();

    expect(api.login).not.toHaveBeenCalled();
    expect(fixture.nativeElement.querySelector('#login-email-error')?.textContent).toContain(
      'Escribe un correo válido.',
    );
    expect(fixture.nativeElement.querySelector('#login-password-error')?.textContent).toContain(
      'Escribe tu contraseña.',
    );
    expect(fixture.nativeElement.querySelector('#login-email')?.getAttribute('aria-invalid')).toBe('true');
    expect(fixture.nativeElement.querySelector('#login-password')?.getAttribute('aria-invalid')).toBe('true');
    fixture.destroy();
  });

  it('returns to the same public space after contextual login without starting a write', async () => {
    const spaceId = '550e8400-e29b-41d4-a716-446655440000';
    const listing = '/explorar?q=oficina+centro&category_code=oficina&sort=relevance';
    const context = `/espacios/${spaceId}?action=reserve&returnTo=${encodeURIComponent(listing)}`;
    const api = {
      login: vi.fn().mockResolvedValue({
        account_id: 'renter-context',
        roles: ['arrendatario'],
        access_token: 'context-bearer',
        token_type: 'Bearer',
        expires_at: '2026-10-12T00:00:00Z',
        capabilities: [],
        restricted_mode: false,
      }),
      session: vi.fn().mockResolvedValue({
        account_id: 'renter-context',
        roles: ['arrendatario'],
        capabilities: [],
        restricted_mode: false,
      }),
      logout: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [IdentityPage],
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideRouter([]),
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: {
              data: { identityMode: 'login' },
              queryParamMap: convertToParamMap({ returnUrl: context }),
            },
          },
        },
        { provide: IdentityApi, useValue: api },
        PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage);
    fixture.detectChanges();
    const router = TestBed.inject(Router);
    const navigate = vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
    const page = fixture.componentInstance as unknown as {
      loginModel: { set(value: { email: string; password: string }): void };
      onLogin(event: SubmitEvent): Promise<void>;
      identityQueryParams(): Record<string, string>;
      contextualCancelUrl(): UrlTree | null;
    };
    page.loginModel.set({ email: 'renter@example.invalid', password: 'Synthetic#123' });
    await page.onLogin(new Event('submit') as SubmitEvent);

    expect(navigate).toHaveBeenCalledWith(
      `/espacios/${spaceId}?context=returned&returnTo=${encodeURIComponent(listing)}`,
    );
    expect(page.identityQueryParams()).toEqual({ returnUrl: context });
    const cancelUrl = page.contextualCancelUrl();
    expect(cancelUrl).toBeTruthy();
    expect(router.serializeUrl(cancelUrl!)).toBe(
      '/explorar?q=oficina%20centro&category_code=oficina&sort=relevance',
    );
    expect(fixture.nativeElement.querySelector('.context-cancel')?.textContent).toContain(
      'Cancelar y volver al catálogo',
    );
    expect(fixture.nativeElement.querySelector('a[href^="/registro"]')?.getAttribute('href')).toContain(
      encodeURIComponent(context),
    );
    const audit = await axe.run(fixture.nativeElement, {
      runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'] },
    });
    expect(audit.violations.map((violation) => violation.id)).toEqual([]);
    expect(api.login).toHaveBeenCalledOnce();
    fixture.destroy();
  });

  it('rejects a contextual redirect with an external or duplicated destination', async () => {
    const api = {
      login: vi.fn().mockResolvedValue({
        account_id: 'renter-safe',
        roles: ['arrendatario'],
        access_token: 'safe-bearer',
        token_type: 'Bearer',
        expires_at: '2026-10-12T00:00:00Z',
        capabilities: [],
        restricted_mode: false,
      }),
      session: vi.fn().mockResolvedValue({
        account_id: 'renter-safe',
        roles: ['arrendatario'],
        capabilities: [],
        restricted_mode: false,
      }),
      logout: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [IdentityPage],
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideRouter([]),
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: {
              data: { identityMode: 'login' },
              queryParamMap: convertToParamMap({
                returnUrl: [
                  '/espacios/550e8400-e29b-41d4-a716-446655440000?action=reserve&returnTo=%2Fexplorar',
                  '//evil.example',
                ],
              }),
            },
          },
        },
        { provide: IdentityApi, useValue: api },
        PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage);
    fixture.detectChanges();
    const router = TestBed.inject(Router);
    const navigate = vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
    const page = fixture.componentInstance as unknown as {
      loginModel: { set(value: { email: string; password: string }): void };
      onLogin(event: SubmitEvent): Promise<void>;
      identityQueryParams(): Record<string, string>;
      contextualCancelUrl(): UrlTree | null;
    };
    page.loginModel.set({ email: 'renter@example.invalid', password: 'Synthetic#123' });
    await page.onLogin(new Event('submit') as SubmitEvent);
    expect(navigate).toHaveBeenCalledWith('/mi-cuenta');
    expect(page.identityQueryParams()).toEqual({});
    expect(page.contextualCancelUrl()).toBeNull();
    fixture.destroy();
  });

  it('preserves the legacy contract invitation return after confirmed login', async () => {
    const contractId = '123e4567-e89b-42d3-a456-426614174000';
    const api = {
      login: vi.fn().mockResolvedValue({ account_id: 'renter-invite', roles: ['arrendatario'], access_token: 'invite-bearer', token_type: 'Bearer', expires_at: '2026-10-12T00:00:00Z', capabilities: [], restricted_mode: false }),
      session: vi.fn().mockResolvedValue({ account_id: 'renter-invite', roles: ['arrendatario'], capabilities: [], restricted_mode: false }),
      logout: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [IdentityPage],
      providers: [
        ...provideFrontendPorts(), provideHttpClient(), provideRouter([]),
        { provide: ActivatedRoute, useValue: { snapshot: { data: { identityMode: 'login' }, queryParamMap: convertToParamMap({ returnUrl: `/reservas?contrato=${contractId}` }) } } },
        { provide: IdentityApi, useValue: api }, PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage); fixture.detectChanges();
    const router = TestBed.inject(Router); const navigate = vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
    const page = fixture.componentInstance as unknown as { loginModel: { set(value: { email: string; password: string }): void }; onLogin(event: SubmitEvent): Promise<void> };
    page.loginModel.set({ email: 'invite@example.invalid', password: 'Synthetic#123' });
    await page.onLogin(new Event('submit') as SubmitEvent);
    expect(navigate).toHaveBeenCalledWith(`/reservas?contrato=${contractId}`);
    fixture.destroy();
  });

  it.each([
    { label: 'restricted', roles: ['arrendatario'], capabilities: [], restricted_mode: true },
    { label: 'incompatible role', roles: ['arrendador'], capabilities: [], restricted_mode: false },
  ] as const)('returns a $label account to the contextual detail for an explanatory state', async ({ roles, restricted_mode }) => {
    const spaceId = '550e8400-e29b-41d4-a716-446655440000';
    const returnUrl = `/espacios/${spaceId}?action=reserve&returnTo=%2Fexplorar%3Fq%3Dpublico`;
    const api = {
      login: vi.fn().mockResolvedValue({ account_id: 'account-limited', roles, access_token: 'limited-bearer', token_type: 'Bearer', expires_at: '2026-10-12T00:00:00Z', restricted_mode }),
      session: vi.fn().mockResolvedValue({ account_id: 'account-limited', roles, restricted_mode }),
      logout: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [IdentityPage],
      providers: [
        ...provideFrontendPorts(), provideHttpClient(), provideRouter([]),
        { provide: ActivatedRoute, useValue: { snapshot: { data: { identityMode: 'login' }, queryParamMap: convertToParamMap({ returnUrl }) } } },
        { provide: IdentityApi, useValue: api }, PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage); fixture.detectChanges();
    const router = TestBed.inject(Router); const navigate = vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
    const page = fixture.componentInstance as unknown as { loginModel: { set(value: { email: string; password: string }): void }; onLogin(event: SubmitEvent): Promise<void> };
    page.loginModel.set({ email: 'account@example.invalid', password: 'Synthetic#123' });
    await page.onLogin(new Event('submit') as SubmitEvent);
    expect(navigate).toHaveBeenCalledWith(`/espacios/${spaceId}?context=returned&returnTo=%2Fexplorar%3Fq%3Dpublico`);
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
        capabilities: [],
        restricted_mode: false,
      }),
      session: vi.fn().mockResolvedValue({
        account_id: 'account-a',
        roles: ['arrendatario'],
        capabilities: [],
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
        capabilities: [],
        restricted_mode: false,
      }),
      session: vi
        .fn()
        .mockResolvedValueOnce({
          account_id: 'account-a',
          roles: ['arrendatario'],
          capabilities: [],
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
        capabilities: [],
        restricted_mode: false,
      }),
      session: vi.fn().mockResolvedValue({
        account_id: 'account-a',
        roles: ['arrendatario'],
        capabilities: [],
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
