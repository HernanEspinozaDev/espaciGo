import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { BehaviorSubject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { ExploreApi } from './infrastructure/explore-api';
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
      businessClock: vi.fn().mockResolvedValue({ data: { now: '2026-10-13T02:30:00Z' } }),
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
        ...provideFrontendPorts(),
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
      duration(): number;
    };
    await vi.waitFor(() => expect(page.options()).toHaveLength(2));
    expect(explore.availability).toHaveBeenCalledWith('bearer-renter', 'space-1', '2026-10-12', 1);
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
    page.chooseInterval('');
    expect(page.selected()).toBeNull();
    expect(page.quote()).toBeNull();
    fixture.detectChanges();
    expect(
      (fixture.nativeElement as HTMLElement)
        .querySelector('button.button')
        ?.hasAttribute('disabled'),
    ).toBe(true);
    await page.chooseDate('2026-11-02');
    expect(page.selected()).toBeNull();
    expect(page.quote()).toBeNull();
    fixture.destroy();
  });

  it('resets duration to a valid value when the routed space changes rate unit', async () => {
    const routeParams = new BehaviorSubject(convertToParamMap({ id: 'hour-space' }));
    const hourly = { ...space, space_id: 'hour-space', rate_unit: 'hora' as const };
    const daily = { ...space, space_id: 'day-space', rate_unit: 'dia' as const };
    const monthly = { ...space, space_id: 'month-space', rate_unit: 'mes' as const };
    const byId = new Map<
      string,
      { space_id: string; rate_unit: 'hora' | 'dia' | 'mes'; time_zone: string }
    >([
      [hourly.space_id, hourly],
      [daily.space_id, daily],
      [monthly.space_id, monthly],
    ]);
    const explore = {
      detail: vi.fn(async (_token: string, id: string) => byId.get(id)!),
      profile: vi
        .fn()
        .mockResolvedValue({ category_code: 'oficina', schema_version: 1, attributes: [] }),
      businessClock: vi.fn().mockResolvedValue({ data: { now: '2026-10-13T05:30:00Z' } }),
      availability: vi.fn(
        async (_token: string, spaceId: string, _date: string, duration: number) => ({
          safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL',
          data: {
            space_id: spaceId,
            time_zone: 'America/New_York',
            rate_unit: byId.get(spaceId)!.rate_unit,
            items: [],
            duration,
          },
        }),
      ),
      quote: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [SpaceDetailPage],
      providers: [
        ...provideFrontendPorts(),
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
      duration(): number;
      chooseDuration(value: string): Promise<void>;
    };
    await vi.waitFor(() => expect(explore.availability).toHaveBeenCalledTimes(1));
    await page.chooseDuration('4');
    expect(explore.availability.mock.calls.at(-1)?.[3]).toBe(4);

    routeParams.next(convertToParamMap({ id: 'day-space' }));
    await vi.waitFor(() => expect(explore.availability).toHaveBeenCalledTimes(3));
    expect(page.duration()).toBe(1);
    expect(explore.availability.mock.calls.at(-1)?.[3]).toBe(1);
    await page.chooseDuration('3');
    expect(explore.availability.mock.calls.at(-1)?.[3]).toBe(3);

    routeParams.next(convertToParamMap({ id: 'month-space' }));
    await vi.waitFor(() => expect(explore.availability).toHaveBeenCalledTimes(5));
    expect(page.duration()).toBe(1);
    expect(explore.availability.mock.calls.at(-1)?.[3]).toBe(1);
    fixture.destroy();
  });

  it('clears the current identity and private detail state on a current Bearer 401', async () => {
    const routeParams = new BehaviorSubject(convertToParamMap({ id: 'space-1' }));
    const explore = {
      detail: vi.fn().mockResolvedValue(space),
      profile: vi
        .fn()
        .mockResolvedValue({ category_code: 'oficina', schema_version: 1, attributes: [] }),
      businessClock: vi.fn().mockResolvedValue({ data: { now: '2026-10-13T05:30:00Z' } }),
      availability: vi.fn().mockResolvedValue({
        safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL',
        data: {
          space_id: 'space-1',
          time_zone: 'America/New_York',
          rate_unit: 'hora',
          items: [{ start_at: '2026-11-01T05:30:00Z', end_at: '2026-11-01T06:30:00Z' }],
        },
      }),
      quote: vi.fn().mockRejectedValue({ status: 401 }),
    };
    TestBed.configureTestingModule({
      imports: [SpaceDetailPage],
      providers: [
        ...provideFrontendPorts(),
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
      options(): { start_at: string; end_at: string }[];
      chooseInterval(value: string): void;
      createQuote(): Promise<void>;
      space(): unknown;
    };
    await vi.waitFor(() => expect(page.options()).toHaveLength(1));
    page.chooseInterval('0');
    await page.createQuote();
    expect(identity.accessToken()).toBeNull();
    expect(identity.session()).toBeNull();
    expect(page.space()).toBeNull();
    fixture.destroy();
  });
});
