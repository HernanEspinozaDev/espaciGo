import { Component, DestroyRef, OnInit, effect, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, RouterLink } from '@angular/router';
import type { SpaceAttributeProfile } from '../../../generated/api/types.gen';
import { IdentitySession } from '../../core/identity/identity-session';
import { AvailabilityInterval, BookingQuote, CatalogItem, ExploreApi } from './explore-api';

@Component({
  selector: 'app-space-detail-page',
  imports: [RouterLink],
  templateUrl: './space-detail-page.html',
  styleUrl: './space-detail-page.scss',
})
export class SpaceDetailPage implements OnInit {
  private readonly api = inject(ExploreApi);
  private readonly identity = inject(IdentitySession);
  private readonly route = inject(ActivatedRoute);
  private readonly meta = inject(Meta);
  private readonly title = inject(Title);
  private readonly destroyRef = inject(DestroyRef);
  private generation = 0;
  private activeSpaceId = '';
  private activeSession = this.identity.session();
  private activeToken = this.identity.accessToken();

  protected readonly space = signal<CatalogItem | null>(null);
  protected readonly profile = signal<SpaceAttributeProfile | null>(null);
  protected readonly date = signal('');
  protected readonly duration = signal(1);
  protected readonly options = signal<AvailabilityInterval[]>([]);
  protected readonly selected = signal<AvailabilityInterval | null>(null);
  protected readonly quote = signal<BookingQuote | null>(null);
  protected readonly notice = signal('ENSAYO LOCAL — SIN COBRO REAL');
  protected readonly busy = signal(false);
  protected readonly error = signal('');

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
        this.generation++;
        this.space.set(null);
        this.profile.set(null);
        this.options.set([]);
        this.selected.set(null);
        this.quote.set(null);
        this.date.set('');
        this.duration.set(1);
        this.busy.set(false);
        this.error.set('La sesión cambió. Vuelve a abrir el espacio desde la búsqueda.');
      }
    });
  }

  ngOnInit(): void {
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      const id = params.get('id') ?? '';
      if (id && id !== this.activeSpaceId) void this.loadSpace(id);
    });
  }

  protected async chooseDate(value: string): Promise<void> {
    this.date.set(value);
    this.invalidateInterval();
    await this.loadAvailability();
  }

  protected async chooseDuration(value: string): Promise<void> {
    const duration = Number(value);
    if (!this.durationOptions().includes(duration)) return;
    this.duration.set(duration);
    this.invalidateInterval();
    await this.loadAvailability();
  }

  protected chooseInterval(value: string): void {
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

  protected durationOptions(): number[] {
    switch (this.space()?.rate_unit) {
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

  protected async loadAvailability(): Promise<void> {
    const space = this.space();
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

  protected async createQuote(): Promise<void> {
    const space = this.space();
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

  protected formatInterval(interval: AvailabilityInterval): string {
    const zone = this.space()?.time_zone ?? 'UTC';
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

  protected formatClp(amount: string): string {
    return `CLP ${BigInt(amount).toLocaleString('es-CL')}`;
  }
  protected formatDateTime(value: string): string {
    return new Intl.DateTimeFormat('es-CL', {
      timeZone: this.space()?.time_zone ?? 'UTC',
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      timeZoneName: 'short',
    }).format(new Date(value));
  }
  protected attributeLabel(code: string): string {
    return this.profile()?.attributes.find((item) => item.code === code)?.label ?? code;
  }
  protected attributeValue(value: unknown): string {
    if (typeof value === 'boolean') return value ? 'Sí' : 'No';
    if (Array.isArray(value)) return value.join(', ');
    return String(value ?? '—');
  }

  private async loadSpace(id: string): Promise<void> {
    this.activeSpaceId = id;
    const token = this.identity.accessToken();
    const session = this.identity.session();
    const generation = ++this.generation;
    this.space.set(null);
    this.profile.set(null);
    this.options.set([]);
    this.selected.set(null);
    this.quote.set(null);
    this.date.set('');
    this.duration.set(1);
    this.error.set('');
    if (!token || !session || session.restricted_mode || !session.roles.includes('arrendatario')) {
      this.error.set('Go no confirmó permisos de arrendatario para ver este espacio.');
      return;
    }
    this.busy.set(true);
    try {
      const item = await this.api.detail(token, id);
      if (!this.isCurrent(generation, token, session, id)) return;
      this.space.set(item);
      if (!this.durationOptions().includes(this.duration())) this.duration.set(1);
      this.title.setTitle(`${item.title} | EspaciGo`);
      const profile = await this.api.profile(token, item.category_code);
      if (!this.isCurrent(generation, token, session, id)) return;
      this.profile.set(profile);
      this.date.set(localDateInZone(item.time_zone));
      await this.loadAvailability();
    } catch (error) {
      if (this.handleUnauthorized(error, token)) return;
      if (this.isCurrent(generation, token, session, id)) this.error.set(apiError(error));
    } finally {
      if (this.isCurrent(generation, token, session, id)) this.busy.set(false);
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
    this.space.set(null);
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

function localDateInZone(timeZone: string): string {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(new Date());
  const value = (type: string) => parts.find((part) => part.type === type)?.value ?? '';
  return `${value('year')}-${value('month')}-${value('day')}`;
}
function apiError(error: unknown): string {
  const status = apiStatus(error);
  if (status === 401) return 'Go cerró la sesión. Inicia sesión nuevamente.';
  if (status === 403) return 'Go confirmó que esta cuenta no puede consultar este espacio.';
  if (status === 404) return 'Este espacio ya no está disponible para tu cuenta.';
  if (status === 409) return 'El horario cambió. Consulta disponibilidad nuevamente.';
  if (status === 422) return 'Go rechazó el horario seleccionado. Elige otra opción.';
  return 'No fue posible completar la operación. Inténtalo nuevamente.';
}
function apiStatus(error: unknown): number | undefined {
  return (error as { status?: number })?.status;
}
