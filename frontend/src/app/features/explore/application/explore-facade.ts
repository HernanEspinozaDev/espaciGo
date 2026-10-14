import { DestroyRef, Injectable, OnInit, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, ParamMap, Router, UrlTree } from '@angular/router';
import {
  ExplorePort,
  PublicCatalogFilters,
  PublicCategory,
  PublicSpace,
  PublicLocation,
  PublicAttributeFilter,
  PublicAttributePredicate,
} from '../domain/ports/explore-port';
import { PublicPageSeo } from '../../../core/public-page-seo';
import { decodePublicAttributeFilters, encodePublicAttributeFilters } from '../../../core/public-attribute-filter-codec';

type ExploreFilters = {
  q: string;
  category_code: string;
  rate_unit: string;
  min_unit_price_clp: string;
  max_unit_price_clp: string;
  sort: string;
  page_size: number;
  cursor: string;
  region_code: string;
  comuna_code: string;
  min_area_m2: string;
  start_at: string;
  end_at: string;
  min_total_clp: string;
  max_total_clp: string;
  attributes: Record<string, PublicAttributePredicate>;
};
type ExploreCriterion = Exclude<keyof ExploreFilters, 'cursor'>;
export type SortRequirement = { target: 'query' | 'rate_unit' | 'start_at'; message: string; action: string };
type CriteriaPageCache = { cursors: string[]; pages: Map<string, PublicSpace[]>; nextCursors: Map<string, string> };

@Injectable()
export class ExploreFacade implements OnInit {
  private readonly api = inject(ExplorePort);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly meta = inject(Meta);
  private readonly title = inject(Title);
  private readonly destroyRef = inject(DestroyRef);
  private readonly seo = inject(PublicPageSeo);
  private requestGeneration = 0;
  private criteriaKey: string | undefined;
  private routeRequestKey: string | undefined;
  private readonly criteriaPageCaches = new Map<string, CriteriaPageCache>();
  private navigationPending = false;
  private attributeFilterCategory = '';
  private attributeFilterGeneration = 0;

  readonly categories = signal<PublicCategory[]>([]);
  readonly locations = signal<PublicLocation[]>([]);
  readonly attributeFilters = signal<PublicAttributeFilter[]>([]);
  readonly items = signal<PublicSpace[]>([]);
  readonly nextCursor = signal('');
  readonly busy = signal(false);
  readonly error = signal('');
  readonly filters = signal<ExploreFilters>({
    q: '',
    category_code: '',
    rate_unit: '',
    min_unit_price_clp: '',
    max_unit_price_clp: '',
    sort: 'default',
    page_size: 20,
    cursor: '',
    region_code: '', comuna_code: '', min_area_m2: '', start_at: '', end_at: '',
    min_total_clp: '', max_total_clp: '', attributes: {},
  });
  readonly appliedFilters = signal<ExploreFilters>({
    q: '', category_code: '', rate_unit: '', min_unit_price_clp: '', max_unit_price_clp: '',
    sort: 'default', page_size: 20, cursor: '', region_code: '', comuna_code: '', min_area_m2: '',
    start_at: '', end_at: '', min_total_clp: '', max_total_clp: '', attributes: {},
  });
  readonly hasUnappliedChanges = computed(() => criteriaSignature(this.filters()) !== criteriaSignature(this.appliedFilters()));
  readonly pendingSort = signal('');

