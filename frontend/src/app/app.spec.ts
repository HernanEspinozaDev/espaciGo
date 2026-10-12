import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
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
});
