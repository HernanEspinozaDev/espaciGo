import { UiButton } from '../../shared/ui/button/button';
import { Component, ElementRef, inject, input, output, viewChild } from '@angular/core';
import type { LocalBookingDetail } from '../../core/contracts/models';
import { BookingConversationFacade } from './application/booking-conversation-facade';
@Component({
  imports: [UiButton],
  providers: [BookingConversationFacade],
  selector: 'app-booking-conversation-panel',
  template: `
    <section class="panel conversation" aria-labelledby="conversation-title">
      <header class="heading">
        <div>
          <h2 id="conversation-title">Conversación de la reserva</h2>
          <p>Solo visible para anfitrión y arrendatario. Los mensajes se muestran como texto.</p>
        </div>
        @if (unreadCount() > 0) {
          <span class="badge" aria-label="Mensajes sin leer">{{ unreadCount() }} sin leer</span>
        }
      </header>
      @if (loading()) {
        <p role="status">Consultando mensajes confirmados por Go…</p>
      }
      @if (error()) {
        <p class="error" role="alert">{{ error() }}</p>
      }
      @if (notice()) {
        <p class="success" role="status">{{ notice() }}</p>
      }
      <div #messageList class="messages" aria-live="polite" aria-relevant="additions text">
        @if (!messages().length && !loading()) {
          <p>Aún no hay mensajes en esta conversación.</p>
        }
        @for (message of messages(); track message.id) {
          <article
            class="message"
            [class.own]="message.author_id === accountId()"
            [attr.data-sequence]="message.sequence"
          >
            <p class="meta">
              {{ message.author_id === accountId() ? 'Tú' : 'Participante' }} ·
              {{ when(message.created_at) }} · #{{ message.sequence }}
            </p>
            <p class="body">{{ message.body }}</p>
          </article>
        }
      </div>
      @if (olderCursor() !== null) {
        <button
          appButton="secondary"
          class="button secondary"
          type="button"
          [disabled]="olderLoading() || loading()"
          (click)="loadOlder()"
        >
          {{ olderLoading() ? 'Cargando…' : 'Ver mensajes anteriores' }}
        </button>
      }
      @if (writable()) {
        <label for="message-body">Nuevo mensaje</label>
        <textarea
          id="message-body"
          [value]="body()"
          (input)="body.set($any($event.target).value)"
          rows="3"
          maxlength="2000"
          aria-describedby="message-help"
        ></textarea>
        <p id="message-help" class="hint">Hasta 2.000 caracteres. No incluyas datos sensibles.</p>
        <button
          appButton="primary"
          class="button"
          type="button"
          [disabled]="sending() || (!sendAttempt() && !body().trim())"
          (click)="send()"
        >
          {{
            sending()
              ? 'Enviando…'
              : sendAttempt()
                ? 'Reintentar mensaje original'
                : 'Enviar mensaje'
          }}
        </button>
      } @else {
        <p class="readonly" role="note">
          Esta reserva está en modo de solo lectura. Go conserva el acceso al historial.
        </p>
      }
      <button
        appButton="primary"
        class="text-button"
        type="button"
        [disabled]="loading()"
        (click)="loadRecent()"
      >
        Actualizar conversación
      </button>
    </section>

    <section class="panel reviews" aria-labelledby="review-title">
      <h2 id="review-title">Reseñas y reportes</h2>
      <p>
        Contenido de ensayo. Go define la elegibilidad y conserva las reseñas como registros
        inmutables.
      </p>
      @if (reviewLoading()) {
        <p role="status">Consultando reseñas…</p>
      }
      @if (reviewError()) {
        <p class="error" role="alert">{{ reviewError() }}</p>
      }
      @if (!reviews().length && !reviewLoading()) {
        <p>No hay reseñas visibles para esta reserva.</p>
      }
      @for (review of reviews(); track review.id) {
        <article class="review">
          <h3>
            {{
              review.target_type === 'espacio'
                ? 'Reseña del arrendatario al espacio'
                : 'Reseña del anfitrión al arrendatario'
            }}
            · {{ review.rating }}/5
          </h3>
          <p class="review-copy">{{ review.comment || 'Sin comentario.' }}</p>
          <p class="meta">{{ review.state ?? 'publicada' }} · {{ when(review.created_at) }}</p>
          @if (reported()[review.id]; as reportStatus) {
            <p role="status">{{ reportStatus }}</p>
          }
          @if (canReport(review)) {
            <label [for]="'report-' + review.id">Motivo del reporte</label>
            <select
              [id]="'report-' + review.id"
              [value]="reportReason(review.id)"
              (change)="setReportReason(review.id, $any($event.target).value)"
            >
              @for (reason of reportReasons; track reason.value) {
                <option [value]="reason.value">{{ reason.label }}</option>
              }
            </select>
            <button
              appButton="secondary"
              class="button secondary"
              type="button"
              [disabled]="reportBusy() === review.id"
              (click)="report(review)"
            >
              {{
                reportBusy() === review.id
                  ? 'Enviando…'
                  : reportAttempt(review.id)
                    ? 'Reintentar reporte original'
                    : 'Reportar reseña'
              }}
            </button>
          } @else if (
            review.target_type === 'espacio' && isHost() && review.state === 'reportada'
          ) {
            <p class="readonly">
              Reportada; pendiente de revisión administrativa. Su promedio depende de la decisión de
              Go.
            </p>
          }
        </article>
      }
      @if (reviewNotice()) {
        <p class="success" role="status">{{ reviewNotice() }}</p>
      }
      @if (canReview()) {
        @if (hasOwnReview()) {
          <p class="readonly">Tu reseña ya fue enviada y no se puede editar.</p>
        } @else {
          <form (submit)="$event.preventDefault(); submitReview()">
            <h3>Dejar una reseña</h3>
            <label for="review-rating">Calificación</label>
            <select
              id="review-rating"
              [value]="rating()"
              (change)="rating.set(+$any($event.target).value)"
              [disabled]="reviewSending()"
            >
              <option [value]="5">5 · Excelente</option>
              <option [value]="4">4 · Muy buena</option>
              <option [value]="3">3 · Buena</option>
              <option [value]="2">2 · Regular</option>
              <option [value]="1">1 · Mala</option>
            </select>
            <label for="review-comment">Comentario (opcional)</label>
            <textarea
              id="review-comment"
              [value]="reviewComment()"
              (input)="reviewComment.set($any($event.target).value)"
              rows="3"
              maxlength="2000"
              [disabled]="reviewSending()"
            ></textarea>
            <button appButton="primary" class="button" type="submit" [disabled]="reviewSending()">
              {{
                reviewSending()
                  ? 'Enviando…'
                  : reviewAttempt()
                    ? 'Reintentar reseña original'
                    : 'Enviar reseña'
              }}
            </button>
          </form>
        }
      } @else if (reservation().state === 'en_disputa') {
        <p class="readonly">La reserva está en disputa. No se admiten reseñas nuevas.</p>
      } @else if (!REVIEW_STATES.has(reservation().state)) {
        <p class="hint">Go solo admite reseñas después de finalizar o cerrar la reserva.</p>
      }
    </section>

    @if (isRenter()) {
      <section class="panel reputation" aria-labelledby="reputation-title">
        <h2 id="reputation-title">Tu reputación</h2>
        @if (reputationLoading()) {
          <p role="status">Consultando calificaciones recibidas…</p>
        }
        @if (reputationError()) {
          <p class="error" role="alert">{{ reputationError() }}</p>
        }
        @if (reputation(); as value) {
          <p>
            Promedio recibido: {{ value.average ?? 0 }}/5 · {{ value.count ?? 0 }} calificación(es).
          </p>
          @if (!value.count) {
            <p>Aún no tienes calificaciones recibidas.</p>
          }
        }
      </section>
    }
    @if (spaceReviews(); as value) {
      <section class="panel public-reviews" aria-labelledby="public-review-title">
        <h2 id="public-review-title">Reseñas visibles del espacio</h2>
        <p>Promedio {{ value.average }}/5 · {{ value.count }} reseña(s).</p>
        @for (review of value.items; track review.id) {
          <article class="review">
            <h3>{{ review.rating }}/5</h3>
            <p class="review-copy">{{ review.comment || 'Sin comentario.' }}</p>
          </article>
        }
        @if (!value.count) {
          <p>El espacio no tiene reseñas públicas visibles.</p>
        }
      </section>
    }
  `,
  styles: [
    `
      :host {
        display: block;
      }
      .panel {
        border: 1px solid var(--color-border);
        border-radius: 1rem;
        background: var(--color-surface);
        padding: clamp(1rem, 3vw, 1.5rem);
        margin-bottom: 1rem;
        min-width: 0;
      }
      .panel h2 {
        margin-top: 0;
      }
      .panel p,
      .panel li {
        line-height: 1.55;
        color: var(--color-text-muted);
        overflow-wrap: anywhere;
      }
      .heading {
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        gap: 1rem;
      }
      .heading p {
        margin-top: 0.3rem;
      }
      .badge {
        flex: none;
        border-radius: 99px;
        background: var(--brand-primary-soft);
        color: var(--brand-primary-hover);
        padding: 0.4rem 0.7rem;
        font-weight: 800;
      }
      .messages {
        display: grid;
        gap: 0.65rem;
        max-height: 32rem;
        overflow: auto;
        padding: 0.2rem;
      }
      .message,
      .review {
        border: 1px solid var(--brand-primary-soft);
        border-radius: 0.8rem;
        padding: 0.8rem;
        min-width: 0;
      }
      .message {
        max-width: min(92%, 42rem);
        justify-self: start;
        background: var(--color-surface-subtle);
      }
      .message.own {
        justify-self: end;
        background: var(--brand-primary-soft);
      }
      .message p,
      .review p {
        margin: 0.35rem 0;
      }
      .meta,
      .hint {
        font-size: 0.88rem;
        color: var(--color-border-control);
      }
      .body,
      .review-copy {
        white-space: pre-wrap;
        overflow-wrap: anywhere;
      }
      .button,
      .text-button {
        min-height: 2.8rem;
        border: 0;
        border-radius: 999px;
        padding: 0.65rem 1rem;
        background: var(--brand-primary);
        color: var(--color-canvas);
        font: inherit;
        font-weight: 800;
        cursor: pointer;
        margin: 0.55rem 0.45rem 0.2rem 0;
      }
      .button.secondary {
        background: var(--brand-primary-soft);
        color: var(--brand-primary-hover);
      }
      .button:disabled,
      .text-button:disabled {
        opacity: 0.55;
        cursor: wait;
      }
      .text-button {
        background: transparent;
        color: var(--brand-primary-hover);
        text-decoration: underline;
        padding-inline: 0.25rem;
      }
      label {
        display: grid;
        gap: 0.4rem;
        margin: 0.65rem 0 0.35rem;
        color: var(--color-text-muted);
        font-weight: 700;
      }
      textarea,
      select {
        box-sizing: border-box;
        width: 100%;
        max-width: 100%;
        border: 1px solid var(--color-border);
        border-radius: 0.55rem;
        padding: 0.65rem;
        background: var(--color-surface);
        font: inherit;
      }
      textarea {
        resize: vertical;
      }
      .readonly {
        padding: 0.75rem;
        border-radius: 0.6rem;
        background: var(--color-surface-subtle);
      }
      .error {
        padding: 0.8rem;
        background: var(--color-error-soft);
        color: var(--brand-primary-hover);
      }
      .success {
        padding: 0.8rem;
        background: var(--color-surface-subtle);
        color: var(--color-success);
      }
      .reviews form {
        border-top: 1px solid var(--brand-primary-soft);
        margin-top: 1rem;
        padding-top: 1rem;
      }
      .review {
        margin: 0.7rem 0;
      }
      .review h3 {
        margin: 0.1rem 0 0.45rem;
        font-size: 1rem;
      }
      @media (max-width: 520px) {
        .heading {
          display: block;
        }
        .badge {
          display: inline-block;
          margin-bottom: 0.5rem;
        }
        .message {
          max-width: 100%;
        }
      }
    `,
  ],
})
export class BookingConversationPanel {
  readonly reservation = input.required<LocalBookingDetail>();
  readonly stateChanged = output<void>();
  private readonly facade = inject(BookingConversationFacade);
  private readonly messageList = viewChild<ElementRef<HTMLElement>>('messageList');
  constructor() {
    this.facade.connectMessageList(this.messageList);
    this.facade.connectReservation(this.reservation, () => this.stateChanged.emit());
  }
  get messages() {
    return this.facade.messages;
  }
  get olderCursor() {
    return this.facade.olderCursor;
  }
  get body() {
    return this.facade.body;
  }
  get loading() {
    return this.facade.loading;
  }
  get olderLoading() {
    return this.facade.olderLoading;
  }
  get sending() {
    return this.facade.sending;
  }
  get error() {
    return this.facade.error;
  }
  get notice() {
    return this.facade.notice;
  }
  get readThrough() {
    return this.facade.readThrough;
  }
  get unreadCount() {
    return this.facade.unreadCount;
  }
  get reviews() {
    return this.facade.reviews;
  }
  get reviewLoading() {
    return this.facade.reviewLoading;
  }
  get reviewError() {
    return this.facade.reviewError;
  }
  get reviewNotice() {
    return this.facade.reviewNotice;
  }
  get reviewSending() {
    return this.facade.reviewSending;
  }
  get reviewComment() {
    return this.facade.reviewComment;
  }
  get rating() {
    return this.facade.rating;
  }
  get reportReasons() {
    return this.facade.reportReasons;
  }
  get reportReasonsByReview() {
    return this.facade.reportReasonsByReview;
  }
  get reportBusy() {
    return this.facade.reportBusy;
  }
  get reported() {
    return this.facade.reported;
  }
  get reputation() {
    return this.facade.reputation;
  }
  get reputationLoading() {
    return this.facade.reputationLoading;
  }
  get reputationError() {
    return this.facade.reputationError;
  }
  get spaceReviews() {
    return this.facade.spaceReviews;
  }
  get accountId() {
    return this.facade.accountId;
  }
  get writable() {
    return this.facade.writable;
  }
  get REVIEW_STATES() {
    return this.facade.REVIEW_STATES;
  }
  readonly loadRecent = this.facade.loadRecent.bind(this.facade);
  readonly loadOlder = this.facade.loadOlder.bind(this.facade);
  readonly send = this.facade.send.bind(this.facade);
  readonly loadReviews = this.facade.loadReviews.bind(this.facade);
  readonly submitReview = this.facade.submitReview.bind(this.facade);
  readonly report = this.facade.report.bind(this.facade);
  readonly loadReputation = this.facade.loadReputation.bind(this.facade);
  readonly loadSpaceReviews = this.facade.loadSpaceReviews.bind(this.facade);
  readonly canReview = this.facade.canReview.bind(this.facade);
  readonly hasOwnReview = this.facade.hasOwnReview.bind(this.facade);
  readonly canReport = this.facade.canReport.bind(this.facade);
  readonly reportReason = this.facade.reportReason.bind(this.facade);
  readonly setReportReason = this.facade.setReportReason.bind(this.facade);
  readonly sendAttempt = this.facade.sendAttempt.bind(this.facade);
  readonly reviewAttempt = this.facade.reviewAttempt.bind(this.facade);
  readonly reportAttempt = this.facade.reportAttempt.bind(this.facade);
  readonly isRenter = this.facade.isRenter.bind(this.facade);
  readonly isHost = this.facade.isHost.bind(this.facade);
  readonly isParticipant = this.facade.isParticipant.bind(this.facade);
  readonly when = this.facade.when.bind(this.facade);
}
