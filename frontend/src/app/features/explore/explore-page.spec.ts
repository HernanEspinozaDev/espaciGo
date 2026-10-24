import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import axe from 'axe-core';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { provideFrontendPorts } from '../../frontend.providers';
import { PublicPageSeo } from '../../core/public-page-seo';
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

  it('strips safe legacy layout parameters without refetching or overwriting filter drafts', async () => {
    const api = { publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]), publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }) };
    TestBed.configureTestingModule({ providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }] });
    const harness = await RouterTestingHarness.create('/explorar?q=aplicado&view=grid&desktop_columns=4&mobile_columns=1&page_size=20');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    await vi.waitFor(() => expect(TestBed.inject(Router).url).not.toMatch(/view=|desktop_columns=|mobile_columns=/));
    page.updateFilter('q', 'borrador');
    const detailHref = TestBed.inject(Router).serializeUrl(page.detailLink('space-1'));
    expect(detailHref).toContain('page_size%3D20');
    expect(detailHref).not.toMatch(/view|desktop_columns|mobile_columns/);
    await TestBed.inject(Router).navigateByUrl('/explorar?q=aplicado&page_size=20&view=list');
    await vi.waitFor(() => expect(TestBed.inject(Router).url).not.toContain('view='));
    expect(api.publicSearch).toHaveBeenCalledOnce();
    expect(page.filters().q).toBe('borrador');
    expect(page.appliedFilters().q).toBe('aplicado');
    expect(harness.routeNativeElement!.querySelector('[aria-label="Vista de resultados"]')).toBeNull();
  });

  it('updates catalog robots metadata while canonicalizing legacy layout URLs without another fetch', async () => {
    const seo = { catalog: vi.fn(), responseForApiError: vi.fn() };
    const api = { publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]), publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }) };
    TestBed.configureTestingModule({ providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }, { provide: PublicPageSeo, useValue: seo }] });
    const harness = await RouterTestingHarness.create('/explorar');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    await vi.waitFor(() => expect(seo.catalog).toHaveBeenLastCalledWith(false));
    await TestBed.inject(Router).navigateByUrl('/explorar?view=grid&desktop_columns=4&mobile_columns=1');
    await vi.waitFor(() => expect(TestBed.inject(Router).url).toBe('/explorar'));
    expect(seo.catalog).toHaveBeenLastCalledWith(false);
    page.updateFilter('q', 'draft');
    await page.search();
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
    expect(seo.catalog).toHaveBeenLastCalledWith(true);
    await TestBed.inject(Router).navigateByUrl('/explorar?q=draft&view=list');
    await vi.waitFor(() => expect(TestBed.inject(Router).url).toBe('/explorar?q=draft'));
    expect(seo.catalog).toHaveBeenLastCalledWith(true);
    expect(api.publicSearch).toHaveBeenCalledTimes(2);
  });

  it('reports loaded results and request batch size across pagination history', async () => {
    const seo = { catalog: vi.fn(), responseForApiError: vi.fn() };
    const api = { publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]), publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValueOnce({ items: [publicSpace('one')], next_cursor: 'two' }).mockResolvedValueOnce({ items: [publicSpace('two')] }) };
    TestBed.configureTestingModule({ providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }, { provide: PublicPageSeo, useValue: seo }] });
    const harness = await RouterTestingHarness.create('/explorar?page_size=1');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    await vi.waitFor(() => expect(page.items()).toHaveLength(1));
    expect(seo.catalog).toHaveBeenLastCalledWith(true);
    harness.detectChanges();
    expect(harness.routeNativeElement!.querySelector('.results-count')?.textContent).toContain('1 espacio cargado · 1 por solicitud');
    page.loadMore();
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
    await vi.waitFor(() => expect(page.items()).toHaveLength(2));
    expect(seo.catalog).toHaveBeenLastCalledWith(true);
    harness.detectChanges();
    expect(harness.routeNativeElement!.querySelector('.results-count')?.textContent).toContain('2 espacios cargados · 1 por solicitud');
    await TestBed.inject(Router).navigateByUrl('/explorar?page_size=1');
    expect(seo.catalog).toHaveBeenLastCalledWith(true);
    harness.detectChanges();
    expect(harness.routeNativeElement!.querySelector('.results-count')?.textContent).toContain('1 espacio cargado · 1 por solicitud');
    await TestBed.inject(Router).navigateByUrl('/explorar?page_size=1&cursor=two');
    expect(seo.catalog).toHaveBeenLastCalledWith(true);
    harness.detectChanges();
    expect(harness.routeNativeElement!.querySelector('.results-count')?.textContent).toContain('2 espacios cargados · 1 por solicitud');
  });

  it('changes to 12 real results, clears the cursor and preserves legacy 20 deep links', async () => {
    const api = { publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]), publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }) };
    TestBed.configureTestingModule({ providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }] });
    const harness = await RouterTestingHarness.create('/explorar?page_size=20&cursor=old');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    await page.changePageSize(12);
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
    expect(TestBed.inject(Router).url).toContain('page_size=12');
    expect(TestBed.inject(Router).url).not.toContain('cursor=');
    expect(api.publicSearch.mock.calls[1][0]).toMatchObject({ page_size: 12, cursor: undefined });
  });

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

  it('puts category immediately after search and sorts by title through the real Router', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([{ code: 'oficina', name: 'Oficina', order: 1 }]),
      publicLocations: vi.fn().mockResolvedValue([]),
      publicAttributeFilters: vi.fn().mockResolvedValue([]),
      publicSearch: vi.fn().mockResolvedValue({ items: [publicSpace('first')], next_cursor: 'cursor-next' }),
    };
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
    });
    const harness = await RouterTestingHarness.create('/explorar?cursor=old-cursor');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    const labels = [...harness.routeNativeElement!.querySelectorAll<HTMLElement>('.filters > label')];
    expect(labels[0].querySelector('#catalog-search')).not.toBeNull();
    expect(labels[1].textContent?.trim().startsWith('Categoría')).toBe(true);

    page.requestSort('title_asc');
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
    expect(TestBed.inject(Router).url).toContain('sort=title_asc');
    expect(TestBed.inject(Router).url).not.toContain('cursor=');
    expect(api.publicSearch.mock.calls[1][0]).toMatchObject({ sort: 'title_asc', cursor: undefined });
  });

  it('asks whether to apply or discard filter drafts before changing the desktop sort', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]),
      publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }),
    };
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
    });
    const harness = await RouterTestingHarness.create('/explorar?q=aplicado');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    page.updateFilter('q', 'borrador');
    page.requestSort('title_desc');
    harness.detectChanges();
    expect(api.publicSearch).toHaveBeenCalledOnce();
    expect(harness.routeNativeElement!.textContent).toContain('¿Quieres aplicar tus borradores');
    const apply = [...harness.routeNativeElement!.querySelectorAll('button')]
      .find((button) => button.textContent?.trim() === 'Aplicar cambios y ordenar');
    apply?.click();
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
    expect(api.publicSearch.mock.calls[1][0]).toMatchObject({ q: 'borrador', sort: 'title_desc' });
    expect(TestBed.inject(Router).url).toContain('q=borrador');
    expect(TestBed.inject(Router).url).toContain('sort=title_desc');
  });

  it('keeps price order available and directs the user to select a tariff unit', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]),
      publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }),
    };
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
    });
    const harness = await RouterTestingHarness.create('/explorar');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    page.requestSort('unit_price_asc');
    harness.detectChanges();
    const option = harness.routeNativeElement!.querySelector<HTMLSelectElement>('.catalog-sort select option[value="unit_price_asc"]');
    expect(option?.disabled).toBe(false);
    expect(harness.routeNativeElement!.textContent).toContain('No se mezclan tarifas de distintas unidades');
    const chooseUnit = [...harness.routeNativeElement!.querySelectorAll('button')]
      .find((button) => button.textContent?.trim() === 'Elegir modalidad');
    expect(chooseUnit).toBeDefined();
    expect(api.publicSearch).toHaveBeenCalledOnce();
  });

  it.each([
    {
      sort: 'unit_price_asc',
      update: (page: ExplorePage) => page.updateFilter('rate_unit', 'hora'),
      expected: { sort: 'unit_price_asc', rate_unit: 'hora' },
    },
    {
      sort: 'relevance',
      update: (page: ExplorePage) => page.updateFilter('q', 'sala'),
      expected: { sort: 'relevance', q: 'sala' },
    },
    {
      sort: 'estimated_total_asc',
      update: (page: ExplorePage) => {
        page.updateDate('start_at', '2026-10-20T13:00:00.000Z');
        page.updateDate('end_at', '2026-10-20T14:00:00.000Z');
      },
      expected: { sort: 'estimated_total_asc', start_at: '2026-10-20T13:00:00.000Z', end_at: '2026-10-20T14:00:00.000Z' },
    },
  ])('applies pending $sort together with completed prerequisites when submitting Search', async ({ sort, update, expected }) => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]),
      publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }),
    };
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
    });
    const harness = await RouterTestingHarness.create('/explorar');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());

    page.requestSort(sort);
    expect(page.pendingSort()).toBe(sort);
    update(page);
    await page.submitSearch();

    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
    expect(page.pendingSort()).toBe('');
    expect(TestBed.inject(Router).url).toContain(`sort=${sort}`);
    expect(api.publicSearch.mock.calls[1][0]).toMatchObject(expected);
  });

  it('keeps a prerequisite intent pending when Search is submitted incomplete', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]),
      publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }),
    };
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
    });
    const harness = await RouterTestingHarness.create('/explorar');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    page.requestSort('unit_price_asc');

    await page.submitSearch();

    expect(page.pendingSort()).toBe('unit_price_asc');
    expect(page.error()).toContain('elige explícitamente hora, día o mes');
    expect(api.publicSearch).toHaveBeenCalledOnce();
    expect(TestBed.inject(Router).url).toBe('/explorar');
  });

  it('focuses the real interval start input from catalog sort guidance', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]),
      publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }),
    };
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
    });
    const harness = await RouterTestingHarness.create('/explorar');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    const element = harness.routeNativeElement!;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());

    page.requestSort('estimated_total_asc');
    harness.detectChanges();
    expect(element.querySelector('#start-at')).not.toBeNull();
    element.querySelector<HTMLButtonElement>('.catalog-results .sort-guidance .text-action')!.click();
    await vi.waitFor(() => expect(element.ownerDocument.activeElement).toBe(element.querySelector('#start-at')));

    page.cancelSort();
  });

  it('keeps the mobile filter dialog open while editing filters', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]),
      publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }),
    };
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
    });
    const harness = await RouterTestingHarness.create('/explorar');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    page.openFilters(harness.routeNativeElement!.querySelector('.filter-toggle') as HTMLElement);
    harness.detectChanges();
    page.updateFilter('q', 'sala');
    harness.detectChanges();
    expect(harness.routeNativeElement!.querySelector('[role="dialog"]')).not.toBeNull();
    expect(harness.routeNativeElement!.querySelector<HTMLInputElement>('#catalog-search')?.value).toBe('sala');
    expect(api.publicSearch).toHaveBeenCalledOnce();
  });

  it('restores search, location, attributes, and interval estimates from the real Router URL', async () => {
    const api = { publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]), publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }) };
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }],
    });
    await RouterTestingHarness.create('/explorar?q=sala+abierta&region_code=13&comuna_code=13123&attribute=wifi%3Atrue&start_at=2026-10-15T12%3A00%3A00.000Z&end_at=2026-10-15T13%3A00%3A00.000Z&min_total_clp=9000&sort=estimated_total_asc');
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    expect(api.publicSearch).toHaveBeenCalledWith(expect.objectContaining({
      q: 'sala abierta', region_code: '13', comuna_code: '13123', attributes: { wifi: { kind: 'boolean', value: true } },
      start_at: '2026-10-15T12:00:00.000Z', end_at: '2026-10-15T13:00:00.000Z', min_total_clp: '9000', sort: 'estimated_total_asc',
    }));
  });

  it('restores typed range and repeated enum-list values from real Router query parameters',async()=>{
    const api={publicCategories:vi.fn().mockResolvedValue([]),publicLocations:vi.fn().mockResolvedValue([]),publicAttributeFilters:vi.fn().mockResolvedValue([]),publicSearch:vi.fn().mockResolvedValue({items:[]})};
    TestBed.configureTestingModule({providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([{path:'explorar',component:ExplorePage}]),{provide:ExploreApi,useValue:api}]});
    await RouterTestingHarness.create('/explorar?category_code=sala_multiproposito&attribute=disposiciones_admitidas%3Ain%3Aauditorio&attribute=disposiciones_admitidas%3Ain%3Aaula&attribute=sillas_disponibles%3Agte%3A20');
    await vi.waitFor(()=>expect(api.publicSearch).toHaveBeenCalledOnce());
    expect(api.publicSearch.mock.calls[0][0].attributes).toEqual({disposiciones_admitidas:{kind:'enum_list',values:['auditorio','aula']},sillas_disponibles:{kind:'range',minimum:'20'}});
  });

  it('restores CSV enum-list values and canonicalizes them in the Router URL', async () => {
    const api = { publicCategories: vi.fn().mockResolvedValue([]), publicLocations: vi.fn().mockResolvedValue([]), publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }) };
    TestBed.configureTestingModule({ providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }] });
    const harness = await RouterTestingHarness.create('/explorar?category_code=oficina&attribute=climatizacion%3Ain%3Acalefaccion%2Caire_acondicionado&attribute=climatizacion%3Ain%3Arefrigeracion');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    expect(api.publicSearch.mock.calls[0][0].attributes).toEqual({ climatizacion: { kind: 'enum_list', values: ['calefaccion', 'aire_acondicionado', 'refrigeracion'] } });
    await page.search();
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
    expect(TestBed.inject(Router).url).toContain('attribute=climatizacion:in:aire_acondicionado,calefaccion,refrigeracion');
    expect(page.filters().attributes['climatizacion']).toEqual({ kind: 'enum_list', values: ['aire_acondicionado', 'calefaccion', 'refrigeracion'] });
  });

  it.each([
    'climatizacion:in:aire_acondicionado,aire_acondicionado',
    'climatizacion:in:aire_acondicionado&attribute=climatizacion:in:aire_acondicionado',
    'climatizacion:in:aire_acondicionado&attribute=climatizacion:eq:aire_acondicionado',
  ])('rejects duplicate or ambiguous list options in the Router URL: %s', async (raw) => {
    const api = { publicCategories: vi.fn().mockResolvedValue([]), publicAttributeFilters: vi.fn().mockResolvedValue([]), publicSearch: vi.fn().mockResolvedValue({ items: [] }) };
    TestBed.configureTestingModule({ providers: [...provideFrontendPorts(), provideHttpClient(), provideRouter([{ path: 'explorar', component: ExplorePage }]), { provide: ExploreApi, useValue: api }] });
    await RouterTestingHarness.create(`/explorar?category_code=oficina&attribute=${encodeURIComponent(raw.split('&')[0])}${raw.includes('&attribute=') ? `&attribute=${encodeURIComponent(raw.split('&attribute=')[1])}` : ''}`);
    await vi.waitFor(() => expect(api.publicCategories).toHaveBeenCalledOnce());
    expect(api.publicSearch).not.toHaveBeenCalled();
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

  it.each([
    {
      start: '/explorar?q=oficina&sort=relevance&page_size=1&cursor=existing-cursor',
      chip: 'q',
      absent: ['q', 'sort'],
      expected: { sort: 'default' },
    },
    {
      start: '/explorar?rate_unit=hora&min_unit_price_clp=10000&max_unit_price_clp=15000&sort=unit_price_asc&page_size=1&cursor=existing-cursor',
      chip: 'rate_unit',
      absent: ['rate_unit', 'min_unit_price_clp', 'max_unit_price_clp', 'sort'],
      expected: { sort: 'default' },
    },
    {
      start: '/explorar?rate_unit=hora&start_at=2026-10-20T10%3A00%3A00.000Z&end_at=2026-10-20T11%3A00%3A00.000Z&min_total_clp=18000&max_total_clp=25000&sort=estimated_total_asc&page_size=1&cursor=existing-cursor',
      chip: 'start_at',
      absent: ['start_at', 'end_at', 'min_total_clp', 'max_total_clp', 'sort'],
      expected: { rate_unit: 'hora', sort: 'default' },
    },
  ])('removes $chip and its dependent criteria through the Router', async ({ start, chip, absent, expected }) => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]),
      publicLocations: vi.fn().mockResolvedValue([]),
      publicSearch: vi.fn().mockResolvedValue({ items: [publicSpace('valid-result')] }),
    };
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(), provideHttpClient(),
        provideRouter([{ path: 'explorar', component: ExplorePage }]),
        { provide: ExploreApi, useValue: api },
      ],
    });
    const harness = await RouterTestingHarness.create(start);
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    const router = TestBed.inject(Router);
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    expect(api.publicSearch.mock.calls[0][0]).toMatchObject({ cursor: 'existing-cursor', page_size: 1 });

    page.removeFilter(chip);
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledTimes(2));
    await vi.waitFor(() => expect(page.items()[0]?.space_id).toBe('valid-result'));
    expect(router.parseUrl(router.url).queryParamMap.keys).not.toContain('cursor');
    for (const key of absent) expect(router.parseUrl(router.url).queryParamMap.has(key)).toBe(false);
    expect(api.publicSearch.mock.calls[1][0]).toMatchObject(expected);
    expect(api.publicSearch.mock.calls[1][0].cursor).toBeUndefined();
    expect(page.hasUnappliedChanges()).toBe(false);
  });

  it('renders applied chips with field-specific names and leaves draft edits unapplied', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([{ code: '13', name: 'Categoría numérica', order: 1 }]),
      publicLocations: vi.fn().mockResolvedValue([
        { region_code: '13', region_name: 'Metropolitana', comuna_code: '13123', comuna_name: 'Providencia' },
      ]),
      publicAttributeFilters: vi.fn().mockResolvedValue([]),
      publicSearch: vi.fn().mockResolvedValue({ items: [] }),
    };
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(), provideHttpClient(),
        provideRouter([{ path: 'explorar', component: ExplorePage }]),
        { provide: ExploreApi, useValue: api },
      ],
    });
    const harness = await RouterTestingHarness.create('/explorar?category_code=13&region_code=13&comuna_code=13123&q=13123');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    await vi.waitFor(() => expect(api.publicLocations).toHaveBeenCalledOnce());

    const chips = page.activeFilters().map((chip) => chip.label);
    expect(chips).toContain('Categoría: Categoría numérica');
    expect(chips).toContain('Región: Metropolitana');
    expect(chips).toContain('Comuna: Providencia');
    expect(chips).toContain('Búsqueda: 13123');

    page.updateFilter('q', 'texto aún sin aplicar');
    expect(page.hasUnappliedChanges()).toBe(true);
    expect(page.activeFilters().find((chip) => chip.key === 'q')?.label).toBe('Búsqueda: 13123');
  });

  it('has no WCAG A/AA axe violations in desktop catalog and mobile filter dialog', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]),
      publicLocations: vi.fn().mockResolvedValue([]),
      publicSearch: vi.fn().mockResolvedValue({ items: [publicSpace('no-photo')] }),
    };
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(), provideHttpClient(),
        provideRouter([{ path: 'explorar', component: ExplorePage }]),
        { provide: ExploreApi, useValue: api },
      ],
    });
    const harness = await RouterTestingHarness.create('/explorar');
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    const tags = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];
    const desktop = await axe.run(harness.fixture.nativeElement, { runOnly: { type: 'tag', values: tags } });
    expect(desktop.violations.map(({ id }) => id)).toEqual([]);

    const trigger = harness.routeNativeElement!.querySelector<HTMLElement>('.filter-toggle')!;
    page.openFilters(trigger);
    harness.fixture.detectChanges();
    const mobile = await axe.run(harness.fixture.nativeElement, { runOnly: { type: 'tag', values: tags } });
    expect(mobile.violations.map(({ id }) => id)).toEqual([]);
  });

  it('replaces a failed public cover with the accessible no-photo placeholder', async () => {
    const api = {
      publicCategories: vi.fn().mockResolvedValue([]),
      publicLocations: vi.fn().mockResolvedValue([]),
      publicSearch: vi.fn().mockResolvedValue({
        items: [{ ...publicSpace('failed-cover'), cover_image_id: 'synthetic-photo' }],
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
    await vi.waitFor(() => expect(api.publicSearch).toHaveBeenCalledOnce());
    const page = harness.routeDebugElement!.componentInstance as ExplorePage;
    await vi.waitFor(() => expect(page.items()).toHaveLength(1));
    expect(page.imageFailed('failed-cover')).toBe(false);
    page.markImageFailed('failed-cover');
    harness.fixture.detectChanges();
    expect(harness.routeNativeElement!.querySelector('.space-cover')).toBeNull();
    expect(
      harness.routeNativeElement!.querySelector('[aria-label="Sin fotografía pública"]'),
    ).not.toBeNull();
  });
});
