import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { BehaviorSubject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentityApi } from '../../core/identity/identity-api';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { ExploreApi } from './explore-api';
import { SpaceDetailPage } from './space-detail-page';

const session = { account_id: 'renter-1', roles: ['arrendatario'], restricted_mode: false };
const login = {
  account_id: 'renter-1',
  roles: ['arrendatario'],
  access_token: 'bearer-renter',
  token_type: 'Bearer',
  expires_at: '2027-01-01T00:00:00Z',
  restricted_mode: false,
};
const space = {
  space_id: 'space-1',
  category_code: 'oficina',
  category_name: 'Oficina',
  title: 'Sala clara',
  description: 'Espacio de trabajo sintético.',
  area_m2: 24,
  rate_unit: 'hora',
  base_price_clp: '9007199254740993',
  currency: 'CLP',
  time_zone: 'America/New_York',
  profile_version: 1,
  profile: {},
  attributes: {},
};

describe('SpaceDetailPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('confirms a real quote and invalidates it when the interval changes', async () => {
    const routeParams = new BehaviorSubject(convertToParamMap({ id: 'space-1' }));
    const first = { start_at: '2026-11-01T05:30:00Z', end_at: '2026-11-01T06:30:00Z' };
    const second = { start_at: '2026-11-01T06:30:00Z', end_at: '2026-11-01T07:30:00Z' };
    const explore = {
      detail: vi.fn().mockResolvedValue(space),
      profile: vi
        .fn()
        .mockResolvedValue({ category_code: 'oficina', schema_version: 1, attributes: [] }),
      availability: vi.fn().mockResolvedValue({
        safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL',
        data: {
          space_id: 'space-1',
          time_zone: 'America/New_York',
          rate_unit: 'hora',
          items: [first, second],
        },
      }),
      quote: vi.fn((_token: string, spaceId: string, startAt: string, endAt: string) =>
        Promise.resolve({
          safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL',
          data: {
            space_id: spaceId,
            start_at: startAt,
            end_at: endAt,
            unit_price_clp: '9007199254740993',
            subtotal_clp: '9007199254740993',
            guarantee_expected_clp: '50000',
          },
        }),
      ),
    };
    TestBed.configureTestingModule({
      imports: [SpaceDetailPage],
      providers: [
        provideHttpClient(),
        provideRouter([]),
        PrivateSessionState,
        { provide: ActivatedRoute, useValue: { paramMap: routeParams.asObservable() } },
        {
          provide: IdentityApi,
          useValue: {
            login: vi.fn().mockResolvedValue(login),
            session: vi.fn().mockResolvedValue(session),
            logout: vi.fn(),
          },
        },
        { provide: ExploreApi, useValue: explore },
      ],
    });
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('renter@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(SpaceDetailPage);
    fixture.detectChanges();
    const page = fixture.componentInstance as unknown as {
      options: () => (typeof first)[];
      selected: () => typeof first | null;
      quote: () => unknown;
      chooseInterval(value: string): void;
      createQuote(): Promise<void>;
      chooseDate(value: string): Promise<void>;
      formatInterval(interval: typeof first): string;
    };
    await vi.waitFor(() => expect(page.options()).toHaveLength(2));
    page.chooseInterval('0');
    await page.createQuote();

    expect((page.quote() as { subtotal_clp: string }).subtotal_clp).toBe('9007199254740993');
    expect(explore.quote).toHaveBeenCalledWith(
      'bearer-renter',
      'space-1',
      first.start_at,
      first.end_at,
    );
    expect(page.formatInterval(first)).not.toBe(page.formatInterval(second));

    page.chooseInterval('1');
    expect(page.quote()).toBeNull();
    await page.createQuote();
    expect((page.quote() as { start_at: string }).start_at).toBe(second.start_at);
    await page.chooseDate('2026-11-02');
    expect(page.selected()).toBeNull();
    expect(page.quote()).toBeNull();
    fixture.destroy();
  });
});
