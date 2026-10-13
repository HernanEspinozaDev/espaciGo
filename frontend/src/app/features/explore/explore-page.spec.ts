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

  it('writes each next cursor once and clears it when criteria change', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([{ code: 'oficina', name: 'Oficina', order: 1 }]),
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
});
