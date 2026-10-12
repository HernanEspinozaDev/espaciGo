import { Component, DestroyRef, computed, effect, inject, input, output, signal, viewChild, ElementRef } from '@angular/core';
import type { LocalBookingDetail, LocalReservationMessage, LocalReview, LocalReviewReport } from '../../../generated/api/types.gen';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { BookingApi, BookingApiError } from './booking-api';
import { BookingCommunicationApi, BookingCommunicationAttempts, type ReportReason } from './booking-communication-api';

type Context = { token: string; accountId: string; reservationId: string; generation: number };
const SEND_STATES = new Set(['pendiente_de_pago', 'pagada', 'aprobada_host', 'firma_parcial', 'lista_para_checkin']);
const REVIEW_STATES = new Set(['finalizada', 'cerrada']);
const REPORT_REASONS: Array<{ value: ReportReason; label: string }> = [
  { value: 'insultos_acoso', label: 'Insultos o acoso' }, { value: 'datos_personales', label: 'Datos personales' },
  { value: 'spam', label: 'Spam' }, { value: 'ajeno_experiencia', label: 'Ajeno a la experiencia' },
];

@Component({
  selector: 'app-booking-conversation-panel',
  template: `
    <section class="panel conversation" aria-labelledby="conversation-title">
      <header class="heading"><div><h2 id="conversation-title">Conversación de la reserva</h2><p>Solo visible para anfitrión y arrendatario. Los mensajes se muestran como texto.</p></div>
        @if (unreadCount() > 0) { <span class="badge" aria-label="Mensajes sin leer">{{ unreadCount() }} sin leer</span> }
      </header>
      @if (loading()) { <p role="status">Consultando mensajes confirmados por Go…</p> }
      @if (error()) { <p class="error" role="alert">{{ error() }}</p> }
      @if (notice()) { <p class="success" role="status">{{ notice() }}</p> }
      <div #messageList class="messages" aria-live="polite" aria-relevant="additions text">
        @if (!messages().length && !loading()) { <p>Aún no hay mensajes en esta conversación.</p> }
        @for (message of messages(); track message.id) {
          <article class="message" [class.own]="message.author_id === accountId()" [attr.data-sequence]="message.sequence">
            <p class="meta">{{ message.author_id === accountId() ? 'Tú' : 'Participante' }} · {{ when(message.created_at) }} · #{{ message.sequence }}</p>
            <p class="body">{{ message.body }}</p>
          </article>
        }
      </div>
      @if (olderCursor() !== null) { <button class="button secondary" type="button" [disabled]="olderLoading() || loading()" (click)="loadOlder()">{{ olderLoading() ? 'Cargando…' : 'Ver mensajes anteriores' }}</button> }
      @if (writable()) {
        <label for="message-body">Nuevo mensaje</label>
        <textarea id="message-body" [value]="body()" (input)="body.set($any($event.target).value)" rows="3" maxlength="2000" aria-describedby="message-help"></textarea>
        <p id="message-help" class="hint">Hasta 2.000 caracteres. No incluyas datos sensibles.</p>
        <button class="button" type="button" [disabled]="sending() || !body().trim()" (click)="send()">{{ sending() ? 'Enviando…' : sendAttempt() ? 'Reintentar mensaje original' : 'Enviar mensaje' }}</button>
      } @else {
        <p class="readonly" role="note">Esta reserva está en modo de solo lectura. Go conserva el acceso al historial.</p>
      }
      <button class="text-button" type="button" [disabled]="loading()" (click)="loadRecent()">Actualizar conversación</button>
    </section>

    <section class="panel reviews" aria-labelledby="review-title">
      <h2 id="review-title">Reseñas y reportes</h2>
      <p>Contenido de ensayo. Go define la elegibilidad y conserva las reseñas como registros inmutables.</p>
      @if (reviewLoading()) { <p role="status">Consultando reseñas…</p> }
      @if (reviewError()) { <p class="error" role="alert">{{ reviewError() }}</p> }
      @if (!reviews().length && !reviewLoading()) { <p>No hay reseñas visibles para esta reserva.</p> }
      @for (review of reviews(); track review.id) {
        <article class="review">
          <h3>{{ review.target_type === 'espacio' ? 'Reseña del arrendatario al espacio' : 'Reseña del anfitrión al arrendatario' }} · {{ review.rating }}/5</h3>
          <p class="review-copy">{{ review.comment || 'Sin comentario.' }}</p>
          <p class="meta">{{ review.state ?? 'publicada' }} · {{ when(review.created_at) }}</p>
          @if (reported()[review.id]; as reportStatus) { <p role="status">{{ reportStatus }}</p> }
          @if (canReport(review)) {
            <label [for]="'report-' + review.id">Motivo del reporte</label>
            <select [id]="'report-' + review.id" [value]="reportReason(review.id)" (change)="setReportReason(review.id, $any($event.target).value)">
              @for (reason of reportReasons; track reason.value) { <option [value]="reason.value">{{ reason.label }}</option> }
            </select>
            <button class="button secondary" type="button" [disabled]="reportBusy() === review.id" (click)="report(review)">{{ reportBusy() === review.id ? 'Enviando…' : reportAttempt(review.id) ? 'Reintentar reporte original' : 'Reportar reseña' }}</button>
          } @else if (review.target_type === 'espacio' && isHost() && review.state === 'reportada') {
            <p class="readonly">Reportada; pendiente de revisión administrativa. Su promedio depende de la decisión de Go.</p>
          }
        </article>
      }
      @if (reviewNotice()) { <p class="success" role="status">{{ reviewNotice() }}</p> }
      @if (canReview()) {
        @if (hasOwnReview()) { <p class="readonly">Tu reseña ya fue enviada y no se puede editar.</p> }
        @else {
          <form (submit)="$event.preventDefault(); submitReview()">
            <h3>Dejar una reseña</h3>
            <label for="review-rating">Calificación</label>
            <select id="review-rating" [value]="rating()" (change)="rating.set(+$any($event.target).value)" [disabled]="reviewSending()"><option [value]="5">5 · Excelente</option><option [value]="4">4 · Muy buena</option><option [value]="3">3 · Buena</option><option [value]="2">2 · Regular</option><option [value]="1">1 · Mala</option></select>
            <label for="review-comment">Comentario (opcional)</label>
            <textarea id="review-comment" [value]="reviewComment()" (input)="reviewComment.set($any($event.target).value)" rows="3" maxlength="2000" [disabled]="reviewSending()"></textarea>
            <button class="button" type="submit" [disabled]="reviewSending()">{{ reviewSending() ? 'Enviando…' : reviewAttempt() ? 'Reintentar reseña original' : 'Enviar reseña' }}</button>
          </form>
        }
      } @else if (reservation().state === 'en_disputa') { <p class="readonly">La reserva está en disputa. No se admiten reseñas nuevas.</p> }
      @else if (!REVIEW_STATES.has(reservation().state)) { <p class="hint">Go solo admite reseñas después de finalizar o cerrar la reserva.</p> }
    </section>

    @if (isRenter()) {
      <section class="panel reputation" aria-labelledby="reputation-title">
        <h2 id="reputation-title">Tu reputación</h2>
        @if (reputationLoading()) { <p role="status">Consultando calificaciones recibidas…</p> }
        @if (reputationError()) { <p class="error" role="alert">{{ reputationError() }}</p> }
        @if (reputation(); as value) {
          <p>Promedio recibido: {{ value.average ?? 0 }}/5 · {{ value.count ?? 0 }} calificación(es).</p>
          @if (!value.count) { <p>Aún no tienes calificaciones recibidas.</p> }
        }
      </section>
    }
    @if (spaceReviews(); as value) {
      <section class="panel public-reviews" aria-labelledby="public-review-title">
        <h2 id="public-review-title">Reseñas visibles del espacio</h2><p>Promedio {{ value.average }}/5 · {{ value.count }} reseña(s).</p>
        @for (review of value.items; track review.id) { <article class="review"><h3>{{ review.rating }}/5</h3><p class="review-copy">{{ review.comment || 'Sin comentario.' }}</p></article> }
        @if (!value.count) { <p>El espacio no tiene reseñas públicas visibles.</p> }
      </section>
    }
  `,
  styles: [`
    :host{display:block}.panel{border:1px solid #e8ddd0;border-radius:1rem;background:#fffdf8;padding:clamp(1rem,3vw,1.5rem);margin-bottom:1rem;min-width:0}.panel h2{margin-top:0}.panel p,.panel li{line-height:1.55;color:#62584f;overflow-wrap:anywhere}.heading{display:flex;align-items:flex-start;justify-content:space-between;gap:1rem}.heading p{margin-top:.3rem}.badge{flex:none;border-radius:99px;background:#f7e7d8;color:#713b2c;padding:.4rem .7rem;font-weight:800}.messages{display:grid;gap:.65rem;max-height:32rem;overflow:auto;padding:.2rem}.message,.review{border:1px solid #eadfd4;border-radius:.8rem;padding:.8rem;min-width:0}.message{max-width:min(92%,42rem);justify-self:start;background:#f7efe6}.message.own{justify-self:end;background:#f5e5dc}.message p,.review p{margin:.35rem 0}.meta,.hint{font-size:.88rem;color:#756b61}.body,.review-copy{white-space:pre-wrap;overflow-wrap:anywhere}.button,.text-button{min-height:2.8rem;border:0;border-radius:999px;padding:.65rem 1rem;background:#a9573f;color:#fffaf3;font:inherit;font-weight:800;cursor:pointer;margin:.55rem .45rem .2rem 0}.button.secondary{background:#f1e4d7;color:#713b2c}.button:disabled,.text-button:disabled{opacity:.55;cursor:wait}.text-button{background:transparent;color:#713b2c;text-decoration:underline;padding-inline:.25rem}label{display:grid;gap:.4rem;margin:.65rem 0 .35rem;color:#51483f;font-weight:700}textarea,select{box-sizing:border-box;width:100%;max-width:100%;border:1px solid #c9b9a7;border-radius:.55rem;padding:.65rem;background:#fff;font:inherit}textarea{resize:vertical}.readonly{padding:.75rem;border-radius:.6rem;background:#f7efe6}.error{padding:.8rem;background:#fff0ed;color:#842d22}.success{padding:.8rem;background:#e8f3e9;color:#245d36}.reviews form{border-top:1px solid #eadfd4;margin-top:1rem;padding-top:1rem}.review{margin:.7rem 0}.review h3{margin:.1rem 0 .45rem;font-size:1rem}@media(max-width:520px){.heading{display:block}.badge{display:inline-block;margin-bottom:.5rem}.message{max-width:100%}}
  `],
})
export class BookingConversationPanel {
  readonly reservation = input.required<LocalBookingDetail>();
  readonly stateChanged = output<void>();
  private readonly identity = inject(IdentitySession);
  private readonly privateState = inject(PrivateSessionState);
  private readonly api = inject(BookingCommunicationApi);
  private readonly bookingApi = inject(BookingApi);
  private readonly attempts = inject(BookingCommunicationAttempts);
  private readonly destroyRef = inject(DestroyRef);
  private readonly messageList = viewChild<ElementRef<HTMLElement>>('messageList');
  private token: string | null = null;
  private owner: string | null = null;
  private reservationId = '';
  private generation = 0;
  private messageRevision = 0;
  private reviewRevision = 0;
  private unreadRevision = 0;
  private destroyed = false;
  private readonly unregister = this.privateState.register(() => this.clearPrivate());

