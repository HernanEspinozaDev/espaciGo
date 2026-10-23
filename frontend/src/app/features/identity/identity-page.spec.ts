import { HttpErrorResponse, provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter, Router } from '@angular/router';
import type { UrlTree } from '@angular/router';
import axe from 'axe-core';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { TermsResponse } from '../../core/contracts/models';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { IdentityPage } from './identity-page';

describe('IdentityPage private forms', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('matches the identity story to registration and shows current consent links', async () => {
    const item = (id: string, type: 'terminos' | 'privacidad'): TermsResponse['items'][number] => ({
      id,
      code: id,
      type,
      sha256: 'synthetic-sha',
      published_at: '2026-10-01T00:00:00Z',
      synthetic: true,
      document: {
        id,
        code: id,
        type,
        title: type === 'terminos' ? 'Términos de ensayo' : 'Privacidad de ensayo',
        scope: 'Cuenta local',
        status: 'vigente',
        sections: [{ heading: 'Contenido sintético', body: 'Texto disponible en ensayo.' }],
      },
    });
    const api = { terms: vi.fn().mockResolvedValue({
      items: [item('terms-current', 'terminos'), item('privacy-current', 'privacidad')],
    }) };
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
              data: { identityMode: 'register' },
              queryParamMap: convertToParamMap({}),
            },
          },
        },
        { provide: IdentityApi, useValue: api },
        PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage);
    fixture.detectChanges();
    await vi.waitFor(() => expect(api.terms).toHaveBeenCalledOnce());
    await vi.waitFor(() => {
      fixture.detectChanges();
      expect(fixture.nativeElement.querySelector('.terms-fieldset')).not.toBeNull();
    });

    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('.story-main h2')?.textContent).toContain('Haz lugar');
    expect(root.querySelector('a[href^="/terminos-y-condiciones"]')?.getAttribute('target')).toBe('_blank');
    expect(root.querySelector('a[href^="/politica-de-privacidad"]')?.getAttribute('target')).toBe('_blank');
    expect(root.querySelectorAll('.consent-row input[type="checkbox"]')).toHaveLength(2);
    fixture.destroy();
  });

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
    expect(page.errorMessage()).toContain('Iniciaste sesión');
    expect(page.successMessage()).toBe('');
    expect(fixture.nativeElement.querySelector('[role="status"]')).toBeNull();
    expect(fixture.nativeElement.querySelector('#login-password-error')).toBeNull();
    expect(fixture.nativeElement.querySelector('[role="alert"]')?.textContent).toContain(
      'Iniciaste sesión',
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

  it('announces verified email on the login page', async () => {
    TestBed.configureTestingModule({
      imports: [IdentityPage],
      providers: [
        ...provideFrontendPorts(), provideHttpClient(), provideRouter([]),
        { provide: ActivatedRoute, useValue: { snapshot: { data: { identityMode: 'login' }, queryParamMap: convertToParamMap({ verificado: '1' }) } } },
        { provide: IdentityApi, useValue: {} }, PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('[role="status"]')?.textContent).toContain('Correo verificado. Ya puedes iniciar sesión.');
    fixture.destroy();
  });

  it.each([
    { returnUrl: '/espacios/550e8400-e29b-41d4-a716-446655440000?action=reserve&returnTo=%2Fexplorar', preserve: true },
    { returnUrl: 'https://untrusted.example.invalid/', preserve: false },
  ])('verification preserves only a validated pending return: $preserve', async ({ returnUrl, preserve }) => {
    const api = { verifyEmail: vi.fn().mockResolvedValue(undefined) };
    TestBed.configureTestingModule({
      imports: [IdentityPage],
      providers: [
        ...provideFrontendPorts(), provideHttpClient(), provideRouter([]),
        { provide: ActivatedRoute, useValue: { snapshot: { data: { identityMode: 'verify' }, queryParamMap: convertToParamMap({ returnUrl }) } } },
        { provide: IdentityApi, useValue: api }, PrivateSessionState,
      ],
    });
    const fixture = TestBed.createComponent(IdentityPage);
    fixture.detectChanges();
    const navigate = vi.spyOn(TestBed.inject(Router), 'navigateByUrl').mockResolvedValue(true);
    const page = fixture.componentInstance as unknown as {
      tokenModel: { set(value: { tokenId: string; token: string }): void };
      onVerify(event: SubmitEvent): Promise<void>;
    };
    page.tokenModel.set({ tokenId: 'synthetic-token-id', token: 'synthetic-verification-token' });
    await page.onVerify(new Event('submit') as SubmitEvent);
    expect(api.verifyEmail).toHaveBeenCalledWith('synthetic-token-id', 'synthetic-verification-token');
    const destination = new URL(navigate.mock.calls[0][0].toString(), 'http://localhost');
    expect(destination.pathname).toBe('/acceso');
    expect(destination.searchParams.get('verificado')).toBe('1');
    expect(destination.searchParams.get('returnUrl')).toBe(preserve ? returnUrl : null);
    fixture.destroy();
  });

  it.each([
    { mode: 'recovery', fragment: 'token_id=id&token=synthetic-secret', expected: 'ready', external: false },
    { mode: 'verify', fragment: 'token_id=id&token=synthetic-secret', expected: 'ready', external: true },
    { mode: 'recovery', fragment: 'token_id=id&token=one&token=two', expected: 'invalid', external: true },
  ])('consumes and scrubs $mode fragment with state $expected', ({ mode, fragment, expected, external }) => {
    const safeReturn = '/espacios/550e8400-e29b-41d4-a716-446655440000?action=reserve&returnTo=%2Fexplorar';
    TestBed.configureTestingModule({ imports: [IdentityPage], providers: [
      ...provideFrontendPorts(), provideHttpClient(), provideRouter([]),
      { provide: ActivatedRoute, useValue: { snapshot: { data: { identityMode: mode }, fragment, queryParamMap: convertToParamMap({ returnUrl: external ? 'https://external.invalid/' : safeReturn, token: 'must-not-copy' }) } } },
      { provide: IdentityApi, useValue: {} }, PrivateSessionState,
    ] });
    const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
    const fixture = TestBed.createComponent(IdentityPage); fixture.detectChanges();
    const page = fixture.componentInstance as unknown as { linkState(): string };
    expect(page.linkState()).toBe(expected);
    expect(navigate).toHaveBeenCalledWith([], expect.objectContaining({ fragment: undefined, replaceUrl: true, queryParams: external ? {} : { returnUrl: safeReturn } }));
    const destination = navigate.mock.calls[0][1];
    const destinationData = JSON.stringify({ queryParams: destination?.queryParams, fragment: destination?.fragment });
    expect(destinationData).not.toContain('synthetic-secret');
    expect(destinationData).not.toContain('must-not-copy');
    expect(fixture.nativeElement.querySelector('#verify-token, #recovery-id, #recovery-token')).toBeNull();
    fixture.destroy();
  });

  it('confirms recovery after the same account bearer is revoked and private forms are cleared', async () => {
    const api = {
      login: vi.fn().mockResolvedValue({ account_id: 'recover-current', roles: [], capabilities: [], restricted_mode: false, access_token: 'current-bearer', token_type: 'Bearer', expires_at: '2027-01-01T00:00:00Z' }),
      session: vi.fn().mockResolvedValueOnce({ account_id: 'recover-current', roles: [], capabilities: [], restricted_mode: false }).mockRejectedValueOnce(new HttpErrorResponse({ status: 401 })),
      consumeRecovery: vi.fn().mockResolvedValue(undefined),
    };
    TestBed.configureTestingModule({ imports: [IdentityPage], providers: [
      ...provideFrontendPorts(), provideHttpClient(), provideRouter([]),
      { provide: ActivatedRoute, useValue: { snapshot: { data: { identityMode: 'recovery' }, queryParamMap: convertToParamMap({}), fragment: 'token_id=synthetic-id&token=synthetic-token' } } },
      { provide: IdentityApi, useValue: api }, PrivateSessionState,
    ] });
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('recovery@example.invalid', 'Synthetic#123');
    vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
    const fixture = TestBed.createComponent(IdentityPage); fixture.detectChanges();
    const page = fixture.componentInstance as unknown as {
      recoveryModel: { set(value: { tokenId: string; token: string; newPassword: string; confirmPassword: string }): void };
      onConsumeRecovery(event: SubmitEvent): Promise<void>;
      recoveryComplete(): boolean;
    };
    page.recoveryModel.set({ tokenId: 'synthetic-id', token: 'synthetic-token', newPassword: 'Changed#234', confirmPassword: 'Changed#234' });
    await page.onConsumeRecovery(new Event('submit') as SubmitEvent); fixture.detectChanges();
    expect(identity.session()).toBeNull(); expect(page.recoveryComplete()).toBe(true);
    expect(fixture.nativeElement.querySelector('h1')?.textContent).toContain('está actualizada');
    expect(fixture.nativeElement.querySelector('#recovery-password')).toBeNull();
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
