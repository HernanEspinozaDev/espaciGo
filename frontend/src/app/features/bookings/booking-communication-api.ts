import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { createClient } from '../../../generated/api/client';
import { Default, M09Local } from '../../../generated/api/sdk.gen';
import type {
  LocalReservationMessage,
  LocalReservationMessagesPageEnvelope,
  LocalReservationReadCursorEnvelope,
  LocalReview,
  LocalReviewEnvelope,
  LocalReviewListEnvelope,
  LocalReviewReportEnvelope,
  LocalSpaceReviewsEnvelope,
} from '../../../generated/api/types.gen';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { BookingApiError } from './booking-api';

type ReviewInput = { rating: number; comment?: string };
type ReportReason = 'insultos_acoso' | 'datos_personales' | 'spam' | 'ajeno_experiencia';
type Attempt = { key: string; payload: string };

/** Bearer-authenticated M09 calls using the generated OpenAPI operations. */
@Injectable({ providedIn: 'root' })
export class BookingCommunicationApi {
  private readonly http = inject(HttpClient);
  private readonly runtime = inject(ApiRuntimeConfig);
  private readonly m09 = inject(M09Local);
  private readonly publicApi = inject(Default);

  async messages(token: string, reservationId: string, before?: number): Promise<LocalReservationMessagesPageEnvelope> {
    return (await this.m09.listOwnLocalReservationMessages({ client: this.client(token), path: { reservationId }, query: { limit: 30, ...(before === undefined ? {} : { before }) }, headers: this.auth(token), throwOnError: true })).data as LocalReservationMessagesPageEnvelope;
  }
  async sendMessage(token: string, reservationId: string, body: string, key: string): Promise<{ safety_notice: string; data: LocalReservationMessage }> {
    return (await this.m09.sendOwnLocalReservationMessage({ client: this.client(token), path: { reservationId }, body: { body }, headers: { ...this.auth(token), 'Idempotency-Key': key }, throwOnError: true })).data as { safety_notice: string; data: LocalReservationMessage };
  }
  async advanceRead(token: string, reservationId: string, through: number): Promise<LocalReservationReadCursorEnvelope> {
    return (await this.m09.advanceOwnLocalReservationMessageReadCursor({ client: this.client(token), path: { reservationId }, body: { through_sequence: through }, headers: this.auth(token), throwOnError: true })).data as LocalReservationReadCursorEnvelope;
  }
  async reservationReviews(token: string, reservationId: string): Promise<LocalReviewListEnvelope> {
    return (await this.publicApi.listReservationReviews({ client: this.client(token), path: { reservationID: reservationId }, headers: this.auth(token), throwOnError: true })).data as LocalReviewListEnvelope;
  }
  async createReview(token: string, reservationId: string, input: ReviewInput, key: string): Promise<LocalReviewEnvelope> {
    return (await this.publicApi.createReservationReview({ client: this.client(token), path: { reservationID: reservationId }, body: input, headers: { ...this.auth(token), 'Idempotency-Key': key }, throwOnError: true })).data as LocalReviewEnvelope;
  }
  async reportReview(token: string, reservationId: string, reviewId: string, reason: ReportReason, key: string): Promise<LocalReviewReportEnvelope> {
    return (await this.publicApi.reportSpaceReview({ client: this.client(token), path: { reservationID: reservationId, reviewID: reviewId }, body: { reason_code: reason }, headers: { ...this.auth(token), 'Idempotency-Key': key }, throwOnError: true })).data as LocalReviewReportEnvelope;
  }
  async spaceReviews(spaceId: string): Promise<LocalSpaceReviewsEnvelope> {
    return (await this.publicApi.listActiveSpaceReviews({ client: this.client(), path: { spaceID: spaceId }, throwOnError: true })).data as LocalSpaceReviewsEnvelope;
  }
  async ownReputation(token: string): Promise<unknown> {
    return (await this.publicApi.getOwnRenterReputation({ client: this.client(token), headers: this.auth(token), throwOnError: true })).data;
  }

  private auth(token: string): { Authorization: string } { return { Authorization: `Bearer ${token}` }; }
  private client(token?: string) {
    const baseUrl = this.runtime.apiBaseUrl();
    if (!baseUrl) throw new BookingApiError('La API todavía no está disponible.');
    const client = createClient({ baseUrl, httpClient: this.http, throwOnError: true, ...(token ? { auth: token } : {}) });
    client.interceptors.error.use((error, response) => {
      const status = response instanceof HttpErrorResponse ? response.status : undefined;
      const body = response instanceof HttpErrorResponse ? response.error : error;
      const message = typeof body === 'object' && body && 'error' in body && typeof body.error === 'object' && body.error && 'message' in body.error
        ? String(body.error.message) : status ? `Go respondió HTTP ${status}.` : 'La conexión con Go no respondió.';
      return new BookingApiError(message, status, body);
    });
    return client;
  }
}

/** In-memory idempotency snapshots; values never include a Bearer token. */
@Injectable({ providedIn: 'root' })
export class BookingCommunicationAttempts {
  private readonly attempts = new Map<string, Attempt>();
  get(scope: string, payload: unknown): Attempt {
    const serialized = JSON.stringify(payload);
    const current = this.attempts.get(scope);
    if (current) {
      if (current.payload !== serialized) throw new Error('Hay una solicitud incierta con otro contenido; reintenta primero el contenido original.');
      return current;
    }
    const attempt = { key: crypto.randomUUID(), payload: serialized };
    this.attempts.set(scope, attempt);
    return attempt;
  }
  find(scope: string): Attempt | null { return this.attempts.get(scope) ?? null; }
  settle(scope: string, key: string): void { if (this.attempts.get(scope)?.key === key) this.attempts.delete(scope); }
}

export type { LocalReview, ReportReason, ReviewInput };
