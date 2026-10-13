import { DestroyRef, Injectable, OnInit, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, ParamMap, Router } from '@angular/router';
import {
  ExplorePort,
  PublicCatalogFilters,
  PublicCategory,
  PublicSpace,
  PublicLocation,
  PublicAttributeFilter,
} from '../domain/ports/explore-port';

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
  attributes: Record<string, boolean>;
};
type ExploreCriterion = Exclude<keyof ExploreFilters, 'cursor'>;

@Injectable()
export class ExploreFacade implements OnInit {
  private readonly api = inject(ExplorePort);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly meta = inject(Meta);
  private readonly title = inject(Title);
  private readonly destroyRef = inject(DestroyRef);
  private requestGeneration = 0;
  private criteriaKey: string | undefined;
  private routeRequestKey: string | undefined;
  private pageCursors: string[] = [];
  private readonly pages = new Map<string, PublicSpace[]>();
  private readonly pageNextCursors = new Map<string, string>();
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

  setAttribute(code: string, value: boolean | null): void {
    const attributes = { ...this.filters().attributes };
    if (value === null) delete attributes[code]; else attributes[code] = value;
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
    const filters = { ...this.filters(), cursor: '' };
    if (!this.validateForSearch(filters)) return;
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

  applySort(sort: string): void {
    this.filters.set({ ...this.appliedFilters(), sort, cursor: '' });
    this.invalidateCriteria();
    void this.search();
  }

  clearFilters(): void {
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
      this.pages.clear();
      this.pageNextCursors.clear();
      this.pageCursors = [];
      this.items.set([]);
      this.nextCursor.set('');
    }
    this.routeRequestKey = requestKey;
    if (!this.validateForSearch(filters)) return;
    const cachedIndex = this.pageCursors.indexOf(filters.cursor);
    if (!criteriaChanged && cachedIndex >= 0) {
      this.items.set(this.pageCursors.slice(0, cachedIndex + 1).flatMap((cursor) => this.pages.get(cursor) ?? []));
      const cached = this.pages.get(filters.cursor) ?? [];
      this.nextCursor.set(cached.length ? (this.pageNextCursors.get(filters.cursor) ?? '') : '');
      this.busy.set(false);
      return;
    }
    void this.loadPage(filters, !criteriaChanged && this.items().length > 0);
  }

  private async loadPage(filters: ExploreFilters, append: boolean): Promise<void> {
    const generation = ++this.requestGeneration;
    this.error.set('');
    this.busy.set(true);
    if (!append && this.pages.size === 0) this.items.set([]);
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
      this.pages.set(filters.cursor, page.items);
      this.pageNextCursors.set(filters.cursor, page.next_cursor ?? '');
      if (!this.pageCursors.includes(filters.cursor)) this.pageCursors.push(filters.cursor);
      this.items.update((current) => (append ? [...current, ...page.items] : page.items));
      this.nextCursor.set(page.next_cursor ?? '');
    } catch {
      if (generation === this.requestGeneration)
        this.error.set('No fue posible cargar los espacios públicos. Inténtalo nuevamente.');
    } finally {
      if (generation === this.requestGeneration) this.busy.set(false);
    }
  }

  private invalidateCriteria(): void {
    this.requestGeneration++;
    this.routeRequestKey = undefined;
    this.pages.clear();
    this.pageNextCursors.clear();
    this.pageCursors = [];
    this.items.set([]);
    this.nextCursor.set('');
    this.error.set('');
    this.busy.set(false);
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
    if (
      (filters.min_unit_price_clp || filters.max_unit_price_clp || filters.sort === 'unit_price_asc' || filters.sort === 'unit_price_desc') &&
      !filters.rate_unit
    ) {
      this.error.set('Elige hora, día o mes para comparar tarifas.');
      this.busy.set(false);
      return false;
    }
    if ((filters.sort === 'relevance' && !filters.q.trim()) ||
        ((filters.min_total_clp || filters.max_total_clp || filters.sort.startsWith('estimated_total_')) && (!filters.start_at || !filters.end_at))) {
      this.error.set('La relevancia requiere texto y el total estimado requiere un intervalo.');
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
    } catch {
      this.error.set('No fue posible cargar las categorías públicas.');
    }
  }

  async loadLocations(): Promise<void> {
    try { this.locations.set(await this.api.publicLocations()); }
    catch { this.error.set('No fue posible cargar las ubicaciones públicas.'); }
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
    }).catch(() => {
      if (this.destroyRef.destroyed || generation !== this.attributeFilterGeneration || this.filters().category_code !== code) return;
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
  const attributes: Record<string, boolean> = {};
  for (const entry of params.getAll('attribute')) {
    const split = entry.lastIndexOf(':');
    if (split < 1 || !/^[a-z][a-z0-9_]{0,79}$/.test(entry.slice(0, split)) || !['true','false'].includes(entry.slice(split + 1)) || attributes[entry.slice(0, split)] !== undefined)
      return { filters: null, error: 'Los atributos de la URL no son válidos.' };
    attributes[entry.slice(0, split)] = entry.slice(split + 1) === 'true';
  }
  if (Object.keys(attributes).length > 8) return { filters: null, error: 'La URL supera el máximo de filtros.' };
  if ([q, category, unit, minimum, maximum, sort, rawSize, cursor, region, comuna, area, start, end, minTotal, maxTotal].some((value) => value === null))
    return { filters: null, error: 'La URL contiene parámetros públicos duplicados.' };
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
    attribute: Object.entries(filters.attributes).sort(([a],[b]) => a.localeCompare(b)).map(([k,v]) => `${k}:${v}`),
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
