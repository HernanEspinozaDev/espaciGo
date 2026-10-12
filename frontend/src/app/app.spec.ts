import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import axe from 'axe-core';
import { IdentitySession } from './core/identity/identity-session';
import { Readiness } from './core/readiness';
import { App } from './app';

describe('App logout navigation', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('does not navigate after a logout response from an older session', async () => {
    const identity = { signOut: vi.fn().mockResolvedValue(false) };
    const readiness = { refresh: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
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
        provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: { status: signal('checking'), refresh: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('.brand')?.textContent).toContain('espaciGo');
    expect(compiled.querySelector('nav')?.textContent).toContain('Categorías');
    fixture.destroy();
  });

  it('has no WCAG A/AA axe violations in the shared shell', async () => {
    const identity = { session: signal(null), notice: signal(''), signOut: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
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

  it('announces offline state and refreshes readiness when connectivity returns', async () => {
    const identity = { session: signal(null), notice: signal(''), signOut: vi.fn() };
    const readiness = { status: signal('ready'), refresh: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
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

    window.dispatchEvent(new Event('online'));
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.offline-banner')).toBeNull();
    expect(readiness.refresh).toHaveBeenCalled();
    fixture.destroy();
  });

  it('moves focus to the new page heading after route activation', async () => {
    const identity = { session: signal(null), notice: signal(''), signOut: vi.fn() };
    TestBed.configureTestingModule({
      imports: [App],
      providers: [
        provideRouter([]),
        { provide: IdentitySession, useValue: identity },
        { provide: Readiness, useValue: { status: signal('ready'), refresh: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const heading = document.createElement('h1');
    heading.textContent = 'Nueva pantalla';
    fixture.nativeElement.querySelector('#contenido')?.append(heading);
    const component = fixture.componentInstance as unknown as { focusCurrentPageHeading(): void };

    component.focusCurrentPageHeading();
    await Promise.resolve();

    expect(heading.getAttribute('tabindex')).toBe('-1');
    expect(document.activeElement).toBe(heading);
    fixture.destroy();
  });
});