  protected readonly messages = signal<LocalReservationMessage[]>([]);
  protected readonly olderCursor = signal<number | null>(null);
  protected readonly body = signal('');
  protected readonly loading = signal(false);
  protected readonly olderLoading = signal(false);
  protected readonly sending = signal(false);
  protected readonly error = signal('');
  protected readonly notice = signal('');
  protected readonly readThrough = signal(0);
  protected readonly unreadCount = signal(0);
  protected readonly reviews = signal<LocalReview[]>([]);
  protected readonly reviewLoading = signal(false);
  protected readonly reviewError = signal('');
  protected readonly reviewNotice = signal('');
  protected readonly reviewSending = signal(false);
  protected readonly reviewComment = signal('');
  protected readonly rating = signal(5);
  protected readonly reportReasons = REPORT_REASONS;
  protected readonly reportReasonsByReview = signal<Record<string, ReportReason>>({});
  protected readonly reportBusy = signal('');
  protected readonly reported = signal<Record<string, string>>({});
  protected readonly reputation = signal<{ average?: number; count?: number } | null>(null);
  protected readonly reputationLoading = signal(false);
  protected readonly reputationError = signal('');
  protected readonly spaceReviews = signal<{ items: LocalReview[]; average: number; count: number } | null>(null);
  protected readonly accountId = computed(() => this.identity.session()?.account_id ?? '');
  protected readonly writable = computed(() => this.isParticipant() && SEND_STATES.has(this.reservation().state));
  protected readonly REVIEW_STATES = REVIEW_STATES;

