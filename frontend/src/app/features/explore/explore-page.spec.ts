import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { provideFrontendPorts } from '../../frontend.providers';
import { ExplorePage } from './explore-page';
import { ExploreApi } from './infrastructure/explore-api';

const publicSpace = (id: string) => ({
  space_id: id,
  category_code: 'oficina',
  category_name: 'Oficina',
  title: id,
  description: 'Synthetic public space',
  area_m2: 20,
  rate_unit: 'hora' as const,
  base_price_clp: '6000',
  currency: 'CLP' as const,
});

describe('ExplorePage public catalog URL state', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('restores page_size and cursor from the real Router URL', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]),
      publicSearch: vi.fn().mockResolvedValue({ items: [] }),
    };
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideRouter([{ path: 'explorar', component: ExplorePage }]),
        { provide: ExploreApi, useValue: api },
      ],
    });

    const harness = await RouterTestingHarness.create(
      '/explorar?page_size=1&cursor=saved-cursor',
    );
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    expect(api.publicSearch).toHaveBeenCalledWith(
      expect.objectContaining({ page_size: 1, cursor: 'saved-cursor' }),
    );
    expect(harness.routeDebugElement!.componentInstance.items()).toEqual([]);
  });

  it('restores search, location, attributes, and interval estimates from the real Router URL', async () => {
    const api = { publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]), publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }) };
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
    });
    await RouterTestingHarness.create('/explorar?q=sala+abierta&region_code=13&comuna_code=13123&attribute=wifi%3Atrue&start_at=2026-10-15T12%3A00%3A00.000Z&end_at=2026-10-15T13%3A00%3A00.000Z&min_total_clp=9000&sort=estimated_total_asc');
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    expect(api.publicSearch).toHaveBeenCalledWith(expect.objectContaining({
      q: 'sala abierta', region_code: '13', comuna_code: '13123', attributes: { wifi: true },
      start_at: '2026-10-15T12:00:00.000Z', end_at: '2026-10-15T13:00:00.000Z', min_total_clp: '9000', sort: 'estimated_total_asc',
    }));
  });

  it.each(['success', 'error'] as const)(
    'clears category-specific attributes and ignores a late old-category filter %s', async (oldOutcome) => {
      let resolveOffice!: (value: { code: string; label: string; type: 'boolean'; profile_version: number }[]) => void;
      let rejectOffice!: (error: unknown) => void;
      const oldFilters = new Promise<{ code: string; label: string; type: 'boolean'; profile_version: number }[]>((resolve, reject) => {
        resolveOffice = resolve; rejectOffice = reject;
      });
      const api = {
        publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]),
        publicAttributeFilters: vi.fn((code: string) => code === 'oficina' ? oldFilters : Promise.resolve([
          { code: 'carro', label: 'Carro', type: 'boolean' as const, profile_version: 1 },
        ])),
        publicSearch: vi.fn().mockResolvedValue({ items: [] }),
      };
      TestBed.configureTestingModule({
        providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
      });
      const harness = await RouterTestingHarness.create('/explorar?category_code=oficina&attribute=wifi%3Atrue');
      const page = harness.routeDebugElement!.componentInstance as ExplorePage;
      await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
      page.updateFilter('category_code', 'bodega');
      expect(page.filters().attributes).toEqual({});
      expect(page.attributeFilters()).toEqual([]);
      await page.search();
      await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
      await vi.waitFor(() => expect(page.attributeFilters().map((filter) => filter.code)).toEqual(['carro']));
      expect(api.publicSearch.mock.calls[1][0].attributes).toEqual({});
      expect(TestBed.inject(Router).url).not.toContain('attribute=');
      if (oldOutcome === 'success') resolveOffice([{ code: 'wifi', label: 'Wi-Fi', type: 'boolean', profile_version: 1 }]);
      else rejectOffice(new Error('stale filter request failed'));
      await Promise.resolve();
      await Promise.resolve();
      expect(page.attributeFilters().map((filter) => filter.code)).toEqual(['carro']);
      expect(page.error()).toBe('');
    },
  );

  it('invalidates pending category filters when the selection is cleared and on destroy', async () => {
    let resolveOffice!: (value: { code: string; label: string; type: 'boolean'; profile_version: number }[]) => void;
    const officeFilters = new Promise<{ code: string; label: string; type: 'boolean'; profile_version: number }[]>((resolve) => { resolveOffice = resolve; });
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]),
      publicAttributeFilters: vi.fn().mockReturnValue(officeFilters), publicSearch: vi.fn().mockResolvedValue({ items: [] }),
    };
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
    });
    const harness = await RouterTestingHarness.create('/explorar?category_code=oficina&attribute=wifi%3Atrue');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    const definitions = page.attributeFilters;
    await vi.waitFor(() => expect(api.publicAttributeFilters).toHaveBeenCalledWith('oficina'));
    page.updateFilter('category_code', '');
    expect(page.filters().attributes).toEqual({});
    expect(definitions()).toEqual([]);
    harness.fixture.destroy();
    resolveOffice([{ code: 'wifi', label: 'Wi-Fi', type: 'boolean', profile_version: 1 }]);
    await Promise.resolve();
    expect(definitions()).toEqual([]);
  });

  it.each(['resolved', 'pending'] as const)(
    'clears a form-only category and its %s attribute profile without changing the empty URL', async (profileState) => {
      let resolveOffice!: (value: { code: string; label: string; type: 'boolean'; profile_version: number }[]) => void;
      const pendingProfile = new Promise<{ code: string; label: string; type: 'boolean'; profile_version: number }[]>((resolve) => { resolveOffice = resolve; });
      const wifiProfile = [{ code: 'wifi', label: 'Wi-Fi', type: 'boolean' as const, profile_version: 1 }];
      const api = {
        publicCategories: vi.fn().mockResolvedValue([]),
        publicLocations: vi.fn().mockResolvedValue([]),
        publicAttributeFilters: vi.fn((code: string) => code === 'oficina' ? (profileState === 'resolved' ? Promise.resolve(wifiProfile) : pendingProfile) : Promise.resolve([])),
        publicSearch: vi.fn().mockResolvedValue({ items: [] }),
      };
      TestBed.configureTestingModule({
        providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
      });
      const harness = await RouterTestingHarness.create('/explorar');
      const page = harness.routeDebugElement!.componentInstance as ExplorePage;
      await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());

      page.updateFilter('category_code', 'oficina');
      if (profileState === 'resolved') {
        await vi.waitFor(() => expect(page.attributeFilters().map((filter) => filter.code)).toEqual(['wifi']));
        page.setAttribute('wifi', true);
      }
      page.filters.update((filters) => ({ ...filters, cursor: 'stale-cursor' }));

      expect(TestBed.inject(Router).url).toBe('/explorar');
      page.clearFilters();
      await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
      expect(TestBed.inject(Router).url).toBe('/explorar');
      expect(page.filters().category_code).toBe('');
      expect(page.filters().attributes).toEqual({});
      expect(page.filters().cursor).toBe('');
      expect(page.attributeFilters()).toEqual([]);
      expect(api.publicSearch.mock.calls[1][0].attributes).toEqual({});
      expect(api.publicSearch.mock.calls[1][0].category_code ?? '').toBe('');
      expect(api.publicSearch.mock.calls[1][0].cursor ?? '').toBe('');

      if (profileState === 'pending') {
        resolveOffice(wifiProfile);
        await Promise.resolve();
        expect(page.attributeFilters()).toEqual([]);
      }
    },
  );

  it('writes each next cursor once and clears it when criteria change', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([{ code: 'oficina', name: 'Oficina', order: 1 }]),
      publicLocations: vi.fn().mockResolvedValue([]), publicAttributeFilters: vi.fn().mockResolvedValue([]),
      publicSearch: vi
        .fn()
        .mockResolvedValueOnce({ items: [publicSpace('page-one')], next_cursor: 'cursor-two' })
        .mockResolvedValueOnce({ items: [publicSpace('page-two')], next_cursor: 'cursor-three' })
        .mockResolvedValue({ items: [] }),
    };
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideRouter([{ path: 'explorar', component: ExplorePage }]),
        { provide: ExploreApi, useValue: api },
      ],
    });

    const harness = await RouterTestingHarness.create('/explorar?page_size=1');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    page.loadMore();
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
    expect(api.publicSearch.mock.calls[1][0]).toMatchObject({ page_size: 1, cursor: 'cursor-two' });
    await vi.waitFor(() => expect(TestBed.inject(Router).url).toContain('cursor=cursor-two'));

    page.updateFilter('category_code', 'oficina');
    await page.search();
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(3));
    expect(api.publicSearch.mock.calls[2][0]).toMatchObject({
      category_code: 'oficina',
      page_size: 1,
      cursor: undefined,
    });
    expect(TestBed.inject(Router).url).not.toContain('cursor=');
  });

  it('discards an in-flight old-criteria result after Router navigation', async () => {
    let resolveOld!: (value: unknown) => void;
    const oldPage = new Promise((resolve) => (resolveOld = resolve));
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]),
      publicLocations: vi.fn().mockResolvedValue([]), publicAttributeFilters: vi.fn().mockResolvedValue([]),
      publicSearch: vi.fn().mockReturnValueOnce(oldPage).mockResolvedValue({
        items: [
          {
            space_id: 'fresh',
            category_code: 'oficina',
            category_name: 'Oficina',
            title: 'Fresh result',
            description: 'Current criteria result',
            area_m2: 20,
            rate_unit: 'hora',
            base_price_clp: '6000',
            currency: 'CLP',
          },
        ],
      }),
    };
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideRouter([{ path: 'explorar', component: ExplorePage }]),
        { provide: ExploreApi, useValue: api },
      ],
    });

    const harness = await RouterTestingHarness.create('/explorar');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    page.updateFilter('category_code', 'oficina');
    await page.search();
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
    await vi.waitFor(() => expect(page.items()[0]?.space_id).toBe('fresh'));
    resolveOld({ items: [{ space_id: 'stale' }] });
    await Promise.resolve();
    expect(page.items().map((item) => item.space_id)).toEqual(['fresh']);
  });

  it.each(['success', 'error'] as const)(
    'keeps a cached route authoritative when an older page request ends with %s',
    async (oldOutcome) => {
      let resolveOld!: (value: unknown) => void;
      let rejectOld!: (error: unknown) => void;
      const oldPage = new Promise((resolve, reject) => {
        resolveOld = resolve;
        rejectOld = reject;
      });
      const api = {
        publicCategories: vi.fn().mockResolvedValue([]),
        publicSearch: vi
          .fn()
          .mockResolvedValueOnce({ items: [publicSpace('first')], next_cursor: 'cursor-two' })
          .mockReturnValueOnce(oldPage),
      };
      TestBed.configureTestingModule({
        providers: [
          ...provideFrontendPorts(),
          provideHttpClient(),
          provideRouter([{ path: 'explorar', component: ExplorePage }]),
          { provide: ExploreApi, useValue: api },
        ],
      });

      const harness = await RouterTestingHarness.create('/explorar?page_size=1');
      const page = harness.routeDebugElement!.componentInstance as ExplorePage;
      await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
      page.loadMore();
      await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
      await TestBed.inject(Router).navigateByUrl('/explorar?page_size=1');
      expect(page.items().map((item) => item.space_id)).toEqual(['first']);

      if (oldOutcome === 'success') resolveOld({ items: [publicSpace('second')] });
      else rejectOld(new Error('stale request failed'));
      await Promise.resolve();
      expect(page.items().map((item) => item.space_id)).toEqual(['first']);
      expect(page.busy()).toBe(false);
    },
  );

  it.each(['success', 'error'] as const)(
    'ignores an older result after invalid route filters and allows a retry (%s)',
    async (oldOutcome) => {
      let resolveOld!: (value: unknown) => void;
      let rejectOld!: (error: unknown) => void;
      const oldPage = new Promise((resolve, reject) => {
        resolveOld = resolve;
        rejectOld = reject;
      });
      const api = {
        publicCategories: vi.fn().mockResolvedValue([]),
        publicSearch: vi.fn().mockReturnValueOnce(oldPage).mockResolvedValue({
          items: [publicSpace('retry-result')],
        }),
      };
      TestBed.configureTestingModule({
        providers: [
          ...provideFrontendPorts(),
          provideHttpClient(),
          provideRouter([{ path: 'explorar', component: ExplorePage }]),
          { provide: ExploreApi, useValue: api },
        ],
      });

      const harness = await RouterTestingHarness.create('/explorar');
      const page = harness.routeDebugElement!.componentInstance as ExplorePage;
      await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
      await TestBed.inject(Router).navigateByUrl(
        '/explorar?rate_unit=hora&min_unit_price_clp=invalid',
      );
      expect(page.items()).toEqual([]);
      expect(page.error()).toContain('precios CLP');

      if (oldOutcome === 'success') resolveOld({ items: [publicSpace('stale')] });
      else rejectOld(new Error('stale request failed'));
      await Promise.resolve();
      expect(page.items()).toEqual([]);
      expect(page.error()).toContain('precios CLP');

      page.updateFilter('min_unit_price_clp', '1000');
      await page.search();
      await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
      await vi.waitFor(() => expect(page.items()[0]?.space_id).toBe('retry-result'));
    },
  );
});
