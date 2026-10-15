import { Component } from '@angular/core';
import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { identityGuard } from '../../core/identity/identity.guard';
import { IdentityPort } from '../../core/identity/domain/ports/identity-port';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { IdentityPage } from '../identity/identity-page';
import { BookingLifecyclePort } from './domain/ports/booking-lifecycle-port';
import { BookingPort, BookingApiError } from './domain/ports/booking-port';
import { BookingsPage } from './bookings-page';

@Component({ standalone: true, template: '<h1>Detalle de reserva</h1>' })
class ReservationDestination {}

describe('contract invitation navigation', () => {
  afterEach(() => TestBed.resetTestingModule());

  function setup() {
    const tokenAccounts = new Map<string, string>();
    const identityApi = {
      login: vi.fn(async (email: string) => {
        const token = `bearer-${email}`;
        tokenAccounts.set(token, email);
        return {
          account_id: email === 'participant@example.invalid' ? 'participant' : 'third-party',
          roles: ['arrendatario'],
          access_token: token,
          token_type: 'Bearer',
          expires_at: '2026-10-13T00:00:00Z',
          capabilities: [],
          restricted_mode: false,
        };
      }),
      session: vi.fn(async (token: string) => ({
        account_id:
          tokenAccounts.get(token) === 'participant@example.invalid'
            ? 'participant'
            : 'third-party',
        roles: ['arrendatario'],
        capabilities: [],
        restricted_mode: false,
      })),
      logout: vi.fn(),
    };
    const bookingApi = {
      list: vi.fn().mockResolvedValue({ safety_notice: 'ensayo', data: { items: [] } }),
    };
    const lifecycleApi = {
      getContract: vi.fn(async (_token: string, contractId: string) => {
        if (
          contractId === '018f1c0a-1234-4a00-8a00-0123456789ab' &&
          tokenAccounts.get(_token) === 'participant@example.invalid'
        ) {
          return {
            notice: 'ensayo',
            data: { id: contractId, reservation_id: 'reservation-authorized' },
          };
        }
        throw new BookingApiError('not found', 404);
      }),
    };
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideRouter([
          { path: 'acceso', component: IdentityPage, data: { identityMode: 'login' } },
          { path: 'reservas', component: BookingsPage, canActivate: [identityGuard] },
          {
            path: 'reservas/:id',
            component: ReservationDestination,
            canActivate: [identityGuard],
          },
        ]),
        { provide: IdentityPort, useValue: identityApi },
        { provide: BookingPort, useValue: bookingApi },
        { provide: BookingLifecyclePort, useValue: lifecycleApi },
        PrivateSessionState,
      ],
    });
    return { identityApi, lifecycleApi };
  }

  it('routes the emailed contract through access and then to the Go-confirmed reservation', async () => {
    const { lifecycleApi } = setup();
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/reservas?contrato=018f1c0a-1234-4a00-8a00-0123456789ab');
    expect(TestBed.inject(IdentitySession).status()).toBe('anonymous');
    expect(TestBed.inject(IdentitySession).accessToken()).toBeNull();

    const page = harness.routeDebugElement?.componentInstance as IdentityPage;
    page.loginModel.set({ email: 'participant@example.invalid', password: 'Synthetic#123' });
    await page.onLogin(new Event('submit') as SubmitEvent);
    await harness.fixture.whenStable();
    harness.detectChanges();

    expect(lifecycleApi.getContract).toHaveBeenCalledWith(
      'bearer-participant@example.invalid',
      '018f1c0a-1234-4a00-8a00-0123456789ab',
    );
    expect(TestBed.inject(IdentitySession).session()?.account_id).toBe('participant');
    expect(TestBed.inject(IdentitySession).accessToken()).toBe(
      'bearer-participant@example.invalid',
    );
    expect(TestBed.inject(Router).url).toBe('/reservas/reservation-authorized');
    expect(harness.routeNativeElement?.textContent).toContain('Detalle de reserva');
  });

  it('leaves a third party on the inbox and shows the non-disclosing Go denial', async () => {
    const { lifecycleApi } = setup();
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/acceso');
    const page = harness.routeDebugElement?.componentInstance as IdentityPage;
    page.loginModel.set({ email: 'other@example.invalid', password: 'Synthetic#123' });
    await page.onLogin(new Event('submit') as SubmitEvent);
    await harness.fixture.whenStable();

    const router = TestBed.inject(Router);
    await router.navigateByUrl('/reservas?contrato=018f1c0a-1234-4a00-8a00-0123456789ab');
    await harness.fixture.whenStable();
    harness.detectChanges();

    expect(lifecycleApi.getContract).toHaveBeenCalledWith(
      'bearer-other@example.invalid',
      '018f1c0a-1234-4a00-8a00-0123456789ab',
    );
    expect(router.url).toBe('/reservas?contrato=018f1c0a-1234-4a00-8a00-0123456789ab');
    expect(harness.routeNativeElement?.textContent).toContain(
      'No fue posible abrir esta invitación con esta cuenta.',
    );
    expect(TestBed.inject(IdentitySession).session()?.account_id).toBe('third-party');
  });
});
