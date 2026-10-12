import { DestroyRef, Injectable, OnInit, effect, inject, signal } from '@angular/core';
import { Meta, Title } from '@angular/platform-browser';
import type { SpaceAttributeProfile, SpaceCategoryList } from '../../../core/contracts/models';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { CatalogFilters, CatalogItem, ExplorePort } from '../domain/ports/explore-port';

@Injectable()
export class ExploreFacade implements OnInit {
  private readonly api = inject(ExplorePort);
  private readonly identity = inject(IdentitySession);
  private readonly meta = inject(Meta);
  private readonly title = inject(Title);
  private readonly destroyRef = inject(DestroyRef);
  private requestGeneration = 0;
  private profileGeneration = 0;
  private ownerSession = this.identity.session();
  private ownerToken = this.identity.accessToken();

  readonly categories = signal<SpaceCategoryList['items']>([]);
  readonly profile = signal<SpaceAttributeProfile | null>(null);
  readonly items = signal<CatalogItem[]>([]);
  readonly nextCursor = signal<string | undefined>(undefined);
  readonly busy = signal(false);
  readonly error = signal('');
  readonly filters = signal({
    q: '',
    category_code: '',
    min_area_m2: '',
    start_local_utc: '',
    end_local_utc: '',
    min_total_clp: '',
    max_total_clp: '',
    latitude: '',
    longitude: '',
    radius_km: '',
  });
  readonly attributeFilters = signal<Record<string, string | boolean>>({});
  readonly mapView = signal(false);
  readonly safetyNotice = 'ENSAYO LOCAL — SIN COBRO REAL';

