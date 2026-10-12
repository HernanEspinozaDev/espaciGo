import {
  DestroyRef,
  ElementRef,
  Injectable,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import type {
  LocalBookingDetail,
  LocalReservationMessage,
  LocalReview,
} from '../../../core/contracts/models';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../../core/identity/private-session-state';
import {
  BookingCommunicationPort,
  type ReportReason,
} from '../domain/ports/booking-communication-port';
import { BookingApiError, BookingPort } from '../domain/ports/booking-port';
import { BookingCommunicationAttempts } from './booking-communication-attempts';
type Context = { token: string; accountId: string; reservationId: string; generation: number };
const SEND_STATES = new Set([
  'pendiente_de_pago',
  'pagada',
  'aprobada_host',
  'firma_parcial',
  'lista_para_checkin',
]);
const REVIEW_STATES = new Set(['finalizada', 'cerrada']);
const REPORT_REASONS: Array<{ value: ReportReason; label: string }> = [
  { value: 'insultos_acoso', label: 'Insultos o acoso' },
  { value: 'datos_personales', label: 'Datos personales' },
  { value: 'spam', label: 'Spam' },
  { value: 'ajeno_experiencia', label: 'Ajeno a la experiencia' },
];
@Injectable()
export class BookingConversationFacade {
  private readonly reservationSource = signal<(() => LocalBookingDetail) | null>(null);
  readonly reservation = computed(() => {
    const source = this.reservationSource();
    if (!source) throw new Error('Reservation source must be connected by the view.');
    return source();
  });
  private notifyStateChanged: () => void = () => undefined;
  connectReservation(source: () => LocalBookingDetail, notify: () => void): void {
    this.notifyStateChanged = notify;
    this.reservationSource.set(source);
  }
  readonly stateChanged = { emit: () => this.notifyStateChanged() };
  private readonly identity = inject(IdentitySession);
  private readonly privateState = inject(PrivateSessionState);
  private readonly api = inject(BookingCommunicationPort);
  private readonly bookingApi = inject(BookingPort);
  private readonly attempts = inject(BookingCommunicationAttempts);
  private readonly destroyRef = inject(DestroyRef);
  private messageListSource: () => ElementRef<HTMLElement> | undefined = () => undefined;
  private readonly messageList = () => this.messageListSource();
  connectMessageList(source: () => ElementRef<HTMLElement> | undefined): void {
    this.messageListSource = source;
  }
  private token: string | null = null;
  private owner: string | null = null;
  private reservationId = '';
  private generation = 0;
  private messageRevision = 0;
  private spaceReviewRevision = 0;
  private reviewRevision = 0;
  private unreadRevision = 0;
  private destroyed = false;
  private readonly unregister = this.privateState.register(() => this.clearPrivate());

  readonly messages = signal<LocalReservationMessage[]>([]);
  readonly olderCursor = signal<number | null>(null);
  readonly body = signal('');
  readonly loading = signal(false);
  readonly olderLoading = signal(false);
  readonly sending = signal(false);
  readonly error = signal('');
  readonly notice = signal('');
  readonly readThrough = signal(0);
  readonly unreadCount = signal(0);
  readonly reviews = signal<LocalReview[]>([]);
  readonly reviewLoading = signal(false);
  readonly reviewError = signal('');
  readonly reviewNotice = signal('');
  readonly reviewSending = signal(false);
  readonly reviewComment = signal('');
  readonly rating = signal(5);
  readonly reportReasons = REPORT_REASONS;
  readonly reportReasonsByReview = signal<Record<string, ReportReason>>({});
  readonly reportBusy = signal('');
  readonly reported = signal<Record<string, string>>({});
  readonly reputation = signal<{ average?: number; count?: number } | null>(null);
  readonly reputationLoading = signal(false);
  readonly reputationError = signal('');
  readonly spaceReviews = signal<{ items: LocalReview[]; average: number; count: number } | null>(
    null,
  );
  readonly accountId = computed(() => this.identity.session()?.account_id ?? '');
  readonly writable = computed(
    () => this.isParticipant() && SEND_STATES.has(this.reservation().state),
  );
  readonly REVIEW_STATES = REVIEW_STATES;

  constructor() {
    this.destroyRef.onDestroy(() => {
      this.destroyed = true;
      this.generation++;
      this.messageRevision++;
      this.reviewRevision++;
      this.spaceReviewRevision++;
      this.unregister();
    });
    effect(() => {
      const item = this.reservation();
      const token = this.identity.accessToken();
      const account = this.identity.session()?.account_id ?? null;
      if (token !== this.token || account !== this.owner || item.id !== this.reservationId) {
        this.token = token;
        this.owner = account;
        this.reservationId = item.id;
        this.generation++;
        this.messageRevision++;
        this.reviewRevision++;
        this.spaceReviewRevision++;
        this.clearPrivate();
        const context = this.context();
        if (context && this.isParticipant()) {
          void this.loadRecent(context);
          void this.loadUnreadCount(context);
          void this.loadReviews(context);
          if (this.isRenter()) void this.loadReputation(context);
          void this.loadSpaceReviews(context);
        }
      }
    });
  }

  async loadRecent(context = this.context()): Promise<void> {
    if (!context || !this.isParticipant()) return;
    const revision = ++this.messageRevision;
    // A refresh supersedes any previous pagination operation and releases its control.
    this.olderLoading.set(false);
    this.loading.set(true);
    this.error.set('');
    try {
      const result = await this.api.messages(context.token, context.reservationId);
      if (!this.current(context) || revision !== this.messageRevision) return;
      this.messages.set(result.data.items);
      this.olderCursor.set(result.data.older_cursor);
      this.loading.set(false);
      const newest = result.data.items.at(-1);
      if (
        newest &&
        result.data.items.some((item) => item.author_id !== context.accountId) &&
        newest.sequence > this.readThrough()
      ) {
        setTimeout(() => {
          void this.advanceVisibleRead(context, newest.sequence, revision);
        }, 0);
      }
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (this.current(context) && revision === this.messageRevision)
        this.error.set(messageFor(error));
    } finally {
      if (this.current(context) && revision === this.messageRevision) this.loading.set(false);
    }
  }

  async loadOlder(): Promise<void> {
    const context = this.context();
    const before = this.olderCursor();
    const revision = this.messageRevision;
    if (!context || before === null || this.olderLoading()) return;
    this.olderLoading.set(true);
    this.error.set('');
    try {
      const result = await this.api.messages(context.token, context.reservationId, before);
      if (!this.current(context) || revision !== this.messageRevision) return;
      const known = new Set(this.messages().map((item) => item.id));
      this.messages.update((items) =>
        [...result.data.items.filter((item) => !known.has(item.id)), ...items].sort(
          (a, b) => a.sequence - b.sequence,
        ),
      );
      this.olderCursor.set(result.data.older_cursor);
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (this.current(context) && revision === this.messageRevision)
        this.error.set(messageFor(error));
    } finally {
      if (this.current(context) && revision === this.messageRevision) this.olderLoading.set(false);
    }
  }

  async send(): Promise<void> {
    const context = this.context();
    if (!context || !this.writable()) return;
    const scope = `${context.accountId}:${context.reservationId}:message`;
    const prior = this.attempts.find(scope);
    const payload = prior
      ? (JSON.parse(prior.payload) as { body: string })
      : { body: this.body().trim() };
    if (!payload.body || payload.body.length > 2000) {
      this.error.set('Escribe un mensaje de 1 a 2.000 caracteres.');
      return;
    }
    let attempt;
    try {
      attempt = this.attempts.get(scope, payload);
    } catch (error) {
      this.error.set(messageFor(error));
      return;
    }
    this.sending.set(true);
    this.error.set('');
    this.notice.set('');
    try {
      const result = await this.api.sendMessage(
        context.token,
        context.reservationId,
        payload.body,
        attempt.key,
      );
      if (!this.current(context)) return;
      this.attempts.settle(scope, attempt.key);
      this.messages.update((items) =>
        [...items.filter((item) => item.id !== result.data.id), result.data].sort(
          (a, b) => a.sequence - b.sequence,
        ),
      );
      this.body.set('');
      this.notice.set(result.safety_notice);
      this.stateChanged.emit();
      void this.loadUnreadCount(context);
      await this.loadRecent(context);
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (!this.current(context)) return;
      if (isDefinitive(error)) this.attempts.settle(scope, attempt.key);
      this.error.set(
        isUncertain(error)
          ? `${messageFor(error)} La respuesta es incierta; puedes reintentar el mismo contenido.`
          : messageFor(error),
      );
    } finally {
      if (this.current(context)) this.sending.set(false);
    }
  }

  async loadReviews(context = this.context()): Promise<void> {
    if (!context || !this.isParticipant()) return;
    const revision = ++this.reviewRevision;
    this.reviewLoading.set(true);
    this.reviewError.set('');
    try {
      const result = await this.api.reservationReviews(context.token, context.reservationId);
      if (!this.current(context) || revision !== this.reviewRevision) return;
      this.reviews.set(result.data.items);
      this.reviewNotice.set(result.notice);
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (this.current(context) && revision === this.reviewRevision)
        this.reviewError.set(messageFor(error));
    } finally {
      if (this.current(context) && revision === this.reviewRevision) this.reviewLoading.set(false);
    }
  }

  async submitReview(): Promise<void> {
    const context = this.context();
    if (!context || !this.canReview() || this.hasOwnReview()) return;
    const scope = `${context.accountId}:${context.reservationId}:review`;
    const prior = this.attempts.find(scope);
    const payload = prior
      ? (JSON.parse(prior.payload) as { rating: number; comment?: string })
      : { rating: this.rating(), comment: this.reviewComment().trim() };
    let attempt;
    try {
      attempt = this.attempts.get(scope, payload);
    } catch (error) {
      this.reviewError.set(messageFor(error));
      return;
    }
    this.reviewSending.set(true);
    this.reviewError.set('');
    this.reviewNotice.set('');
    try {
      const result = await this.api.createReview(
        context.token,
        context.reservationId,
        payload,
        attempt.key,
      );
      if (!this.current(context)) return;
      this.attempts.settle(scope, attempt.key);
      this.reviewComment.set('');
      this.reviewNotice.set(
        result.data.reused
          ? 'Go reutilizó la reseña ya registrada; es inmutable.'
          : 'Go confirmó la reseña sintética; no se puede editar.',
      );
      await this.loadReviews(context);
      await this.loadSpaceReviews(context);
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (!this.current(context)) return;
      if (isDefinitive(error)) this.attempts.settle(scope, attempt.key);
      this.reviewError.set(
        isUncertain(error)
          ? `${messageFor(error)} La respuesta es incierta; reintenta la solicitud original.`
          : messageFor(error),
      );
    } finally {
      if (this.current(context)) this.reviewSending.set(false);
    }
  }

  async report(review: LocalReview): Promise<void> {
    const context = this.context();
    if (!context || !this.canReport(review)) return;
    const scope = `${context.accountId}:${context.reservationId}:report:${review.id}`;
    const prior = this.attempts.find(scope);
    const payload = prior
      ? (JSON.parse(prior.payload) as { reason: ReportReason })
      : { reason: this.reportReason(review.id) };
    let attempt;
    try {
      attempt = this.attempts.get(scope, payload);
    } catch (error) {
      this.reviewError.set(messageFor(error));
      return;
    }
    this.reportBusy.set(review.id);
    this.reviewError.set('');
    try {
      const result = await this.api.reportReview(
        context.token,
        context.reservationId,
        review.id,
        payload.reason,
        attempt.key,
      );
      if (!this.current(context)) return;
      this.attempts.settle(scope, attempt.key);
      this.reported.update((items) => ({
        ...items,
        [review.id]: `Reporte ${result.data.state}; Go conserva el promedio hasta la decisión administrativa.`,
      }));
      await this.loadReviews(context);
      await this.loadSpaceReviews(context);
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (!this.current(context)) return;
      if (isDefinitive(error)) this.attempts.settle(scope, attempt.key);
      this.reviewError.set(
        isUncertain(error)
          ? `${messageFor(error)} Reintenta el reporte original.`
          : messageFor(error),
      );
    } finally {
      if (this.current(context)) this.reportBusy.set('');
    }
  }

  async loadReputation(context = this.context()): Promise<void> {
    if (!context || !this.isRenter()) return;
    this.reputationLoading.set(true);
    this.reputationError.set('');
    try {
      const result = (await this.api.ownReputation(context.token)) as {
        data?: { average?: number; count?: number };
      };
      if (this.current(context)) this.reputation.set(result.data ?? { count: 0, average: 0 });
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (this.current(context)) this.reputationError.set(messageFor(error));
    } finally {
      if (this.current(context)) this.reputationLoading.set(false);
    }
  }

  async loadSpaceReviews(context = this.context()): Promise<void> {
    if (!context) return;
    const revision = ++this.spaceReviewRevision;
    try {
      const result = await this.api.spaceReviews(this.reservation().space_id);
      if (this.current(context) && revision === this.spaceReviewRevision)
        this.spaceReviews.set(result.data);
    } catch {
      if (this.current(context) && revision === this.spaceReviewRevision)
        this.spaceReviews.set(null); // Hidden/inactive publications return 404.
    }
  }

  canReview(): boolean {
    return this.isParticipant() && REVIEW_STATES.has(this.reservation().state);
  }
  hasOwnReview(): boolean {
    const target = this.isRenter() ? 'espacio' : 'arrendatario';
    return this.reviews().some((item) => item.target_type === target);
  }
  canReport(review: LocalReview): boolean {
    return this.isHost() && review.target_type === 'espacio' && review.state !== 'reportada';
  }
  reportReason(reviewId: string): ReportReason {
    return this.reportReasonsByReview()[reviewId] ?? 'insultos_acoso';
  }
  setReportReason(reviewId: string, value: string): void {
    this.reportReasonsByReview.update((items) => ({ ...items, [reviewId]: value as ReportReason }));
  }
  sendAttempt(): boolean {
    const c = this.context();
    return !!c && !!this.attempts.find(`${c.accountId}:${c.reservationId}:message`);
  }
  reviewAttempt(): boolean {
    const c = this.context();
    return !!c && !!this.attempts.find(`${c.accountId}:${c.reservationId}:review`);
  }
  reportAttempt(reviewId: string): boolean {
    const c = this.context();
    return !!c && !!this.attempts.find(`${c.accountId}:${c.reservationId}:report:${reviewId}`);
  }
  isRenter(): boolean {
    return this.identity.session()?.account_id === this.reservation().renter_id;
  }
  isHost(): boolean {
    return this.identity.session()?.account_id === this.reservation().host_id;
  }
  isParticipant(): boolean {
    return this.isRenter() || this.isHost();
  }
  when(value: string): string {
    return new Intl.DateTimeFormat('es-CL', {
      dateStyle: 'medium',
      timeStyle: 'short',
      timeZone: this.reservation().time_zone,
    }).format(new Date(value));
  }

  private async advanceVisibleRead(
    context: Context,
    sequence: number,
    revision: number,
  ): Promise<void> {
    const element = this.messageList()?.nativeElement.querySelector(
      `[data-sequence="${sequence}"]`,
    );
    if (
      !element ||
      !this.current(context) ||
      revision !== this.messageRevision ||
      sequence <= this.readThrough()
    )
      return;
    try {
      const result = await this.api.advanceRead(context.token, context.reservationId, sequence);
      if (!this.current(context) || revision !== this.messageRevision) return;
      this.readThrough.update((current) => Math.max(current, result.data.read_through_sequence));
      await this.loadUnreadCount(context);
      if (this.current(context)) this.stateChanged.emit();
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (this.current(context) && revision === this.messageRevision)
        this.error.set(messageFor(error));
    }
  }
  private async loadUnreadCount(context: Context): Promise<void> {
    const revision = ++this.unreadRevision;
    try {
      const result = await this.bookingApi.list(context.token);
      if (!this.current(context) || revision !== this.unreadRevision) return;
      this.unreadCount.set(
        result.data.items.find((item) => item.id === context.reservationId)?.unread_count ?? 0,
      );
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      // The detail remains usable if the inbox counter cannot be refreshed.
    }
  }
  private context(): Context | null {
    const token = this.identity.accessToken();
    const accountId = this.identity.session()?.account_id ?? null;
    return token &&
      accountId &&
      accountId === this.owner &&
      this.reservation().id === this.reservationId
      ? { token, accountId, reservationId: this.reservationId, generation: this.generation }
      : null;
  }
  private current(context: Context): boolean {
    return (
      !this.destroyed &&
      context.generation === this.generation &&
      context.token === this.identity.accessToken() &&
      context.accountId === this.identity.session()?.account_id &&
      context.reservationId === this.reservation().id
    );
  }
  private unauthorized(error: unknown, context: Context): boolean {
    return error instanceof BookingApiError &&
      error.status === 401 &&
      this.identity.accessToken() === context.token &&
      this.identity.session()?.account_id === context.accountId
      ? this.identity.expireIfCurrent(context.token)
      : false;
  }
  private clearPrivate(): void {
    this.unreadRevision++;
    this.messages.set([]);
    this.olderCursor.set(null);
    this.body.set('');
    this.loading.set(false);
    this.olderLoading.set(false);
    this.sending.set(false);
    this.error.set('');
    this.notice.set('');
    this.readThrough.set(0);
    this.reviews.set([]);
    this.reviewLoading.set(false);
    this.reviewError.set('');
    this.unreadCount.set(0);
    this.reviewNotice.set('');
    this.reviewSending.set(false);
    this.reviewComment.set('');
    this.rating.set(5);
    this.reportReasonsByReview.set({});
    this.reportBusy.set('');
    this.reported.set({});
    this.reputation.set(null);
    this.reputationLoading.set(false);
    this.reputationError.set('');
    this.spaceReviews.set(null);
  }
}

function messageFor(error: unknown): string {
  return error instanceof Error ? error.message : 'Go no pudo completar esta operación.';
}
function isDefinitive(error: unknown): boolean {
  return (
    error instanceof BookingApiError &&
    error.status !== undefined &&
    error.status >= 400 &&
    error.status < 500
  );
}
function isUncertain(error: unknown): boolean {
  return (
    !(error instanceof BookingApiError) ||
    error.status === undefined ||
    error.status === 0 ||
    error.status >= 500
  );
}
