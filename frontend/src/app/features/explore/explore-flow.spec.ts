import { provideHttpClient } from '@angular/common/http';
import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { Router, RouterOutlet, provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { roleGuard } from '../../core/identity/identity.guard';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { ExplorePage } from './explore-page';
import { ExploreApi } from './infrastructure/explore-api';
import { SpaceDetailPage } from './space-detail-page';

@Component({ standalone: true, imports: [RouterOutlet], template: '<router-outlet />' })
class ExploreFlowHost {}

const item = {
  space_id: 'space-flow',
  category_code: 'oficina',
  category_name: 'Oficina',
  title: 'Sala de trabajo',
  description: 'Espacio sintético.',
  area_m2: 24,
  rate_unit: 'hora',
  base_price_clp: '9007199254740993',
  currency: 'CLP',
  time_zone: 'America/Santiago',
  profile_version: 1,
  profile: {},
  attributes: {},
};
const interval = { start_at: '2026-11-01T04:30:00Z', end_at: '2026-11-01T05:30:00Z' };
const quote = {
  id: 'quote-flow',
  space_id: item.space_id,
  rate_version: 1,
  rate_unit: 'hora',
  unit_price_clp: '9007199254740993',
  subtotal_clp: '9007199254740993',
  units: 1,
  currency: 'CLP',
  start_at: interval.start_at,
  end_at: interval.end_at,
  time_zone: item.time_zone,
  conditions: '',
  category_code: item.category_code,
  profile_version: 1,
  profile_values: {},
  cancellation_policy_version: 'local_flexible_v1',
  guarantee_policy_version: 'garantia_local_fija_v1',
  guarantee_currency: 'CLP',
  guarantee_expected_clp: '50000',
  settlement_policy_version: 'liquidacion_local_0pct_v1',
  settlement_commission_bps: 0,
  created_at: '2026-10-12T00:00:00Z',
  expires_at: '2026-10-12T00:15:00Z',
};

describe('authenticated explore to quote flow', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('routes from confirmed login through search, safe detail, IANA availability, and Go quote', async () => {
    const exploreApi = {
      categories: vi.fn().mockResolvedValue({ items: [{ code: 'oficina', name: 'Oficina' }] }),
      search: vi.fn().mockResolvedValue({
        safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL',
        data: { items: [item] },
      }),
      detail: vi.fn().mockResolvedValue(item),
      profile: vi
        .fn()
        .mockResolvedValue({ category_code: 'oficina', schema_version: 1, attributes: [] }),
      businessClock: vi.fn().mockResolvedValue({ data: { now: '2026-10-13T05:30:00Z' } }),
      availability: vi.fn().mockResolvedValue({
        safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL',
        data: {
          space_id: item.space_id,
          time_zone: item.time_zone,
          rate_unit: 'hora',
          items: [interval],
        },
      }),
      quote: vi
        .fn()
        .mockResolvedValue({ safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data: quote }),
    };
    TestBed.configureTestingModule({
      imports: [ExploreFlowHost],
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        PrivateSessionState,
        provideRouter([
          {
            path: 'explorar',
            component: ExplorePage,
            canActivate: [roleGuard],
            data: { requiredRole: 'arrendatario' },
          },
          {
            path: 'espacios/:id',
            component: SpaceDetailPage,
            canActivate: [roleGuard],
            data: { requiredRole: 'arrendatario' },
          },
        ]),
        {
          provide: IdentityApi,
          useValue: {
            login: vi.fn().mockResolvedValue({
              account_id: 'renter-flow',
              roles: ['arrendatario'],
              access_token: 'bearer-flow',
              token_type: 'Bearer',
              expires_at: '2027-01-01T00:00:00Z',
              restricted_mode: false,
            }),
            session: vi.fn().mockResolvedValue({
              account_id: 'renter-flow',
              roles: ['arrendatario'],
              restricted_mode: false,
            }),
            logout: vi.fn(),
          },
        },
        { provide: ExploreApi, useValue: exploreApi },
      ],
    });
    const fixture = TestBed.createComponent(ExploreFlowHost);
    fixture.detectChanges();
    const identity = TestBed.inject(IdentitySession);
    const router = TestBed.inject(Router);
    await identity.signIn('renter@example.invalid', 'Synthetic#123');
    await router.navigateByUrl('/explorar');
    fixture.detectChanges();

    const search = fixture.debugElement.query(By.directive(ExplorePage))
      .componentInstance as ExplorePage & {
      search(): Promise<void>;
    };
    await search.search();
    expect(exploreApi.search).toHaveBeenCalledWith(
      'bearer-flow',
      expect.objectContaining({ page_size: 8 }),
    );
    await router.navigateByUrl(`/espacios/${item.space_id}`);
    fixture.detectChanges();
    const detail = fixture.debugElement.query(By.directive(SpaceDetailPage))
      .componentInstance as SpaceDetailPage & {
      options(): (typeof interval)[];
      selected(): typeof interval | null;
      quote(): typeof quote | null;
      chooseInterval(value: string): void;
      createQuote(): Promise<void>;
    };
    await vi.waitFor(() => expect(detail.options()).toEqual([interval]));
    detail.chooseInterval('0');
    await detail.createQuote();

    expect(exploreApi.detail).toHaveBeenCalledWith('bearer-flow', item.space_id);
    expect(exploreApi.availability).toHaveBeenCalledWith(
      'bearer-flow',
      item.space_id,
      expect.any(String),
      1,
    );
    expect(exploreApi.quote).toHaveBeenCalledWith(
      'bearer-flow',
      item.space_id,
      interval.start_at,
      interval.end_at,
    );
    expect(detail.quote()?.subtotal_clp).toBe('9007199254740993');
    expect(identity.session()?.account_id).toBe('renter-flow');
    fixture.destroy();
  });
});
