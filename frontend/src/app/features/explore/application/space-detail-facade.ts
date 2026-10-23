import { DestroyRef, Injectable, OnInit, effect, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, Router, UrlTree } from '@angular/router';
import { PublicPageSeo } from '../../../core/public-page-seo';
import type { SpaceAttributeProfile } from '../../../core/contracts/models';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { buildContextualReturn, canonicalCatalogReturn } from '../../../core/identity/contextual-return';
import { BookingAttempts } from '../../bookings/application/booking-attempts';
import { BookingApiError, BookingPort } from '../../bookings/domain/ports/booking-port';
import {
  AvailabilityInterval,
  BookingQuote,
  CatalogItem,
  ExplorePort,
  PublicSpace,
  PublicSpaceDetail,
} from '../domain/ports/explore-port';

@Injectable()
export class SpaceDetailFacade implements OnInit {
  private readonly api = inject(ExplorePort);
  private readonly bookingApi = inject(BookingPort);
  private readonly bookingAttempts = inject(BookingAttempts);
  private readonly router = inject(Router);
  private readonly identity = inject(IdentitySession);
  private readonly route = inject(ActivatedRoute);
  private readonly meta = inject(Meta);
  private readonly title = inject(Title);
  private readonly destroyRef = inject(DestroyRef);
  private readonly seo = inject(PublicPageSeo);
  private generation = 0;
  private reservationGeneration = 0;
  private activeSpaceId = '';
  private activeSession = this.identity.session();
  private activeToken = this.identity.accessToken();

  readonly space = signal<(PublicSpaceDetail & Partial<CatalogItem>) | null>(null);
  readonly privateSpace = signal<CatalogItem | null>(null);
  readonly profile = signal<SpaceAttributeProfile | null>(null);
  readonly date = signal('');
  readonly duration = signal(1);
  readonly options = signal<AvailabilityInterval[]>([]);
  readonly selected = signal<AvailabilityInterval | null>(null);
  readonly quote = signal<BookingQuote | null>(null);
  readonly notice = signal('ENSAYO LOCAL — SIN COBRO REAL');
  readonly busy = signal(false);
  readonly error = signal('');
  readonly requestingReservation = signal(false);