  constructor() {
    this.destroyRef.onDestroy(() => { this.destroyed = true; this.generation++; this.messageRevision++; this.reviewRevision++; this.unregister(); });
    effect(() => {
      const item = this.reservation();
      const token = this.identity.accessToken();
      const account = this.identity.session()?.account_id ?? null;
      if (token !== this.token || account !== this.owner || item.id !== this.reservationId) {
        this.token = token; this.owner = account; this.reservationId = item.id;
        this.generation++; this.messageRevision++; this.reviewRevision++; this.clearPrivate();
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

  protected async loadRecent(context = this.context()): Promise<void> {
    if (!context || !this.isParticipant()) return;
    const revision = ++this.messageRevision;
    this.loading.set(true); this.error.set('');
    try {
      const result = await this.api.messages(context.token, context.reservationId);
      if (!this.current(context) || revision !== this.messageRevision) return;
      this.messages.set(result.data.items);
      this.olderCursor.set(result.data.older_cursor);
      this.loading.set(false);
      const newest = result.data.items.at(-1);
      if (newest && result.data.items.some((item) => item.author_id !== context.accountId) && newest.sequence > this.readThrough()) {
        setTimeout(() => { void this.advanceVisibleRead(context, newest.sequence, revision); }, 0);
      }
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (this.current(context) && revision === this.messageRevision) this.error.set(messageFor(error));
    } finally { if (this.current(context) && revision === this.messageRevision) this.loading.set(false); }
  }

  protected async loadOlder(): Promise<void> {
    const context = this.context(); const before = this.olderCursor(); const revision = this.messageRevision;
    if (!context || before === null || this.olderLoading()) return;
    this.olderLoading.set(true); this.error.set('');
    try {
      const result = await this.api.messages(context.token, context.reservationId, before);
      if (!this.current(context) || revision !== this.messageRevision) return;
      const known = new Set(this.messages().map((item) => item.id));
      this.messages.update((items) => [...result.data.items.filter((item) => !known.has(item.id)), ...items].sort((a,b) => a.sequence-b.sequence));
      this.olderCursor.set(result.data.older_cursor);
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (this.current(context) && revision === this.messageRevision) this.error.set(messageFor(error));
    } finally { if (this.current(context) && revision === this.messageRevision) this.olderLoading.set(false); }
  }

  protected async send(): Promise<void> {
    const context = this.context();
    if (!context || !this.writable()) return;
    const scope = `${context.accountId}:${context.reservationId}:message`;
    const prior = this.attempts.find(scope);
    const payload = prior ? JSON.parse(prior.payload) as { body: string } : { body: this.body().trim() };
    if (!payload.body || payload.body.length > 2000) { this.error.set('Escribe un mensaje de 1 a 2.000 caracteres.'); return; }
    let attempt;
    try { attempt = this.attempts.get(scope, payload); } catch (error) { this.error.set(messageFor(error)); return; }
    this.sending.set(true); this.error.set(''); this.notice.set('');
    try {
      const result = await this.api.sendMessage(context.token, context.reservationId, payload.body, attempt.key);
      if (!this.current(context)) return;
      this.attempts.settle(scope, attempt.key);
      this.messages.update((items) => [...items.filter((item) => item.id !== result.data.id), result.data].sort((a,b) => a.sequence-b.sequence));
      this.body.set(''); this.notice.set(result.safety_notice); this.stateChanged.emit();
      void this.loadUnreadCount(context);
      await this.loadRecent(context);
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (!this.current(context)) return;
      if (isDefinitive(error)) this.attempts.settle(scope, attempt.key);
      this.error.set(isUncertain(error) ? `${messageFor(error)} La respuesta es incierta; puedes reintentar el mismo contenido.` : messageFor(error));
    } finally { if (this.current(context)) this.sending.set(false); }
  }

  protected async loadReviews(context = this.context()): Promise<void> {
    if (!context || !this.isParticipant()) return;
    const revision = ++this.reviewRevision;
    this.reviewLoading.set(true); this.reviewError.set('');
    try {
      const result = await this.api.reservationReviews(context.token, context.reservationId);
      if (!this.current(context) || revision !== this.reviewRevision) return;
      this.reviews.set(result.data.items); this.reviewNotice.set(result.notice);
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (this.current(context) && revision === this.reviewRevision) this.reviewError.set(messageFor(error));
    } finally { if (this.current(context) && revision === this.reviewRevision) this.reviewLoading.set(false); }
  }

  protected async submitReview(): Promise<void> {
    const context = this.context();
    if (!context || !this.canReview() || this.hasOwnReview()) return;
    const scope = `${context.accountId}:${context.reservationId}:review`;
    const prior = this.attempts.find(scope);
    const payload = prior ? JSON.parse(prior.payload) as { rating: number; comment?: string } : { rating: this.rating(), comment: this.reviewComment().trim() };
    let attempt;
    try { attempt = this.attempts.get(scope, payload); } catch (error) { this.reviewError.set(messageFor(error)); return; }
    this.reviewSending.set(true); this.reviewError.set(''); this.reviewNotice.set('');
    try {
      const result = await this.api.createReview(context.token, context.reservationId, payload, attempt.key);
      if (!this.current(context)) return;
      this.attempts.settle(scope, attempt.key); this.reviewComment.set(''); this.reviewNotice.set(result.data.reused ? 'Go reutilizó la reseña ya registrada; es inmutable.' : 'Go confirmó la reseña sintética; no se puede editar.');
      await this.loadReviews(context);
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (!this.current(context)) return;
      if (isDefinitive(error)) this.attempts.settle(scope, attempt.key);
      this.reviewError.set(isUncertain(error) ? `${messageFor(error)} La respuesta es incierta; reintenta la solicitud original.` : messageFor(error));
    } finally { if (this.current(context)) this.reviewSending.set(false); }
  }

  protected async report(review: LocalReview): Promise<void> {
    const context = this.context();
    if (!context || !this.canReport(review)) return;
    const scope = `${context.accountId}:${context.reservationId}:report:${review.id}`;
    const prior = this.attempts.find(scope);
    const payload = prior ? JSON.parse(prior.payload) as { reason: ReportReason } : { reason: this.reportReason(review.id) };
    let attempt;
    try { attempt = this.attempts.get(scope, payload); } catch (error) { this.reviewError.set(messageFor(error)); return; }
    this.reportBusy.set(review.id); this.reviewError.set('');
    try {
      const result = await this.api.reportReview(context.token, context.reservationId, review.id, payload.reason, attempt.key);
      if (!this.current(context)) return;
      this.attempts.settle(scope, attempt.key); this.reported.update((items) => ({ ...items, [review.id]: `Reporte ${result.data.state}; Go conserva el promedio hasta la decisión administrativa.` }));
      await this.loadReviews(context);
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (!this.current(context)) return;
      if (isDefinitive(error)) this.attempts.settle(scope, attempt.key);
      this.reviewError.set(isUncertain(error) ? `${messageFor(error)} Reintenta el reporte original.` : messageFor(error));
    } finally { if (this.current(context)) this.reportBusy.set(''); }
  }

  protected async loadReputation(context = this.context()): Promise<void> {
    if (!context || !this.isRenter()) return;
    this.reputationLoading.set(true); this.reputationError.set('');
    try {
      const result = await this.api.ownReputation(context.token) as { data?: { average?: number; count?: number } };
      if (this.current(context)) this.reputation.set(result.data ?? { count: 0, average: 0 });
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (this.current(context)) this.reputationError.set(messageFor(error));
    } finally { if (this.current(context)) this.reputationLoading.set(false); }
  }

  protected async loadSpaceReviews(context = this.context()): Promise<void> {
    if (!context) return;
    try {
      const result = await this.api.spaceReviews(this.reservation().space_id);
      if (this.current(context)) this.spaceReviews.set(result.data);
    } catch {
      if (this.current(context)) this.spaceReviews.set(null); // Hidden/inactive publications return 404.
    }
  }

  protected canReview(): boolean { return this.isParticipant() && REVIEW_STATES.has(this.reservation().state); }
  protected hasOwnReview(): boolean {
    const target = this.isRenter() ? 'espacio' : 'arrendatario';
    return this.reviews().some((item) => item.target_type === target);
  }
  protected canReport(review: LocalReview): boolean { return this.isHost() && review.target_type === 'espacio' && review.state !== 'reportada'; }
  protected reportReason(reviewId: string): ReportReason { return this.reportReasonsByReview()[reviewId] ?? 'insultos_acoso'; }
  protected setReportReason(reviewId: string, value: string): void { this.reportReasonsByReview.update((items) => ({ ...items, [reviewId]: value as ReportReason })); }
  protected sendAttempt(): boolean { const c = this.context(); return !!c && !!this.attempts.find(`${c.accountId}:${c.reservationId}:message`); }
  protected reviewAttempt(): boolean { const c = this.context(); return !!c && !!this.attempts.find(`${c.accountId}:${c.reservationId}:review`); }
  protected reportAttempt(reviewId: string): boolean { const c = this.context(); return !!c && !!this.attempts.find(`${c.accountId}:${c.reservationId}:report:${reviewId}`); }
  protected isRenter(): boolean { return this.identity.session()?.account_id === this.reservation().renter_id; }
  protected isHost(): boolean { return this.identity.session()?.account_id === this.reservation().host_id; }
  protected isParticipant(): boolean { return this.isRenter() || this.isHost(); }
  protected when(value: string): string { return new Intl.DateTimeFormat('es-CL', { dateStyle: 'medium', timeStyle: 'short', timeZone: this.reservation().time_zone }).format(new Date(value)); }

  private async advanceVisibleRead(context: Context, sequence: number, revision: number): Promise<void> {
    const element = this.messageList()?.nativeElement.querySelector(`[data-sequence="${sequence}"]`);
    if (!element || !this.current(context) || revision !== this.messageRevision || sequence <= this.readThrough()) return;
    try {
      const result = await this.api.advanceRead(context.token, context.reservationId, sequence);
      if (!this.current(context) || revision !== this.messageRevision) return;
      this.readThrough.update((current) => Math.max(current, result.data.read_through_sequence));
      await this.loadUnreadCount(context);
      if (this.current(context)) this.stateChanged.emit();
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      if (this.current(context) && revision === this.messageRevision) this.error.set(messageFor(error));
    }
  }
  private async loadUnreadCount(context: Context): Promise<void> {
    const revision = ++this.unreadRevision;
    try {
      const result = await this.bookingApi.list(context.token);
      if (!this.current(context) || revision !== this.unreadRevision) return;
      this.unreadCount.set(result.data.items.find((item) => item.id === context.reservationId)?.unread_count ?? 0);
    } catch (error) {
      if (this.unauthorized(error, context)) return;
      // The detail remains usable if the inbox counter cannot be refreshed.
    }
  }
  private context(): Context | null {
    const token = this.identity.accessToken(); const accountId = this.identity.session()?.account_id ?? null;
    return token && accountId && accountId === this.owner && this.reservation().id === this.reservationId
      ? { token, accountId, reservationId: this.reservationId, generation: this.generation } : null;
  }
  private current(context: Context): boolean {
    return !this.destroyed && context.generation === this.generation && context.token === this.identity.accessToken()
      && context.accountId === this.identity.session()?.account_id && context.reservationId === this.reservation().id;
  }
  private unauthorized(error: unknown, context: Context): boolean {
    return error instanceof BookingApiError && error.status === 401 && this.identity.accessToken() === context.token
      && this.identity.session()?.account_id === context.accountId ? this.identity.expireIfCurrent(context.token) : false;
  }
  private clearPrivate(): void {
    this.unreadRevision++; this.messages.set([]); this.olderCursor.set(null); this.body.set(''); this.loading.set(false); this.olderLoading.set(false); this.sending.set(false);
    this.error.set(''); this.notice.set(''); this.readThrough.set(0); this.reviews.set([]); this.reviewLoading.set(false); this.reviewError.set('');
    this.unreadCount.set(0);
    this.reviewNotice.set(''); this.reviewSending.set(false); this.reviewComment.set(''); this.rating.set(5); this.reportReasonsByReview.set({});
    this.reportBusy.set(''); this.reported.set({}); this.reputation.set(null); this.reputationLoading.set(false); this.reputationError.set(''); this.spaceReviews.set(null);
  }
}
function messageFor(error: unknown): string { return error instanceof Error ? error.message : 'Go no pudo completar esta operación.'; }
function isDefinitive(error: unknown): boolean { return error instanceof BookingApiError && error.status !== undefined && error.status >= 400 && error.status < 500; }
function isUncertain(error: unknown): boolean { return !(error instanceof BookingApiError) || error.status === undefined || error.status === 0 || error.status >= 500; }