  constructor() {
    this.title.setTitle('Explorar espacios | EspaciGo');
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => this.meta.removeTag("name='robots'"));
    // Any account/token transition invalidates results and cursors held by this page.
    effect(() => {
      const session = this.identity.session();
      const token = this.identity.accessToken();
      if (session !== this.ownerSession || token !== this.ownerToken) {
        this.ownerSession = session;
        this.ownerToken = token;
        this.invalidate();
        this.clearPrivateResults();
      }
    });
  }

  ngOnInit(): void {
    const token = this.identity.accessToken();
    const session = this.identity.session();
    if (!token || !session || session.restricted_mode || !session.roles.includes('arrendatario')) {
      this.error.set('Go no confirmó permisos de arrendatario para explorar.');
      return;
    }
    this.busy.set(true);
    this.api
      .categories(token)
      .then((response) => {
        if (!this.isSameSession(token, session)) return;
        this.categories.set(response.items);
      })
      .catch((error) => {
        if (this.handleUnauthorized(error, token)) return;
        if (this.isSameSession(token, session)) this.error.set(errorMessage(error));
      })
      .finally(() => {
        if (this.isSameSession(token, session)) this.busy.set(false);
      });
  }

  updateFilter<K extends keyof ReturnType<typeof this.filters>>(
    key: K,
    value: ReturnType<typeof this.filters>[K],
  ): void {
    this.filters.update((filters) => ({ ...filters, [key]: value }));
    this.invalidateCriteria();
    if (key === 'category_code') {
      this.attributeFilters.set({});
      this.profile.set(null);
      const token = this.identity.accessToken();
      const category = value as string;
      if (token && category) {
        const owner = this.identity.session();
        const profileGeneration = ++this.profileGeneration;
        this.api
          .profile(token, category)
          .then((profile) => {
            if (
              profileGeneration === this.profileGeneration &&
              this.isSameSession(token, owner) &&
              this.filters().category_code === category
            )
              this.profile.set(profile);
          })
          .catch((error) => {
            if (this.handleUnauthorized(error, token)) return;
            if (
              profileGeneration === this.profileGeneration &&
              this.isSameSession(token, owner) &&
              this.filters().category_code === category
            )
              this.error.set(errorMessage(error));
          });
      }
    }
  }

  setAttribute(code: string, value: string | boolean): void {
    this.attributeFilters.update((filters) => ({ ...filters, [code]: value }));
    this.invalidateCriteria();
  }

  async search(reset = true): Promise<void> {
    const token = this.identity.accessToken();
    const session = this.identity.session();
    if (!token || !session || session.restricted_mode || !session.roles.includes('arrendatario')) {
      this.error.set('Necesitas una sesión de arrendatario confirmada por Go.');
      return;
    }
    const generation = ++this.requestGeneration;
    this.error.set('');
    this.busy.set(true);
    if (reset) {
      this.items.set([]);
      this.nextCursor.set(undefined);
    }
    const values = this.filters();
    const dateInterval = values.start_local_utc || values.end_local_utc;
    if (dateInterval && (!values.start_local_utc || !values.end_local_utc)) {
      this.error.set('Completa el inicio y término UTC para filtrar por total.');
      this.busy.set(false);
      return;
    }
    const filters: CatalogFilters = {
      q: values.q.trim() || undefined,
      category_code: values.category_code || undefined,
      min_area_m2: values.min_area_m2 || undefined,
      start_at: dateInterval ? utcDateTime(values.start_local_utc) : undefined,
      end_at: dateInterval ? utcDateTime(values.end_local_utc) : undefined,
      min_total_clp: values.min_total_clp || undefined,
      max_total_clp: values.max_total_clp || undefined,
      latitude: values.latitude || undefined,
      longitude: values.longitude || undefined,
      radius_km: values.radius_km || undefined,
      attributes: this.typedAttributes(),
      page_size: 8,
      cursor: reset ? undefined : this.nextCursor(),
    };
    if ((filters.min_total_clp || filters.max_total_clp) && !dateInterval) {
      this.error.set('El rango de total requiere un intervalo UTC.');
      this.busy.set(false);
      return;
    }
    if (!validClp(filters.min_total_clp) || !validClp(filters.max_total_clp)) {
      this.error.set('Ingresa importes CLP enteros válidos.');
      this.busy.set(false);
      return;
    }
    const proximity = [values.latitude, values.longitude, values.radius_km];
    if (proximity.some(Boolean) && !proximity.every(Boolean)) {
      this.error.set('Completa latitud, longitud y radio para filtrar por proximidad.');
      this.busy.set(false);
      return;
    }
    if (
      values.latitude &&
      (!Number.isFinite(Number(values.latitude)) ||
        Number(values.latitude) < -90 ||
        Number(values.latitude) > 90)
    ) {
      this.error.set('La latitud debe estar entre -90 y 90.');
      this.busy.set(false);
      return;
    }
    if (
      values.longitude &&
      (!Number.isFinite(Number(values.longitude)) ||
        Number(values.longitude) < -180 ||
        Number(values.longitude) > 180)
    ) {
      this.error.set('La longitud debe estar entre -180 y 180.');
      this.busy.set(false);
      return;
    }
    if (values.radius_km && !['1', '3', '5', '10', '25'].includes(values.radius_km)) {
      this.error.set('El radio debe ser uno de los valores disponibles.');
      this.busy.set(false);
      return;
    }
    try {
      const result = await this.api.search(token, filters);
      if (!this.isCurrent(generation, token, session)) return;
      const incoming = result.data.items;
      this.items.update((current) => (reset ? incoming : [...current, ...incoming]));
      this.nextCursor.set(result.data.next_cursor);
    } catch (error) {
      if (this.handleUnauthorized(error, token)) return;
      if (this.isCurrent(generation, token, session)) this.error.set(errorMessage(error));
    } finally {
      if (this.isCurrent(generation, token, session)) this.busy.set(false);
    }
  }

  clearFilters(): void {
    this.invalidate();
    ++this.profileGeneration;
    this.filters.set({
      q: '',
      category_code: '',
      min_area_m2: '',
      start_local_utc: '',
      end_local_utc: '',
      min_total_clp: '',
      max_total_clp: '',
      latitude: '',
      longitude: '',
      radius_km: '',
    });
    this.attributeFilters.set({});
    this.profile.set(null);
    this.items.set([]);
    this.nextCursor.set(undefined);
    void this.search(true);
  }

  toggleMap(): void {
    this.mapView.update((value) => !value);
  }
  loadMore(): void {
    if (this.nextCursor() && !this.busy()) void this.search(false);
  }
  attributeValue(code: string): string | boolean {
    return this.attributeFilters()[code] ?? '';
  }
  attributeLabel(code: string): string {
    return this.profile()?.attributes.find((attribute) => attribute.code === code)?.label ?? code;
  }
  displayValue(value: unknown): string {
    if (typeof value === 'boolean') return value ? 'Sí' : 'No';
    if (Array.isArray(value)) return value.join(', ');
    return String(value ?? '—');
  }
  price(value: string | undefined): string {
    return value === undefined ? '' : formatClp(value);
  }
  position(item: CatalogItem): string {
    return item.map_position ? `left:${item.map_position.x}%;top:${item.map_position.y}%` : '';
  }

  private invalidate(): void {
    ++this.requestGeneration;
    this.busy.set(false);
  }
  private invalidateCriteria(): void {
    this.invalidate();
    this.items.set([]);
    this.nextCursor.set(undefined);
    this.error.set('');
  }
  private clearPrivateResults(): void {
    ++this.profileGeneration;
    this.categories.set([]);
    this.profile.set(null);
    this.items.set([]);
    this.nextCursor.set(undefined);
    this.filters.set({
      q: '',
      category_code: '',
      min_area_m2: '',
      start_local_utc: '',
      end_local_utc: '',
      min_total_clp: '',
      max_total_clp: '',
      latitude: '',
      longitude: '',
      radius_km: '',
    });
    this.attributeFilters.set({});
  }
  private handleUnauthorized(error: unknown, token: string): boolean {
    if (statusCode(error) !== 401 || !this.identity.expireIfCurrent(token)) return false;
    this.invalidate();
    this.clearPrivateResults();
    this.error.set('Go cerró la sesión. Inicia sesión nuevamente.');
    return true;
  }
  private typedAttributes(): Record<string, unknown> {
    const typed: Record<string, unknown> = {};
    for (const [code, value] of Object.entries(this.attributeFilters())) {
      if (value === '') continue;
      const type = this.profile()?.attributes.find((attribute) => attribute.code === code)?.type;
      if (type === 'boolean') {
        typed[code] = value === true || value === 'true';
        continue;
      }
      if (type === 'integer' || type === 'number') {
        const number = Number(value);
        if (Number.isFinite(number)) typed[code] = number;
        continue;
      }
      typed[code] = type === 'enum_list' ? [String(value)] : String(value);
    }
    return typed;
  }
  private isSameSession(token: string, session: ReturnType<IdentitySession['session']>): boolean {
    return this.identity.accessToken() === token && this.identity.session() === session;
  }
  private isCurrent(
    generation: number,
    token: string,
    session: ReturnType<IdentitySession['session']>,
  ): boolean {
    return generation === this.requestGeneration && this.isSameSession(token, session);
  }
}

function utcDateTime(value: string): string {
  return new Date(`${value}Z`).toISOString();
}
function validClp(value: string | undefined): boolean {
  return !value || /^(0|[1-9]\d*)$/.test(value);
}
function formatClp(value: string): string {
  const parsed = BigInt(value);
  return `CLP ${parsed.toLocaleString('es-CL')}`;
}
function errorMessage(error: unknown): string {
  const status = statusCode(error);
  if (status === 401) return 'Go cerró la sesión. Inicia sesión nuevamente.';
  if (status === 403) return 'Go confirmó que esta cuenta no tiene permiso para explorar.';
  if (status === 422)
    return 'Go rechazó los filtros o el cursor. Limpia los filtros y vuelve a buscar.';
  return 'No fue posible completar la búsqueda. Inténtalo nuevamente.';
}
function statusCode(error: unknown): number | undefined {
  return (error as { status?: number })?.status;
}