  constructor() {
    this.title.setTitle('Detalle de espacio | EspaciGo');
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => this.meta.removeTag("name='robots'"));
    effect(() => {
      const session = this.identity.session();
      const token = this.identity.accessToken();
      if (session !== this.activeSession || token !== this.activeToken) {
        this.activeSession = session;
        this.activeToken = token;
        this.reservationGeneration++;
        this.requestingReservation.set(false);
        if (this.space()) {
          this.generation++;
          this.profile.set(null);
          this.privateSpace.set(null);
          this.options.set([]);
          this.selected.set(null);
          this.quote.set(null);
          this.date.set('');
          this.duration.set(1);
          this.busy.set(false);
          this.error.set('La sesión cambió. La información pública sigue disponible.');
        }
        if (this.canBook()) void this.loadPrivateBookingData();
      }
    });
  }

  ngOnInit(): void {
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      const id = params.get('id') ?? '';
      if (id && id !== this.activeSpaceId) void this.loadSpace(id);
    });
  }

  async retryPublicDetail(): Promise<void> {
    if (this.activeSpaceId) await this.loadSpace(this.activeSpaceId);
  }

  async chooseDate(value: string): Promise<void> {
    this.date.set(value);
    this.invalidateInterval();
    await this.loadAvailability();
  }

  async chooseDuration(value: string): Promise<void> {
    const duration = Number(value);
    if (!this.durationOptions().includes(duration)) return;
    this.duration.set(duration);
    this.invalidateInterval();
    await this.loadAvailability();
  }

  chooseInterval(value: string): void {
    ++this.generation;
    this.busy.set(false);
    const index = value === '' ? Number.NaN : Number(value);
    this.selected.set(
      Number.isInteger(index) && index >= 0 && index < this.options().length
        ? this.options()[index]
        : null,
    );
    this.quote.set(null);
  }

  durationOptions(): number[] {
    switch (this.privateSpace()?.rate_unit) {
      case 'hora':
        return [1, 2, 4];
      case 'dia':
        return [1, 2, 3];
      case 'mes':
        return [1];
      default:
        return [];
    }
  }

  async loadAvailability(): Promise<void> {
    const space = this.privateSpace();
    const date = this.date();
    const token = this.identity.accessToken();
    const session = this.identity.session();
    if (!space || !date || !token || !session) return;
    const generation = ++this.generation;
    this.error.set('');
    this.busy.set(true);
    try {
      const response = await this.api.availability(token, space.space_id, date, this.duration());
      if (!this.isCurrent(generation, token, session, space.space_id)) return;
      if (
        response.data.space_id !== space.space_id ||
        response.data.time_zone !== space.time_zone
      ) {
        this.error.set('Go devolvió horarios para otra zona o espacio.');
        return;
      }
      this.options.set(response.data.items);
    } catch (error) {
      if (this.handleUnauthorized(error, token)) return;
      if (this.isCurrent(generation, token, session, space.space_id))
        this.error.set(apiError(error));
    } finally {
      if (this.isCurrent(generation, token, session, space.space_id)) this.busy.set(false);
    }
  }

  async createQuote(): Promise<void> {
    const space = this.privateSpace();
    const interval = this.selected();
    const token = this.identity.accessToken();
    const session = this.identity.session();
    if (!space || !interval || !token || !session) return;
    const generation = ++this.generation;
    const signature = `${space.space_id}|${interval.start_at}|${interval.end_at}`;
    this.error.set('');
    this.quote.set(null);
    this.busy.set(true);
    try {
      const response = await this.api.quote(
        token,
        space.space_id,
        interval.start_at,
        interval.end_at,
      );
      if (!this.isCurrent(generation, token, session, space.space_id)) return;
      if (
        `${response.data.space_id}|${response.data.start_at}|${response.data.end_at}` !== signature
      ) {
        this.error.set(
          'Go devolvió una cotización para una selección distinta. Elige el horario nuevamente.',
        );
        return;
      }
      this.quote.set(response.data);
      this.notice.set(response.safety_notice);
    } catch (error) {
      if (this.handleUnauthorized(error, token)) return;
      if (this.isCurrent(generation, token, session, space.space_id))
        this.error.set(apiError(error));
    } finally {
      if (this.isCurrent(generation, token, session, space.space_id)) this.busy.set(false);
    }
  }

  async requestReservation(): Promise<void> {
    const quote = this.quote();
    const token = this.identity.accessToken();
    const session = this.identity.session();
    if (!quote || !token || !session || this.requestingReservation()) return;
    const generation = this.generation;
    let attempt;
    try {
      attempt = this.bookingAttempts.get(session.account_id, `quote:${quote.id}`, {
        quote_id: quote.id,
      });
    } catch (error) {
      this.error.set(apiError(error));
      return;
    }
    this.requestingReservation.set(true);
    const reservationGeneration = ++this.reservationGeneration;
    this.error.set('');
    try {
      const result = await this.bookingApi.requestReservation(token, quote.id, attempt.key);
      this.bookingAttempts.settle(session.account_id, `quote:${quote.id}`, attempt.key);
      if (!this.isCurrent(generation, token, session, quote.space_id)) return;
      this.notice.set(result.safety_notice);
      await this.router.navigate(['/reservas', result.data.id]);
    } catch (error) {
      if (error instanceof BookingApiError && error.status === 401) {
        this.identity.expireIfCurrent(token);
        return;
      }
      if (
        error instanceof BookingApiError &&
        error.status !== undefined &&
        error.status > 0 &&
        error.status < 500
      ) {
        this.bookingAttempts.settle(session.account_id, `quote:${quote.id}`, attempt.key);
      }
      if (this.isCurrent(generation, token, session, quote.space_id))
        this.error.set(apiError(error));
    } finally {
      if (reservationGeneration === this.reservationGeneration)
        this.requestingReservation.set(false);
    }
  }

  formatInterval(interval: AvailabilityInterval): string {
    const zone = this.privateSpace()?.time_zone ?? 'UTC';
    const formatter = new Intl.DateTimeFormat('es-CL', {
      timeZone: zone,
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      timeZoneName: 'short',
    });
    return `${formatter.format(new Date(interval.start_at))} – ${formatter.format(new Date(interval.end_at))} (${zone})`;
  }

  formatClp(amount: string): string {
    return `CLP ${BigInt(amount).toLocaleString('es-CL')}`;
  }
  formatDateTime(value: string): string {
    return new Intl.DateTimeFormat('es-CL', {
      timeZone: this.privateSpace()?.time_zone ?? 'UTC',
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      timeZoneName: 'short',
    }).format(new Date(value));
  }
  attributeLabel(code: string): string {
    return this.profile()?.attributes.find((item) => item.code === code)?.label ?? code;
  }
  attributeValue(value: unknown): string {
    if (typeof value === 'boolean') return value ? 'Sí' : 'No';
    if (Array.isArray(value)) return value.join(', ');
    return String(value ?? '—');
  }

  publicImageUrl(imageId:string):string { const item=this.space(); return item ? this.api.publicImageUrl(item.space_id,imageId) : ''; }

  private async loadSpace(id: string): Promise<void> {
    this.activeSpaceId = id;
    const generation = ++this.generation;
    this.space.set(null);
    this.privateSpace.set(null);
    this.profile.set(null);
    this.options.set([]);
    this.selected.set(null);
    this.quote.set(null);
    this.date.set('');
    this.duration.set(1);
    this.error.set('');
    this.busy.set(true);
    try {
      const item = await this.api.publicDetail(id);
      if (generation !== this.generation || this.activeSpaceId !== id) return;
      this.space.set(item);
      this.seo.detail({
        id: item.space_id,
        title: item.title,
        description: item.description,
        imageId: item.images?.find((image) => image.is_cover)?.image_id,
        contextual: this.route.snapshot.queryParamMap.keys.length > 0,
      });
      if (this.canBook()) await this.loadPrivateBookingData();
    } catch (error) {
      if (generation === this.generation && this.activeSpaceId === id) {
        this.seo.responseForApiError(error);
        if (apiStatus(error) === 404) this.seo.unavailable();
        this.error.set(apiError(error));
      }
    } finally {
      if (generation === this.generation && this.activeSpaceId === id) this.busy.set(false);
    }
  }

  canBook(): boolean {
    const session=this.identity.session();
    return !!this.identity.accessToken() && !!session && !session.restricted_mode && session.roles.includes('arrendatario');
  }

  get session() {
    return this.identity.session;
  }

  get identityReturnUrl(): string | null {
    const item = this.space();
    if (!item) return null;
    const values = this.route.snapshot?.queryParamMap?.getAll('returnTo') ?? [];
    const listing = values.length === 1 ? canonicalCatalogReturn(values[0]) : null;
    return buildContextualReturn(item.space_id, listing ?? '/explorar');
  }

  get listingReturnUrl(): UrlTree {
    const values = this.route.snapshot?.queryParamMap?.getAll('returnTo') ?? [];
    const listing = values.length === 1 ? canonicalCatalogReturn(values[0]) ?? '/explorar' : '/explorar';
    return this.router.parseUrl(listing);
  }

  get hasReservationContext(): boolean {
    const context = this.route.snapshot?.queryParamMap?.getAll('context') ?? [];
    return context.length === 1 && context[0] === 'returned' && !!this.listingReturnUrl;
  }

  private async loadPrivateBookingData(): Promise<void> {
    const item=this.space();const token=this.identity.accessToken();const session=this.identity.session();
    if(!item||!token||!session||!this.canBook())return;
    const generation=++this.generation;const id=item.space_id;this.busy.set(true);this.error.set('');
    this.options.set([]);this.selected.set(null);this.quote.set(null);
    try {
      const full=await this.api.detail(token,id);
      if(!this.isCurrent(generation,token,session,id))return;
      this.privateSpace.set(full);
      const profile=await this.api.profile(token,full.category_code);
      if(!this.isCurrent(generation,token,session,id))return;
      this.profile.set(profile);
      const clock=await this.api.businessClock(token);
      if(!this.isCurrent(generation,token,session,id))return;
      this.date.set(localDateInZone(full.time_zone,clock.data.now));
      await this.loadAvailability();
    } catch(error) {
      if(this.handleUnauthorized(error,token))return;
      if(this.isCurrent(generation,token,session,id))this.error.set(apiError(error));
    } finally {
      if(this.isCurrent(generation,token,session,id))this.busy.set(false);
    }
  }

  private invalidateInterval(): void {
    ++this.generation;
    this.selected.set(null);
    this.quote.set(null);
    this.options.set([]);
  }

  private handleUnauthorized(error: unknown, token: string): boolean {
    if (apiStatus(error) !== 401 || !this.identity.expireIfCurrent(token)) return false;
    ++this.generation;
    this.privateSpace.set(null);
    this.profile.set(null);
    this.options.set([]);
    this.selected.set(null);
    this.quote.set(null);
    this.date.set('');
    this.duration.set(1);
    this.busy.set(false);
    this.error.set('Go cerró la sesión. Inicia sesión nuevamente.');
    return true;
  }

  private isCurrent(
    generation: number,
    token: string,
    session: ReturnType<IdentitySession['session']>,
    spaceId: string,
  ): boolean {
    return (
      generation === this.generation &&
      this.activeSpaceId === spaceId &&
      this.identity.accessToken() === token &&
      this.identity.session() === session
    );
  }
}

function localDateInZone(timeZone: string, instant: string): string {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(new Date(instant));
  const value = (type: string) => parts.find((part) => part.type === type)?.value ?? '';
  return `${value('year')}-${value('month')}-${value('day')}`;
}
function apiError(error: unknown): string {
  const status = apiStatus(error);
  if (status === 401) return 'Go cerró la sesión. Inicia sesión nuevamente.';
  if (status === 403) return 'Go confirmó que esta cuenta no puede consultar este espacio.';
  if (status === 404) return 'Este espacio ya no está disponible.';
  if (status === 409) return 'El horario cambió. Consulta disponibilidad nuevamente.';
  if (status === 422) return 'Go rechazó el horario seleccionado. Elige otra opción.';
  return 'No fue posible completar la operación. Inténtalo nuevamente.';
}
function apiStatus(error: unknown): number | undefined {
  return (error as { status?: number })?.status;
}
