import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { Component, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, provideRouter, Router } from '@angular/router';
import axe from 'axe-core';
import { map } from 'rxjs/operators';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { App } from './app';
import type { LoginResponse, SessionResponse } from './core/contracts/models';
import { IdentitySession } from './core/identity/application/identity-session';
import { IdentityApi } from './core/identity/infrastructure/identity-api';
import { Readiness } from './core/readiness';
import { provideFrontendPorts } from './frontend.providers';

@Component({
  selector: 'test-reservation-route',
  template: '<h1>Reserva {{ id() }}</h1><button>Acción</button>',
})
class TestReservationRoute {
  readonly route = inject(ActivatedRoute);
  readonly id = toSignal(this.route.paramMap.pipe(map((params) => params.get('id'))), {
    initialValue: '',
  });
}

describe('App logout navigation', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('does not navigate after a logout response from an older session', async () => {
    const identity = { signOut: vi.fn().mockResolvedValue(false) };
    const readiness = { refresh: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: readiness },
      ],
    });
    const fixture = TestBed.createComponent(App);
    const router = TestBed.inject(Router);
    const navigate = vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
    const component = fixture.componentInstance as unknown as { signOut(): Promise<void> };

    await component.signOut();

    expect(identity.signOut).toHaveBeenCalledOnce();
    expect(navigate).not.toHaveBeenCalled();
    fixture.destroy();
  });

  it('navigates away after local logout cleanup when Go cannot confirm revocation', async () => {
    const identity = { signOut: vi.fn().mockResolvedValue(true) };
    const readiness = { refresh: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: readiness },
      ],
    });
    const fixture = TestBed.createComponent(App);
    const router = TestBed.inject(Router);
    const navigate = vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
    const component = fixture.componentInstance as unknown as { signOut(): Promise<void> };

    await component.signOut();

    expect(identity.signOut).toHaveBeenCalledOnce();
    expect(navigate).toHaveBeenCalledWith('/acceso');
    fixture.destroy();
  });

  it('creates the app shell', () => {
    const identity = { session: signal(null), notice: signal(''), signOut: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: { status: signal('checking'), refresh: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(App);
    expect(fixture.componentInstance).toBeTruthy();
    fixture.destroy();
  });

  it('renders the EspaciGo shell and primary navigation', async () => {
    const identity = { session: signal(null), notice: signal(''), signOut: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: { status: signal('checking'), refresh: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const compiled = fixture.nativeElement as HTMLElement;
    const brand = compiled.querySelector<HTMLAnchorElement>('.brand');
    const brandParts = [...(brand?.children ?? [])].map((part) =>
      part instanceof HTMLImageElement ? part.getAttribute('src') : part.textContent?.trim(),
    );
    expect(brand?.getAttribute('aria-label')).toBe('EspaciGo, ir a la portada');
    expect(brandParts).toEqual(['/espacigo-mark.svg', 'EspaciGo']);
    expect(compiled.querySelector('nav')?.textContent).toContain('Categorías');
    expect(compiled.querySelector('.nav-sign-in')?.textContent).toContain('Iniciar sesión');
    expect(compiled.querySelector('[role="dialog"]')).toBeNull();
    fixture.destroy();
  });

  it('groups authenticated navigation without exposing unpublished account roles', async () => {
    const identity = {
      session: signal<SessionResponse>({
        account_id: 'synthetic-renter',
        roles: ['arrendatario'],
        capabilities: [],
        restricted_mode: false,
      }),
      notice: signal(''),
      signOut: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: { status: signal('ready'), refresh: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const navigation = fixture.nativeElement.querySelector('#workspace-navigation') as HTMLElement;

    expect(navigation.textContent).toContain('Área personal');
    expect(navigation.querySelector('a[href="/panel/espacios"]')?.textContent).toContain('Ofrecer espacio');
    expect(navigation.querySelector('a[href="/reservas"]')?.textContent).toContain('Reservas');
    expect(navigation.querySelector('a[href="/mi-cuenta/verificaciones"]')).toBeTruthy();
    expect(navigation.querySelector('a[href="/administracion"]')).toBeNull();
    expect(navigation.querySelector('a[href="/administracion/global"]')).toBeNull();
    fixture.destroy();
  });

  it('has no WCAG A/AA axe violations in the authenticated shell', async () => {
    const identity = {
      session: signal<SessionResponse>({
        account_id: 'synthetic-host',
        roles: ['arrendatario', 'arrendador'],
        capabilities: [],
        restricted_mode: false,
      }),
      notice: signal(''),
      signOut: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: { status: signal('ready'), refresh: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const result = await axe.run(fixture.nativeElement, {
      runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'] },
    });

    expect(result.violations.map(({ id }) => id)).toEqual([]);
    fixture.destroy();
  });

  it('has no WCAG A/AA axe violations in the shared shell', async () => {
    const identity = { session: signal(null), notice: signal(''), signOut: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: { status: signal('ready'), refresh: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const result = await axe.run(fixture.nativeElement, {
      runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'] },
    });

    expect(
      result.violations.map(({ id, nodes }) => ({
        id,
        targets: nodes.map(({ target }) => target),
      })),
    ).toEqual([]);
    fixture.destroy();
  });

  it('closes the mobile navigation with Escape and returns focus to its trigger', async () => {
    const identity = { session: signal(null), notice: signal(''), signOut: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: { status: signal('ready'), refresh: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    fixture.detectChanges();
    const button = (fixture.nativeElement as HTMLElement).querySelector<HTMLButtonElement>(
      '#main-menu-toggle',
    )!;
    button.style.display = 'inline-grid';
    button.focus();
    button.click();
    fixture.detectChanges();
    expect(button.getAttribute('aria-expanded')).toBe('true');

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    fixture.detectChanges();

    expect(button.getAttribute('aria-expanded')).toBe('false');
    expect(document.activeElement).toBe(button);
    fixture.destroy();
  });

  it('offers the first publication to a renter and shows hosting after Go confirms both roles', async () => {
    const identity = {
      session: signal<SessionResponse>({ account_id: 'renter-first', roles: ['arrendatario'], capabilities: [], restricted_mode: false }),
      notice: signal(''), signOut: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(), provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: { status: signal('ready'), refresh: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const link = () => (fixture.nativeElement as HTMLElement).querySelector<HTMLAnchorElement>('a[href="/panel/espacios"]');
    expect(link()?.textContent?.trim()).toBe('Ofrecer espacio');
    identity.session.set({ account_id: 'renter-first', roles: ['arrendatario', 'arrendador'], capabilities: [], restricted_mode: false });
    fixture.detectChanges();
    expect(link()?.textContent?.trim()).toBe('Mis espacios');
    identity.session.update((session) => ({ ...session, restricted_mode: true }));
    fixture.detectChanges();
    expect(link()).toBeNull();
    fixture.destroy();
  });

  it('announces offline state and refreshes readiness when connectivity returns', async () => {
    const activeSession = {
      account_id: 'offline-account',
      roles: ['arrendatario'],
      capabilities: [],
      restricted_mode: false,
    };
    const identity = { session: signal(activeSession), notice: signal(''), signOut: vi.fn() };
    const readiness = { status: signal('ready'), refresh: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: readiness },
      ],
    });
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    window.dispatchEvent(new Event('offline'));
    fixture.detectChanges();
    expect(
      fixture.nativeElement.querySelector('[role="status"].offline-banner')?.textContent,
    ).toContain('Sin conexión');
    expect(identity.session()).toBe(activeSession);

    window.dispatchEvent(new Event('online'));
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.offline-banner')).toBeNull();
    expect(readiness.refresh).toHaveBeenCalled();
    expect(identity.session()).toBe(activeSession);
    fixture.destroy();
  });

  it('moves focus after a real navigation and reused reservation parameter change', async () => {
    const identity = { session: signal(null), notice: signal(''), signOut: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([{ path: 'reservas/:id', component: TestReservationRoute }]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: { status: signal('ready'), refresh: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(App);
    const router = TestBed.inject(Router);
    await fixture.whenStable();
    await router.navigateByUrl('/reservas/a');
    fixture.detectChanges();
    await fixture.whenStable();
    const firstRoute = fixture.nativeElement.querySelector('test-reservation-route');
    const heading = fixture.nativeElement.querySelector('#contenido h1') as HTMLElement;
    expect(heading.textContent).toContain('a');
    expect(document.activeElement).toBe(heading);

    (
      fixture.nativeElement.querySelector('test-reservation-route button') as HTMLButtonElement
    ).focus();
    await router.navigateByUrl('/reservas/b');
    fixture.detectChanges();
    await fixture.whenStable();

    expect(fixture.nativeElement.querySelector('test-reservation-route')).toBe(firstRoute);
    expect(heading.textContent).toContain('b');
    expect(document.activeElement).toBe(heading);
    fixture.destroy();
  });

  it('sets aria-current page on only the exact nested destination', async () => {
    const identity = {
      session: signal({ roles: ['administrador', 'arrendador', 'arrendatario'], capabilities: [], restricted_mode: false }),
      notice: signal(''),
      signOut: vi.fn(),
    };
    @Component({ standalone: true, template: '<h1>Destino</h1>' })
    class Destination {}
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([
          { path: 'administracion', component: Destination },
          { path: 'administracion/disputas', component: Destination },
          { path: 'panel/espacios/:spaceId/editar', component: Destination },
          { path: 'mi-cuenta', component: Destination },
          { path: 'mi-cuenta/privacidad', component: Destination },
        ]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: { status: signal('ready'), refresh: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(App);
    const router = TestBed.inject(Router);
    await router.navigateByUrl('/administracion/disputas');
    fixture.detectChanges();
    await fixture.whenStable();
    let currentPages = fixture.nativeElement.querySelectorAll('#workspace-navigation [aria-current="page"]');
    expect(currentPages).toHaveLength(1);
    expect(currentPages[0].textContent).toContain('Disputas y finanzas');
    const adminLocation = fixture.nativeElement.querySelector('[aria-current="location"]');
    expect(adminLocation?.textContent).toContain('Cola operativa');

    await router.navigateByUrl('/mi-cuenta/privacidad');
    fixture.detectChanges();
    await fixture.whenStable();
    currentPages = fixture.nativeElement.querySelectorAll('#workspace-navigation [aria-current="page"]');
    expect(currentPages).toHaveLength(0);
    const accountLocation = fixture.nativeElement.querySelector('[aria-current="location"]');
    expect(accountLocation?.textContent).toContain('Perfil y privacidad');

    await router.navigateByUrl('/panel/espacios/synthetic-space-id/editar');
    fixture.detectChanges();
    await fixture.whenStable();
    const hostingLocation = fixture.nativeElement.querySelector('[aria-current="location"]');
    const breadcrumb = fixture.nativeElement.querySelector('.breadcrumb') as HTMLElement;
    expect(hostingLocation?.textContent).toContain('Mis espacios');
    expect(breadcrumb.textContent).toContain('Área personal');
    expect(breadcrumb.textContent).toContain('Mis espacios');
    expect(breadcrumb.textContent).not.toContain('synthetic-space-id');
    fixture.destroy();
  });
});

describe('App API outage recovery with a real identity session', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('keeps the in-memory session through Go downtime and readiness restoration', async () => {
    const login: LoginResponse = {
      account_id: 'offline-account',
      access_token: 'opaque-in-memory-token',
      token_type: 'Bearer',
      expires_at: '2026-10-13T00:00:00Z',
      roles: ['arrendatario'],
      capabilities: [],
      restricted_mode: false,
    };
    const session: SessionResponse = {
      account_id: 'offline-account',
      roles: ['arrendatario'],
      capabilities: [],
      restricted_mode: false,
    };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        ...provideFrontendPorts(),
        provideRouter([]),
        provideHttpClient(),
        provideHttpClientTesting(),
        {
          provide: IdentityApi,
          useValue: {
            login: vi.fn().mockResolvedValue(login),
            session: vi.fn().mockResolvedValue(session),
          },
        },
      ],
    });
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('offline@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(App);
    const http = TestBed.inject(HttpTestingController);
    await fixture.whenStable();

    http.expectOne('/config.json').flush({ apiBaseUrl: 'http://localhost:8080/api/v1', siteOrigin: 'http://localhost:4000' });
    const outageCheck = await vi.waitFor(() =>
      http.expectOne('http://localhost:8080/health/ready'),
    );
    outageCheck.error(new ProgressEvent('error'));
    await fixture.whenStable();
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Servicio no disponible');
    expect(identity.status()).toBe('authenticated');
    expect(identity.session()).toEqual(session);
    expect(identity.accessToken()).toBe('opaque-in-memory-token');

    window.dispatchEvent(new Event('online'));
    const restoredCheck = await vi.waitFor(() =>
      http.expectOne('http://localhost:8080/health/ready'),
    );
    restoredCheck.flush({ status: 'ready' });
    await fixture.whenStable();
    expect(identity.session()).toEqual(session);
    expect(identity.accessToken()).toBe('opaque-in-memory-token');
    http.verify();
    fixture.destroy();
  });
});
