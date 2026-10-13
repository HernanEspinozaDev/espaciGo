import { HttpClient, provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it } from 'vitest';
import { client } from '../../../generated/api/client.gen';
import { provideHeyApiClient } from '../../../generated/api/client/client.gen';
import { provideFrontendPorts } from '../../frontend.providers';
import type { LoginResponse, SessionResponse } from '../contracts/models';
import { IdentitySession } from './application/identity-session';
import { PrivateSessionState } from './private-session-state';

describe('IdentityApi generated SDK integration', () => {
  afterEach(() => {
    client.setConfig({ auth: undefined });
    TestBed.resetTestingModule();
  });

  it('sends login then confirms the returned bearer with Go session before publishing roles', async () => {
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideHttpClientTesting(),
        provideHeyApiClient(client),
        PrivateSessionState,
      ],
    });
    client.setConfig({
      baseUrl: 'http://localhost:8080/api/v1',
      httpClient: TestBed.inject(HttpClient),
    });
    const http = TestBed.inject(HttpTestingController);
    const identity = TestBed.inject(IdentitySession);

    const signIn = identity.signIn('person@example.invalid', 'Synthetic#123');
    await new Promise((resolve) => setTimeout(resolve, 0));
    const login = http.expectOne('http://localhost:8080/api/v1/auth/login');
    expect(login.request.method).toBe('POST');
    expect(login.request.body).toEqual({
      email: 'person@example.invalid',
      password: 'Synthetic#123',
    });
    expect(login.request.headers.has('Authorization')).toBe(false);
    login.flush({
      account_id: 'account-a',
      roles: ['arrendador'],
      access_token: 'opaque-bearer-a',
      token_type: 'Bearer',
      expires_at: '2026-10-12T00:00:00Z',
      restricted_mode: false,
    } satisfies LoginResponse);

    await new Promise((resolve) => setTimeout(resolve, 0));
    const current = http.expectOne('http://localhost:8080/api/v1/auth/session');
    expect(current.request.method).toBe('GET');
    expect(current.request.headers.get('Authorization')).toBe('Bearer opaque-bearer-a');
    current.flush({
      account_id: 'account-a',
      roles: ['arrendador'],
      restricted_mode: true,
    } satisfies SessionResponse);

    await expect(signIn).resolves.toEqual({
      account_id: 'account-a',
      roles: ['arrendador'],
      restricted_mode: true,
    });
    expect(identity.session()?.roles).toEqual(['arrendador']);
    expect(identity.session()?.restricted_mode).toBe(true);
    http.verify();
  });

  it('uses the captured in-memory token for logout after clearing the live state', async () => {
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideHttpClientTesting(),
        provideHeyApiClient(client),
        PrivateSessionState,
      ],
    });
    client.setConfig({
      baseUrl: 'http://localhost:8080/api/v1',
      httpClient: TestBed.inject(HttpClient),
    });
    const http = TestBed.inject(HttpTestingController);
    const identity = TestBed.inject(IdentitySession);

    const signIn = identity.signIn('person@example.invalid', 'Synthetic#123');
    await new Promise((resolve) => setTimeout(resolve, 0));
    http.expectOne('http://localhost:8080/api/v1/auth/login').flush({
      account_id: 'account-a',
      roles: ['arrendatario'],
      access_token: 'logout-bearer',
      token_type: 'Bearer',
      expires_at: '2026-10-12T00:00:00Z',
      restricted_mode: false,
    } satisfies LoginResponse);
    await new Promise((resolve) => setTimeout(resolve, 0));
    http.expectOne('http://localhost:8080/api/v1/auth/session').flush({
      account_id: 'account-a',
      roles: ['arrendatario'],
      restricted_mode: false,
    } satisfies SessionResponse);
    await signIn;

    const signOut = identity.signOut();
    expect(identity.accessToken()).toBeNull();
    expect(identity.session()).toBeNull();
    await new Promise((resolve) => setTimeout(resolve, 0));
    const logout = http.expectOne('http://localhost:8080/api/v1/auth/logout');
    expect(logout.request.method).toBe('POST');
    expect(logout.request.headers.get('Authorization')).toBe('Bearer logout-bearer');
    logout.flush(null, { status: 204, statusText: 'No Content' });
    await signOut;
    http.verify();
  });
});