  constructor() {
    this.title.setTitle('Explorar espacios | EspaciGo');
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => {
      this.attributeFilterGeneration++;
      this.meta.removeTag("name='robots'");
    });
  }

  ngOnInit(): void {
    this.route.queryParamMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      void this.loadCategories();
      void this.loadLocations();
      this.loadFromRoute(params);
    });
  }

  updateFilter<K extends ExploreCriterion>(key: K, value: ExploreFilters[K]): void {
    if (key === 'category_code') {
      this.filters.update((filters) => ({ ...filters, category_code: String(value), attributes: {}, cursor: '' }));
      this.selectAttributeFilterCategory(String(value));
    } else {
      this.filters.update((filters) => ({ ...filters, [key]: value, cursor: '' }));
    }
    this.invalidateCriteria();
  }

  setAttribute(code: string, value: PublicAttributePredicate | boolean | null): void {
    const attributes = { ...this.filters().attributes };
    if (value === null) delete attributes[code]; else attributes[code] = typeof value==='boolean'?{kind:'boolean',value}:value;
    this.filters.update((filters) => ({ ...filters, attributes, cursor: '' }));
    this.invalidateCriteria();
  }

  updateDate(key: 'start_at' | 'end_at', value: string): void {
    this.updateFilter(key, value ? new Date(value).toISOString() : '');
  }

  dateInput(value: string): string {
    if (!value) return '';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? '' : new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16);
  }

  async search(): Promise<void> {
    const requestedSort = this.pendingSort();
    const filters = {
      ...this.filters(),
      ...(requestedSort ? { sort: requestedSort } : {}),
      cursor: '',
    };
    const sortRequirement = requestedSort ? this.sortRequirement(requestedSort, filters) : null;
    if (sortRequirement) {
      this.error.set(sortRequirement.message);
      return;
    }
    if (!this.validateForSearch(filters)) return;
    if (requestedSort) this.pendingSort.set('');
    this.filters.set(filters);
    const queryParams = encodeFilters(filters);
    if (!sameOwnedQuery(this.route.snapshot.queryParamMap, queryParams)) {
      this.navigationPending = true;
      await this.router.navigate([], {
        relativeTo: this.route,
        queryParams,
        replaceUrl: false,
      });
      this.navigationPending = false;
      return;
    }
    if (!this.busy()) {
      this.criteriaKey = criteriaSignature(filters);
      this.routeRequestKey = `${this.criteriaKey}|cursor=${filters.cursor}`;
      await this.loadPage(filters, false);
    }
  }

  removeAppliedFilter(key: string): void {
    const current = this.appliedFilters();
    const next: ExploreFilters = { ...current, attributes: { ...current.attributes }, cursor: '' };
    if (key.startsWith('attribute:')) {
      delete next.attributes[key.slice('attribute:'.length)];
    } else if (key === 'q') {
      next.q = '';
      if (next.sort === 'relevance') next.sort = 'default';
    } else if (key === 'rate_unit') {
      next.rate_unit = '';
      next.min_unit_price_clp = '';
      next.max_unit_price_clp = '';
      if (next.sort === 'unit_price_asc' || next.sort === 'unit_price_desc') next.sort = 'default';
    } else if (key === 'start_at' || key === 'end_at') {
      next.start_at = '';
      next.end_at = '';
      next.min_total_clp = '';
      next.max_total_clp = '';
      if (next.sort === 'estimated_total_asc' || next.sort === 'estimated_total_desc') next.sort = 'default';
    } else if (key === 'region_code') {
      next.region_code = '';
      next.comuna_code = '';
    } else if (key === 'category_code') {
      next.category_code = '';
      next.attributes = {};
    } else if (key === 'sort') {
      next.sort = 'default';
    } else {
      switch (key) {
        case 'comuna_code': next.comuna_code = ''; break;
        case 'min_unit_price_clp': next.min_unit_price_clp = ''; break;
        case 'max_unit_price_clp': next.max_unit_price_clp = ''; break;
        case 'min_area_m2': next.min_area_m2 = ''; break;
        case 'min_total_clp': next.min_total_clp = ''; break;
        case 'max_total_clp': next.max_total_clp = ''; break;
      }
    }
    this.filters.set(next);
    this.selectAttributeFilterCategory(next.category_code);
    this.invalidateCriteria();
    void this.search();
  }

  requestSort(sort: string): void {
    if (!isSort(sort)) return;
    this.pendingSort.set(sort);
    if (this.sortRequirement(sort, this.filters())) return;
    if (this.hasUnappliedChanges()) {
      return;
    }
    this.confirmSort(true);
  }

  confirmSort(applyDraftChanges: boolean): void {
    const sort = this.pendingSort();
    if (!sort) return;
    const current = applyDraftChanges ? this.filters() : this.appliedFilters();
    const requirement = this.sortRequirement(sort, current);
    if (requirement) {
      this.filters.set({ ...current, sort, cursor: '' });
      this.pendingSort.set(sort);
      this.error.set(requirement.message);
      return;
    }
    const next = { ...current, sort, cursor: '' };
    this.filters.set(next);
    this.selectAttributeFilterCategory(next.category_code);
    this.pendingSort.set('');
    this.invalidateCriteria();
    void this.search();
  }

  cancelSort(): void {
    this.pendingSort.set('');
  }

  sortRequirement(sort: string, filters: ExploreFilters): SortRequirement | null {
    if ((sort === 'unit_price_asc' || sort === 'unit_price_desc') && !filters.rate_unit) {
      return { target: 'rate_unit', message: 'Para comparar precios, elige explícitamente hora, día o mes. No se mezclan tarifas de distintas unidades.', action: 'Elegir modalidad' };
    }
    if (sort === 'relevance' && !filters.q.trim()) {
      return { target: 'query', message: 'La relevancia necesita un texto en Buscar.', action: 'Escribir búsqueda' };
    }
    if (sort.startsWith('estimated_total_') && (!filters.start_at || !filters.end_at)) {
      return { target: 'start_at', message: 'El total estimado necesita las fechas Desde y Hasta.', action: 'Completar intervalo' };
    }
    return null;
  }

  detailLink(spaceId: string): UrlTree {
    const queryParams = encodeFilters({ ...this.appliedFilters(), cursor: '' });
    const listing = this.router.serializeUrl(
      this.router.createUrlTree(['/explorar'], { queryParams }),
    );
    return this.router.createUrlTree(['/espacios', spaceId], {
      queryParams: { returnTo: listing },
    });
  }

  clearFilters(): void {
    this.cancelSort();
    this.filters.set({
      category_code: '',
      q: '',
      rate_unit: '',
      min_unit_price_clp: '',
      max_unit_price_clp: '',
      sort: 'default',
      page_size: 20,
      cursor: '',
      region_code: '', comuna_code: '', min_area_m2: '', start_at: '', end_at: '',
      min_total_clp: '', max_total_clp: '', attributes: {},
    });
    this.selectAttributeFilterCategory('');
    this.invalidateCriteria();
    void this.search();
  }

  loadMore(): void {
    const next = this.nextCursor();
    if (!next || this.busy() || this.navigationPending) return;
    const filters = { ...this.filters(), cursor: next };
    const queryParams = encodeFilters(filters);
    if (sameOwnedQuery(this.route.snapshot.queryParamMap, queryParams)) return;
    this.navigationPending = true;
    void this.router
      .navigate([], { relativeTo: this.route, queryParams, replaceUrl: false })
      .finally(() => (this.navigationPending = false));
  }

  price(value: string): string {
    return `CLP ${BigInt(value).toLocaleString('es-CL')}`;
  }

  private loadFromRoute(params: ParamMap): void {
    this.cancelSort();
    const decoded = decodeFilters(params);
    if (!decoded.filters) {
      this.invalidateCriteria();
      this.error.set(decoded.error);
      return;
    }
    const filters = decoded.filters;
    this.appliedFilters.set(filters);
    this.filters.set(filters);
    this.selectAttributeFilterCategory(filters.category_code);
    const nextCriteriaKey = criteriaSignature(filters);
    const criteriaChanged = this.criteriaKey !== nextCriteriaKey;
    const requestKey = `${nextCriteriaKey}|cursor=${filters.cursor}`;
    if (!criteriaChanged && this.routeRequestKey === requestKey) return;
    // Every distinct route state supersedes the previous request, including
    // cache hits and invalid filters that do not start a replacement request.
    this.requestGeneration++;
    this.busy.set(false);
    if (criteriaChanged) {
      this.criteriaKey = nextCriteriaKey;
      this.items.set([]);
      this.nextCursor.set('');
    }
    this.routeRequestKey = requestKey;
    if (!this.validateForSearch(filters)) return;
    this.error.set('');
    const cache = this.pageCache(nextCriteriaKey);
    const cachedIndex = cache.cursors.indexOf(filters.cursor);
    if (cachedIndex >= 0) {
      this.items.set(cache.cursors.slice(0, cachedIndex + 1).flatMap((cursor) => cache.pages.get(cursor) ?? []));
      const cached = cache.pages.get(filters.cursor) ?? [];
      this.nextCursor.set(cached.length ? (cache.nextCursors.get(filters.cursor) ?? '') : '');
      this.busy.set(false);
      return;
    }
    void this.loadPage(filters, !criteriaChanged && this.items().length > 0);
  }

  private async loadPage(filters: ExploreFilters, append: boolean): Promise<void> {
    const generation = ++this.requestGeneration;
    this.error.set('');
    this.busy.set(true);
    if (!append && this.items().length === 0) this.items.set([]);
    const request: PublicCatalogFilters = {
      category_code: filters.category_code || undefined,
      q: filters.q || undefined,
      rate_unit: filters.rate_unit || undefined,
      min_unit_price_clp: filters.min_unit_price_clp || undefined,
      max_unit_price_clp: filters.max_unit_price_clp || undefined,
      sort: filters.sort,
      page_size: filters.page_size,
      cursor: filters.cursor || undefined,
      region_code: filters.region_code || undefined, comuna_code: filters.comuna_code || undefined,
      min_area_m2: filters.min_area_m2 || undefined, start_at: filters.start_at || undefined,
      end_at: filters.end_at || undefined, min_total_clp: filters.min_total_clp || undefined,
      max_total_clp: filters.max_total_clp || undefined, attributes: filters.attributes,
    };
    try {
      const page = await this.api.publicSearch(request);
      if (generation !== this.requestGeneration) return;
      this.seo.catalog(this.route.snapshot.queryParamMap.keys.length > 0);
      const cache = this.pageCache(criteriaSignature(filters));
      cache.pages.set(filters.cursor, page.items);
      cache.nextCursors.set(filters.cursor, page.next_cursor ?? '');
      if (!cache.cursors.includes(filters.cursor)) cache.cursors.push(filters.cursor);
      this.items.update((current) => (append ? [...current, ...page.items] : page.items));
      this.nextCursor.set(page.next_cursor ?? '');
    } catch (error) {
      if (generation === this.requestGeneration) {
        this.seo.responseForApiError(error);
        this.error.set('No fue posible cargar los espacios públicos. Inténtalo nuevamente.');
      }
    } finally {
      if (generation === this.requestGeneration) this.busy.set(false);
    }
  }

  private invalidateCriteria(): void {
    this.requestGeneration++;
    this.routeRequestKey = undefined;
    this.items.set([]);
    this.nextCursor.set('');
    this.error.set('');
    this.busy.set(false);
  }

  private pageCache(key: string): CriteriaPageCache {
    const existing = this.criteriaPageCaches.get(key);
    if (existing) {
      this.criteriaPageCaches.delete(key);
      this.criteriaPageCaches.set(key, existing);
      return existing;
    }
    const cache: CriteriaPageCache = { cursors: [], pages: new Map(), nextCursors: new Map() };
    this.criteriaPageCaches.set(key, cache);
    while (this.criteriaPageCaches.size > 8) {
      const oldest = this.criteriaPageCaches.keys().next().value as string | undefined;
      if (oldest === undefined) break;
      this.criteriaPageCaches.delete(oldest);
    }
    return cache;
  }

  private validateForSearch(filters: ExploreFilters): boolean {
    if (!Number.isInteger(filters.page_size) || filters.page_size < 1 || filters.page_size > 25) {
      this.error.set('El tamaño de página debe estar entre 1 y 25.');
      this.busy.set(false);
      return false;
    }
    if (!validPrice(filters.min_unit_price_clp) || !validPrice(filters.max_unit_price_clp)) {
      this.error.set('Ingresa precios CLP enteros válidos.');
      this.busy.set(false);
      return false;
    }
    const sortRequirement = this.sortRequirement(filters.sort, filters);
    if (sortRequirement || ((filters.min_total_clp || filters.max_total_clp) && (!filters.start_at || !filters.end_at))) {
      this.error.set(sortRequirement?.message ?? 'Los filtros de total estimado necesitan las fechas Desde y Hasta.');
      this.busy.set(false);
      return false;
    }
    if (
      (filters.min_unit_price_clp || filters.max_unit_price_clp) && !filters.rate_unit
    ) {
      this.error.set('Elige hora, día o mes para filtrar una tarifa sin mezclar unidades.');
      this.busy.set(false);
      return false;
    }
    if (!validPrice(filters.min_total_clp) || !validPrice(filters.max_total_clp)) {
      this.error.set('Ingresa totales CLP enteros válidos.'); this.busy.set(false); return false;
    }
    if (filters.min_area_m2 && (!/^\d+(?:\.\d{1,2})?$/.test(filters.min_area_m2) || Number(filters.min_area_m2) <= 0)) {
      this.error.set('Ingresa una superficie mínima válida.'); this.busy.set(false); return false;
    }
    if (filters.start_at && filters.end_at && Date.parse(filters.end_at) <= Date.parse(filters.start_at)) {
      this.error.set('El fin debe ser posterior al inicio.'); this.busy.set(false); return false;
    }
    return true;
  }

  private async loadCategories(): Promise<void> {
    try {
      this.categories.set(await this.api.publicCategories());
    } catch (error) {
      this.seo.responseForApiError(error);
      this.error.set('No fue posible cargar las categorías públicas.');
    }
  }

  async loadLocations(): Promise<void> {
    try { this.locations.set(await this.api.publicLocations()); }
    catch (error) { this.seo.responseForApiError(error); this.error.set('No fue posible cargar las ubicaciones públicas.'); }
  }

  private selectAttributeFilterCategory(code: string): void {
    if (code === this.attributeFilterCategory) return;
    this.attributeFilterCategory = code;
    const generation = ++this.attributeFilterGeneration;
    this.attributeFilters.set([]);
    if (!code) return;
    void this.api.publicAttributeFilters(code).then((filters) => {
      if (this.destroyRef.destroyed || generation !== this.attributeFilterGeneration || this.filters().category_code !== code) return;
      this.attributeFilters.set(filters);
    }).catch((error) => {
      if (this.destroyRef.destroyed || generation !== this.attributeFilterGeneration || this.filters().category_code !== code) return;
      this.seo.responseForApiError(error);
      this.attributeFilters.set([]);
      this.error.set('No fue posible cargar los filtros de esta categoría.');
    });
  }
}

