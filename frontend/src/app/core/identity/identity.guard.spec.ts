import { EnvironmentInjector, runInInjectionContext } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRouteSnapshot, provideRouter, Router, UrlTree } from '@angular/router';
import { afterEach, describe, expect, it } from 'vitest';
import { client } from '../../../generated/api/client.gen';
import { provideFrontendPorts } from '../../frontend.providers';
import { IdentitySession } from './application/identity-session';
import { identityGuard, roleGuard } from './identity.guard';
import { IdentityApi } from './infrastructure/identity-api';
import { PrivateSessionState } from './private-session-state';

describe('identity and role guards', () => {
  afterEach(() => {
    client.setConfig({ auth: undefined });
    TestBed.resetTestingModule();
  });

  it('requires a confirmed in-memory session for private account routes', () => {
    setup();
    const result = runInInjectionContext(TestBed.inject(EnvironmentInjector), () =>
      identityGuard({} as ActivatedRouteSnapshot, {} as never),
    );
    expect(result).toBeInstanceOf(UrlTree);
    expect(TestBed.inject(Router).serializeUrl(result as UrlTree)).toBe('/acceso');
  });

  it('shows a role route only when Go confirmed that role and access is unrestricted', () => {
    setup();
    const identity = TestBed.inject(IdentitySession);
    identity.session.set({
      account_id: 'synthetic-account',
      roles: ['arrendador'],
      capabilities: [],
      restricted_mode: false,
    });
    const route = { data: { requiredRole: 'arrendador' } } as unknown as ActivatedRouteSnapshot;
    const result = runInInjectionContext(TestBed.inject(EnvironmentInjector), () =>
      roleGuard(route, {} as never),
    );
    expect(result).toBe(true);

    identity.session.set({
      account_id: 'synthetic-account',
      roles: ['arrendador'],
      capabilities: [],
      restricted_mode: true,
    });
    const restricted = runInInjectionContext(TestBed.inject(EnvironmentInjector), () =>
      roleGuard(route, {} as never),
    );
    expect(TestBed.inject(Router).serializeUrl(restricted as UrlTree)).toBe('/mi-cuenta');
  });

  it('does not grant an administrator route from the renter role', () => {
    setup();
    TestBed.inject(IdentitySession).session.set({
      account_id: 'synthetic-account',
      roles: ['arrendatario'],
      capabilities: [],
      restricted_mode: false,
    });
    const route = { data: { requiredRole: 'administrador' } } as unknown as ActivatedRouteSnapshot;
    const result = runInInjectionContext(TestBed.inject(EnvironmentInjector), () =>
      roleGuard(route, {} as never),
    );
    expect(TestBed.inject(Router).serializeUrl(result as UrlTree)).toBe('/mi-cuenta');
  });

  it('allows a verified role-free account to start the first landlord action only on the onboarding route', () => {
    setup();
    TestBed.inject(IdentitySession).session.set({
      account_id: 'synthetic-account',
      roles: [],
      capabilities: [],
      restricted_mode: false,
    });
    const firstActionRoute = {
      data: { requiredRole: 'arrendador', allowFirstBusinessAction: true },
    } as unknown as ActivatedRouteSnapshot;
    const firstAction = runInInjectionContext(TestBed.inject(EnvironmentInjector), () =>
      roleGuard(firstActionRoute, {} as never),
    );
    expect(firstAction).toBe(true);

    const laterActionRoute = {
      data: { requiredRole: 'arrendador' },
    } as unknown as ActivatedRouteSnapshot;
    const laterAction = runInInjectionContext(TestBed.inject(EnvironmentInjector), () =>
      roleGuard(laterActionRoute, {} as never),
    );
    expect(TestBed.inject(Router).serializeUrl(laterAction as UrlTree)).toBe('/mi-cuenta');
  });

  it('requires Go-confirmed accounts.list for the global directory', () => {
    setup();
    const identity = TestBed.inject(IdentitySession);
    const route = { data: { requiredCapability: 'accounts.list' } } as unknown as ActivatedRouteSnapshot;
    identity.session.set({ account_id: 'synthetic-account', roles: ['administrador_global'], capabilities: [], restricted_mode: false });
    let result = runInInjectionContext(TestBed.inject(EnvironmentInjector), () => roleGuard(route, {} as never));
    expect(TestBed.inject(Router).serializeUrl(result as UrlTree)).toBe('/mi-cuenta');
    identity.session.set({ account_id: 'synthetic-account', roles: ['administrador_global'], capabilities: ['accounts.list'], restricted_mode: false });
    result = runInInjectionContext(TestBed.inject(EnvironmentInjector), () => roleGuard(route, {} as never));
    expect(result).toBe(true);
  });
});

function setup(): void {
  TestBed.configureTestingModule({
    providers: [
      ...provideFrontendPorts(),
      provideRouter([]),
      { provide: IdentityApi, useValue: {} },
      PrivateSessionState,
    ],
  });
}
