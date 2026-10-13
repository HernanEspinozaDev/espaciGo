import { DestroyRef, Injectable, OnInit, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, ParamMap, Router } from '@angular/router';
import {
  ExplorePort,
  PublicCatalogFilters,
  PublicCategory,
  PublicSpace,
} from '../domain/ports/explore-port';

type ExploreFilters = {
  category_code: string;
  rate_unit: string;
  min_unit_price_clp: string;
  max_unit_price_clp: string;
  sort: string;
  page_size: number;
  cursor: string;
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

  readonly categories = signal<PublicCategory[]>([]);
  readonly items = signal<PublicSpace[]>([]);
  readonly nextCursor = signal('');
  readonly busy = signal(false);
  readonly error = signal('');
  readonly filters = signal<ExploreFilters>({
    category_code: '',
    rate_unit: '',
    min_unit_price_clp: '',
    max_unit_price_clp: '',
    sort: 'default',
    page_size: 20,
    cursor: '',
  });

  constructor() {
    this.title.setTitle('Explorar espacios | EspaciGo');
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => this.meta.removeTag("name='robots'"));
  }

  ngOnInit(): void {
    this.route.queryParamMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      void this.loadCategories();
      this.loadFromRoute(params);
    });
  }

  updateFilter<K extends ExploreCriterion>(key: K, value: ExploreFilters[K]): void {
    this.filters.update((filters) => ({ ...filters, [key]: value, cursor: '' }));
    this.invalidateCriteria();
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
        replaceUrl: true,
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

  clearFilters(): void {
    this.filters.set({
      category_code: '',
      rate_unit: '',
      min_unit_price_clp: '',
      max_unit_price_clp: '',
      sort: 'default',
      page_size: 20,
      cursor: '',
    });
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
    this.filters.set(filters);
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
      rate_unit: filters.rate_unit || undefined,
      min_unit_price_clp: filters.min_unit_price_clp || undefined,
      max_unit_price_clp: filters.max_unit_price_clp || undefined,
      sort: filters.sort,
      page_size: filters.page_size,
      cursor: filters.cursor || undefined,
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
      (filters.min_unit_price_clp || filters.max_unit_price_clp || filters.sort !== 'default') &&
      !filters.rate_unit
    ) {
      this.error.set('Elige hora, día o mes para comparar tarifas.');
      this.busy.set(false);
      return false;
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
}

function decodeFilters(params: ParamMap): { filters: ExploreFilters | null; error: string } {
  const singleValue = (key: string, fallback = ''): string | null => {
    const values = params.getAll(key);
    return values.length > 1 ? null : (values[0] ?? fallback);
  };
  const category = singleValue('category_code');
  const unit = singleValue('rate_unit');
  const minimum = singleValue('min_unit_price_clp');
  const maximum = singleValue('max_unit_price_clp');
  const sort = singleValue('sort', 'default');
  const rawSize = singleValue('page_size');
  const cursor = singleValue('cursor');
  if ([category, unit, minimum, maximum, sort, rawSize, cursor].some((value) => value === null))
    return { filters: null, error: 'La URL contiene parámetros públicos duplicados.' };
  const pageSize = rawSize === '' ? 20 : Number(rawSize);
  if (rawSize !== '' && !/^\d+$/.test(rawSize!))
    return { filters: null, error: 'El tamaño de página de la URL no es válido.' };
  return {
    filters: {
      category_code: category!,
      rate_unit: unit!,
      min_unit_price_clp: minimum!,
      max_unit_price_clp: maximum!,
      sort: sort!,
      page_size: pageSize,
      cursor: cursor!,
    },
    error: '',
  };
}

function encodeFilters(filters: ExploreFilters): Record<string, string | null> {
  return {
    category_code: filters.category_code || null,
    rate_unit: filters.rate_unit || null,
    min_unit_price_clp: filters.min_unit_price_clp || null,
    max_unit_price_clp: filters.max_unit_price_clp || null,
    sort: filters.sort === 'default' ? null : filters.sort,
    page_size: filters.page_size === 20 ? null : String(filters.page_size),
    cursor: filters.cursor || null,
  };
}

function sameOwnedQuery(params: ParamMap, target: Record<string, string | null>): boolean {
  const targetKeys = new Set(Object.keys(target));
  return (
    params.keys.every((key) => targetKeys.has(key)) &&
    Object.entries(target).every(([key, value]) => (params.get(key) || null) === value)
  );
}

function criteriaSignature(filters: ExploreFilters): string {
  return JSON.stringify([
    filters.category_code,
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