function decodeFilters(params: ParamMap): { filters: ExploreFilters | null; error: string } {
  const singleValue = (key: string, fallback = ''): string | null => {
    const values = params.getAll(key);
    return values.length > 1 ? null : (values[0] ?? fallback);
  };
  const q = singleValue('q');
  const category = singleValue('category_code');
  const unit = singleValue('rate_unit');
  const minimum = singleValue('min_unit_price_clp');
  const maximum = singleValue('max_unit_price_clp');
  const sort = singleValue('sort', 'default');
  const rawSize = singleValue('page_size');
  const cursor = singleValue('cursor');
  const region = singleValue('region_code'); const comuna = singleValue('comuna_code');
  const area = singleValue('min_area_m2'); const start = singleValue('start_at'); const end = singleValue('end_at');
  const minTotal = singleValue('min_total_clp'); const maxTotal = singleValue('max_total_clp');
  const rawAttributes = params.getAll('attribute');
  const attributes = decodePublicAttributeFilters(rawAttributes);
  if (attributes === null) return { filters: null, error: rawAttributes.length > 32 ? 'La URL supera el máximo de filtros.' : 'Los atributos de la URL no son válidos.' };
  if (Object.keys(attributes).length > 16) return { filters: null, error: 'La URL supera el máximo de filtros.' };
  if ([q, category, unit, minimum, maximum, sort, rawSize, cursor, region, comuna, area, start, end, minTotal, maxTotal].some((value) => value === null))
    return { filters: null, error: 'La URL contiene parámetros públicos duplicados.' };
  if (!isSort(sort!)) return { filters: null, error: 'El orden de la URL no es válido.' };
  const pageSize = rawSize === '' ? 20 : Number(rawSize);
  if (rawSize !== '' && !/^\d+$/.test(rawSize!))
    return { filters: null, error: 'El tamaño de página de la URL no es válido.' };
  return {
    filters: {
      q: q!,
      category_code: category!,
      rate_unit: unit!,
      min_unit_price_clp: minimum!,
      max_unit_price_clp: maximum!,
      sort: sort!,
      page_size: pageSize,
      cursor: cursor!,
      region_code: region!, comuna_code: comuna!, min_area_m2: area!, start_at: start!, end_at: end!,
      min_total_clp: minTotal!, max_total_clp: maxTotal!, attributes,
    },
    error: '',
  };
}

