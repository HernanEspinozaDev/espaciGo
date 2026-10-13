import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { createClient } from '../../../../generated/api/client';
import { M07Local, M08Local } from '../../../../generated/api/sdk.gen';
import { ApiRuntimeConfig } from '../../../core/api-runtime-config';
import type {
  LocalRentalOperation,
  LocalRentalOperationInput,
  LocalRentalOperationListEnvelope,
  LocalSyntheticContract,
} from '../../../core/contracts/models';
import { BookingLifecyclePort, Envelope } from '../domain/ports/booking-lifecycle-port';
import { BookingApiError } from './booking-api';

/** Bearer-authenticated access to the generated FE-08 OpenAPI operations. */
@Injectable({ providedIn: 'root' })
export class BookingLifecycleApi extends BookingLifecyclePort {
  private readonly http = inject(HttpClient);
  private readonly runtime = inject(ApiRuntimeConfig);
  private readonly m07 = inject(M07Local);
  private readonly m08 = inject(M08Local);

  async createOrGetContract(
    token: string,
    reservationId: string,
  ): Promise<Envelope<LocalSyntheticContract>> {
    return (
      await this.m07.createOrGetSyntheticReservationContract({
        client: this.client(token),
        path: { reservationId },
        headers: this.authHeaders(token),
        throwOnError: true,
      })
    ).data as Envelope<LocalSyntheticContract>;
  }
  async getContract(token: string, contractId: string): Promise<Envelope<LocalSyntheticContract>> {
    return (
      await this.m07.getOwnSyntheticContract({
        client: this.client(token),
        path: { contractId },
        headers: this.authHeaders(token),
        throwOnError: true,
      })
    ).data as Envelope<LocalSyntheticContract>;
  }
  async signContract(token: string, contractId: string): Promise<Envelope<LocalSyntheticContract>> {
    return (
      await this.m07.signSyntheticContractAsParticipant({
        client: this.client(token),
        path: { contractId },
        headers: this.authHeaders(token),
        throwOnError: true,
      })
    ).data as Envelope<LocalSyntheticContract>;
  }
  async rejectContract(
    token: string,
    contractId: string,
  ): Promise<Envelope<LocalSyntheticContract>> {
    return (
      await this.m07.rejectSyntheticContractVersion({
        client: this.client(token),
        path: { contractId },
        headers: this.authHeaders(token),
        throwOnError: true,
      })
    ).data as Envelope<LocalSyntheticContract>;
  }
  async contractPdf(token: string, contractId: string): Promise<Blob> {
    const options = {
      client: this.client(token),
      path: { contractId },
      headers: this.authHeaders(token),
      responseType: 'blob',
      throwOnError: true,
    } as unknown as Parameters<M07Local['downloadSignedSyntheticContract']>[0];
    return (await this.m07.downloadSignedSyntheticContract(options)).data as Blob;
  }
  async operations(
    token: string,
    reservationId: string,
  ): Promise<LocalRentalOperationListEnvelope> {
    return (
      await this.m08.listSyntheticRentalOperations({
        client: this.client(token),
        path: { reservationID: reservationId },
        headers: this.authHeaders(token),
        throwOnError: true,
      })
    ).data as LocalRentalOperationListEnvelope;
  }
  async record(
    token: string,
    reservationId: string,
    action: 'check-in' | 'check-out' | 'reception',
    body: LocalRentalOperationInput,
    key: string,
  ): Promise<Envelope<LocalRentalOperation>> {
    const common = {
      client: this.client(token),
      path: { reservationID: reservationId },
      body,
      headers: { ...this.authHeaders(token), 'Idempotency-Key': key },
      throwOnError: true as const,
    };
    if (action === 'check-in')
      return (await this.m08.recordSyntheticCheckIn(common)).data as Envelope<LocalRentalOperation>;
    if (action === 'check-out')
      return (await this.m08.recordSyntheticCheckOut(common))
        .data as Envelope<LocalRentalOperation>;
    return (await this.m08.recordSyntheticHostReception(common))
      .data as Envelope<LocalRentalOperation>;
  }
  async evidence(token: string, reservationId: string, evidenceId: string): Promise<Blob> {
    const options = {
      client: this.client(token),
      path: { reservationID: reservationId, evidenceID: evidenceId },
      headers: this.authHeaders(token),
      responseType: 'blob',
      throwOnError: true,
    } as unknown as Parameters<M08Local['downloadSyntheticRentalOperationEvidence']>[0];
    return (await this.m08.downloadSyntheticRentalOperationEvidence(options)).data as Blob;
  }

  private authHeaders(token: string) {
    return { Authorization: `Bearer ${token}` };
  }

  private client(token: string) {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new BookingApiError('La API todavía no está disponible.');
    const client = createClient({
      baseUrl: base,
      auth: token,
      httpClient: this.http,
      throwOnError: true,
    });
    client.interceptors.error.use((error, response) => {
      const status = response instanceof HttpErrorResponse ? response.status : undefined;
      const body = response instanceof HttpErrorResponse ? response.error : error;
      const message =
        typeof body === 'object' &&
        body &&
        'error' in body &&
        typeof body.error === 'object' &&
        body.error &&
        'message' in body.error
          ? String(body.error.message)
          : status
            ? `Go respondió HTTP ${status}.`
            : 'La conexión con Go no respondió.';
      return new BookingApiError(message, status, body);
    });
    return client;
  }
}

/** Idempotency is kept only in memory and is scoped to owner, reservation and operation. */

export { type Attempt, type Envelope } from '../domain/ports/booking-lifecycle-port';
