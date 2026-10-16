import { Component, signal } from '@angular/core';
import { provideRouter, Router, RouterLink } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { TestBed } from '@angular/core/testing';
import { PLATFORM_ID } from '@angular/core';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiRuntimeConfig } from '../api-runtime-config';
import { PRIVATE_ENTRY_DOCUMENT_NAVIGATOR } from './private-entry.guard';
import { buildContextualReturn } from './contextual-return';
import { routes } from '../../app.routes';

const spaceId = '8c8f85f1-c5ef-4d73-96d2-08ce0b7ac164';

@Component({ standalone: true, imports: [RouterLink], template: '<a id="detail" [routerLink]="[\'/espacios\', spaceId]">Detalle</a><a id="catalog-login" routerLink="/acceso">Ingresar</a>' })
class CatalogRoute {
  readonly spaceId = spaceId;
}

@Component({ standalone: true, imports: [RouterLink], template: '<h1>Detalle sintético</h1><a id="detail-login" routerLink="/acceso" [queryParams]="{ returnUrl }">Iniciar sesión</a>' })
class DetailRoute {
  readonly returnUrl = buildContextualReturn(spaceId, '/explorar?q=oficina&category_code=oficina');
}

@Component({ standalone: true, template: '<h1>Acceso privado</h1>' })
class LoginRoute {}

describe('private entry SPA navigation', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('uses a full document navigation when a RouterLink opens login from the catalog', async () => {
    const navigate = vi.fn();
    configure(navigate);
    const harness = await RouterTestingHarness.create('/explorar');
    harness.routeNativeElement?.querySelector<HTMLAnchorElement>('#catalog-login')?.click();
    await harness.fixture.whenStable();
    expect(navigate).toHaveBeenCalledWith('https://private.example.test/acceso');
    expect(TestBed.inject(Router).url).toBe('/explorar');
  });

  it('preserves only the validated space return context from the detail SPA route', async () => {
    const navigate = vi.fn();
    configure(navigate);
    const harness = await RouterTestingHarness.create(`/espacios/${spaceId}`);
    harness.routeNativeElement?.querySelector<HTMLAnchorElement>('#detail-login')?.click();
    await harness.fixture.whenStable();
    const target = new URL(navigate.mock.calls[0][0] as string);
    const returned = target.searchParams.get('returnUrl');
    expect(target.origin).toBe('https://private.example.test');
    expect(target.pathname).toBe('/acceso');
    expect(returned).toContain(`/espacios/${spaceId}?action=reserve&returnTo=`);
    expect(returned).toContain('q%3Doficina');
    expect(target.searchParams.has('token')).toBe(false);
    expect(TestBed.inject(Router).url).toBe(`/espacios/${spaceId}`);
  });
});

function configure(navigate: (url: string) => void): void {
  const testRoutes = routes.map((route) => {
    if (route.path === 'explorar') return { ...route, loadComponent: async () => CatalogRoute };
    if (route.path === 'espacios/:id') return { ...route, loadComponent: async () => DetailRoute };
    if (route.path === 'acceso') return { ...route, loadComponent: async () => LoginRoute };
    return route;
  });
  TestBed.configureTestingModule({
    providers: [
      { provide: PLATFORM_ID, useValue: 'browser' },
      { provide: ApiRuntimeConfig, useValue: { privateDemoOrigin: signal('https://private.example.test') } },
      { provide: PRIVATE_ENTRY_DOCUMENT_NAVIGATOR, useValue: navigate },
      provideRouter(testRoutes),
    ],
  });
}