function encodeFilters(filters: ExploreFilters): Record<string, string | string[] | null> {
  // Repeated attribute values are serialized below by Angular Router as an array.
  return {
    q: filters.q || null,
    category_code: filters.category_code || null,
    rate_unit: filters.rate_unit || null,
    min_unit_price_clp: filters.min_unit_price_clp || null,
    max_unit_price_clp: filters.max_unit_price_clp || null,
    sort: filters.sort === 'default' ? null : filters.sort,
    page_size: filters.page_size === 20 ? null : String(filters.page_size),
    cursor: filters.cursor || null,
    region_code: filters.region_code || null, comuna_code: filters.comuna_code || null,
    min_area_m2: filters.min_area_m2 || null, start_at: filters.start_at || null, end_at: filters.end_at || null,
    min_total_clp: filters.min_total_clp || null, max_total_clp: filters.max_total_clp || null,
    attribute: encodePublicAttributeFilters(filters.attributes),
  };
}

function sameOwnedQuery(params: ParamMap, target: Record<string, string | string[] | null>): boolean {
  const targetKeys = new Set(Object.keys(target));
  return (
    params.keys.every((key) => targetKeys.has(key)) &&
    Object.entries(target).every(([key, value]) => Array.isArray(value)
      ? JSON.stringify(params.getAll(key)) === JSON.stringify(value)
      : (params.get(key) || null) === value)
  );
}

function criteriaSignature(filters: ExploreFilters): string {
  return JSON.stringify([
    filters.category_code,
    filters.q, filters.region_code, filters.comuna_code, filters.min_area_m2, filters.start_at, filters.end_at,
    filters.min_total_clp, filters.max_total_clp, filters.attributes,
    filters.rate_unit,
    filters.min_unit_price_clp,
    filters.max_unit_price_clp,
    filters.sort,
    filters.page_size,
  ]);
}

function validPrice(value: string): boolean {
  return !value || /^(0|[1-9]\d*)$/.test(value);
}

function isSort(value: string): boolean {
  return ['default', 'title_asc', 'title_desc', 'unit_price_asc', 'unit_price_desc', 'relevance', 'estimated_total_asc', 'estimated_total_desc'].includes(value);
}
