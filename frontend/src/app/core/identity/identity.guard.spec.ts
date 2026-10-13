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
      restricted_mode: false,
    });
    const route = { data: { requiredRole: 'administrador' } } as unknown as ActivatedRouteSnapshot;
    const result = runInInjectionContext(TestBed.inject(EnvironmentInjector), () =>
      roleGuard(route, {} as never),
    );
    expect(TestBed.inject(Router).serializeUrl(result as UrlTree)).toBe('/mi-cuenta');
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
