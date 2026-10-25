import "client-only";

import type { AccessToken } from "@/features/auth/application/auth.port";
import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type { BookingsPort } from "../application/bookings.port";
import type {
  BookingDetail,
  BookingQuote,
  BookingReservation,
  BusinessClock,
  CancellationPreview,
  DamageClaim,
  Guarantee,
  RentalOperation,
  ReservationMessage,
  ReservationReview,
  ReviewReport,
  Settlement,
  SettlementReceipt,
  SimulatedPaymentOutcome,
  SyntheticContract,
  TrialDispute,
  TrialDisputeTransition,
} from "../domain/types";

export class OpenApiBookingsAdapter implements BookingsPort {
  async quote(
    token: AccessToken,
    input: { space_id: string; start_at: string; end_at: string },
    signal?: AbortSignal,
  ): Promise<BookingQuote> {
    const result = await apiClient.POST("/local/booking-trial/quotes", {
      headers: { Authorization: `Bearer ${token}` },
      body: input,
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as BookingQuote;
  }

  async getFixture(token: AccessToken, signal?: AbortSignal): Promise<unknown> {
    const result = await apiClient.GET("/local/booking-trial/fixture", {
      headers: { Authorization: `Bearer ${token}` },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data;
  }

  async listReservations(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<BookingReservation[]> {
    const result = await apiClient.GET("/local/booking-trial/reservations", {
      headers: { Authorization: `Bearer ${token}` },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data?.items as unknown as BookingReservation[]) ?? [];
  }

  async getReservation(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<BookingDetail | null> {
    const result = await apiClient.GET(
      "/local/booking-trial/reservations/{reservationId}",
      {
        params: { path: { reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data as unknown as BookingDetail) ?? null;
  }

  async createReservation(
    token: AccessToken,
    idempotencyKey: string,
    quoteId: string,
    signal?: AbortSignal,
  ): Promise<BookingReservation> {
    const result = await apiClient.POST("/local/booking-trial/reservations", {
      params: { header: { "Idempotency-Key": idempotencyKey } },
      headers: { Authorization: `Bearer ${token}` },
      body: { quote_id: quoteId },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as BookingReservation;
  }

  async decideReservation(
    token: AccessToken,
    reservationId: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
    signal?: AbortSignal,
  ): Promise<BookingReservation> {
    const body: { decision: "aprobar" | "rechazar"; reason?: string } = {
      decision,
    };
    if (reason) {
      body.reason = reason;
    }
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationId}/decision",
      {
        params: { path: { reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        body,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as BookingReservation;
  }

  async previewCancellation(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<CancellationPreview> {
    const result = await apiClient.GET(
      "/local/booking-trial/reservations/{reservationId}/cancellation-preview",
      {
        params: { path: { reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    const d = result.data.data;
    return {
      reservation_id: d.reservation_id,
      policy_version: d.policy_version,
      eligible: d.eligible,
      refund_ratio: d.eligible ? 1 : 0,
      refund_amount_clp: d.amount_clp,
      penalty_amount_clp: 0,
      reason: d.reason_code ?? d.refund_label,
    };
  }

  async cancelReservation(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    reason?: string,
    signal?: AbortSignal,
  ): Promise<BookingReservation> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationId}/cancel",
      {
        params: {
          path: { reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: { reason },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data?.reservation) throw new ApiProtocolError();
    return result.data.data.reservation as unknown as BookingReservation;
  }

  async simulatePayment(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    outcome: SimulatedPaymentOutcome,
    signal?: AbortSignal,
  ): Promise<BookingReservation> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationId}/payment",
      {
        params: {
          path: { reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: { outcome },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as BookingReservation;
  }

  async simulateRefund(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    outcome: "exito" | "fallo" | "sin_respuesta",
    signal?: AbortSignal,
  ): Promise<BookingReservation> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationId}/refund",
      {
        params: {
          path: { reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: { outcome },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    const updated = await this.getReservation(token, reservationId, signal);
    if (updated) return updated;
    throw new ApiProtocolError();
  }

  async createOrGetContract(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationId}/contract",
      {
        params: { path: { reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as SyntheticContract;
  }

  async getContract(
    token: AccessToken,
    contractId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract | null> {
    const result = await apiClient.GET(
      "/local/booking-trial/contracts/{contractId}",
      {
        params: { path: { contractId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data as unknown as SyntheticContract) ?? null;
  }

  async signContract(
    token: AccessToken,
    contractId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract> {
    const result = await apiClient.POST(
      "/local/booking-trial/contracts/{contractId}/sign",
      {
        params: { path: { contractId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as SyntheticContract;
  }

  async rejectContract(
    token: AccessToken,
    contractId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract> {
    const result = await apiClient.POST(
      "/local/booking-trial/contracts/{contractId}/reject",
      {
        params: { path: { contractId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as SyntheticContract;
  }

  async downloadContractPdf(
    token: AccessToken,
    contractId: string,
    signal?: AbortSignal,
  ): Promise<Blob | null> {
    const result = await apiClient.GET(
      "/local/booking-trial/contracts/{contractId}/document",
      {
        params: { path: { contractId } },
        headers: { Authorization: `Bearer ${token}` },
        parseAs: "blob",
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data as Blob) ?? (await result.response.blob());
  }

  async recordCheckIn(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    input: {
      occurred_at?: string;
      time_zone?: string;
      comments?: string;
      fixture_evidence_code?:
        "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
    },
    signal?: AbortSignal,
  ): Promise<RentalOperation> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationID}/check-in",
      {
        params: {
          path: { reservationID: reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: input,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as RentalOperation;
  }

  async recordCheckOut(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    input: {
      occurred_at?: string;
      time_zone?: string;
      comments?: string;
      fixture_evidence_code?:
        "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
    },
    signal?: AbortSignal,
  ): Promise<RentalOperation> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationID}/check-out",
      {
        params: {
          path: { reservationID: reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: input,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as RentalOperation;
  }

  async recordReception(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    input: {
      result?:
        "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";
      observations?: string;
    },
    signal?: AbortSignal,
  ): Promise<RentalOperation> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationID}/reception",
      {
        params: {
          path: { reservationID: reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: input,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as RentalOperation;
  }

  async listOperations(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<RentalOperation[]> {
    const result = await apiClient.GET(
      "/local/booking-trial/reservations/{reservationID}/operations",
      {
        params: { path: { reservationID: reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data?.items as unknown as RentalOperation[]) ?? [];
  }

  async downloadEvidence(
    token: AccessToken,
    reservationId: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<Blob | null> {
    const result = await apiClient.GET(
      "/local/booking-trial/reservations/{reservationID}/evidence/{evidenceID}",
      {
        params: {
          path: { reservationID: reservationId, evidenceID: evidenceId },
        },
        headers: { Authorization: `Bearer ${token}` },
        parseAs: "blob",
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data as Blob) ?? (await result.response.blob());
  }

  async listMessages(
    token: AccessToken,
    reservationId: string,
    query?: { before?: number | string; limit?: number },
    signal?: AbortSignal,
  ): Promise<ReservationMessage[]> {
    const result = await apiClient.GET(
      "/local/booking-trial/reservations/{reservationId}/messages",
      {
        params: {
          path: { reservationId },
          query,
        },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data?.items as unknown as ReservationMessage[]) ?? [];
  }

  async sendMessage(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    body: string,
    signal?: AbortSignal,
  ): Promise<ReservationMessage> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationId}/messages",
      {
        params: {
          path: { reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: { body },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as ReservationMessage;
  }

  async advanceReadCursor(
    token: AccessToken,
    reservationId: string,
    throughSequence: number | string,
    signal?: AbortSignal,
  ): Promise<{ sequence: number }> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationId}/messages/read",
      {
        params: { path: { reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        body: { through_sequence: throughSequence },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return { sequence: Number(result.data.data.read_through_sequence) };
  }

  async listReviews(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<ReservationReview[]> {
    const result = await apiClient.GET(
      "/local/booking-trial/reservations/{reservationID}/reviews",
      {
        params: { path: { reservationID: reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data?.items as unknown as ReservationReview[]) ?? [];
  }

  async createReview(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    input: {
      rating: number;
      comment?: string;
      target_type?: "espacio" | "arrendatario";
    },
    signal?: AbortSignal,
  ): Promise<ReservationReview> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationID}/reviews",
      {
        params: {
          path: { reservationID: reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: input,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as ReservationReview;
  }

  async reportReview(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    reviewId: string,
    input: {
      reason_code:
        "insultos_acoso" | "datos_personales" | "spam" | "ajeno_experiencia";
      comment?: string;
    },
    signal?: AbortSignal,
  ): Promise<ReviewReport> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationID}/reviews/{reviewID}/report",
      {
        params: {
          path: { reservationID: reservationId, reviewID: reviewId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: input,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as ReviewReport;
  }

  async listDisputes(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<TrialDispute[]> {
    const result = await apiClient.GET(
      "/local/booking-trial/reservations/{reservation_id}/disputes",
      {
        params: { path: { reservation_id: reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.items as unknown as TrialDispute[]) ?? [];
  }

  async openDispute(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    _reasonCode: string,
    signal?: AbortSignal,
  ): Promise<TrialDispute> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservation_id}/disputes",
      {
        params: {
          path: { reservation_id: reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: { reason_code: "ensayo_privacidad" },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return result.data as unknown as TrialDispute;
  }

  async getDisputeHistory(
    token: AccessToken,
    disputeId: string,
    signal?: AbortSignal,
  ): Promise<TrialDisputeTransition[]> {
    const result = await apiClient.GET(
      "/local/booking-trial/disputes/{dispute_id}/history",
      {
        params: { path: { dispute_id: disputeId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.items as unknown as TrialDisputeTransition[]) ?? [];
  }

  async getDamageClaim(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<DamageClaim | null> {
    const result = await apiClient.GET(
      "/local/booking-trial/reservations/{reservationID}/damage-claim",
      {
        params: { path: { reservationID: reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data as unknown as DamageClaim) ?? null;
  }

  async openDamageClaim(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    description: string,
    signal?: AbortSignal,
  ): Promise<DamageClaim> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationID}/damage-claim",
      {
        params: {
          path: { reservationID: reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: { description },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as DamageClaim;
  }

  async submitDamageClaimDefense(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    description: string,
    signal?: AbortSignal,
  ): Promise<DamageClaim> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationID}/damage-claim/defense",
      {
        params: {
          path: { reservationID: reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: { description },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    const updated = await this.getDamageClaim(token, reservationId, signal);
    if (updated) return updated;
    throw new ApiProtocolError();
  }

  async createDamageClaimDefenseEvidence(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<RentalOperation> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationID}/damage-claim/defense/evidence",
      {
        params: {
          path: { reservationID: reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: {},
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as RentalOperation;
  }

  async getGuarantee(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<Guarantee | null> {
    const result = await apiClient.GET(
      "/local/booking-trial/reservations/{reservationId}/guarantee",
      {
        params: { path: { reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data as unknown as Guarantee) ?? null;
  }

  async executeGuaranteeOperation(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    input: {
      kind: "autorizacion" | "captura" | "liberacion";
      amount_clp: number;
      outcome?: SimulatedPaymentOutcome;
    },
    signal?: AbortSignal,
  ): Promise<Guarantee> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationId}/guarantee",
      {
        params: {
          path: { reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: {
          kind: input.kind,
          amount_clp: input.amount_clp,
          outcome: input.outcome ?? "exito",
        },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as Guarantee;
  }

  async getSettlement(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<Settlement | null> {
    const result = await apiClient.GET(
      "/local/booking-trial/reservations/{reservationId}/settlement",
      {
        params: { path: { reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data as unknown as Settlement) ?? null;
  }

  async requestSettlement(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    outcome?: SimulatedPaymentOutcome,
    signal?: AbortSignal,
  ): Promise<Settlement> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationId}/settlement",
      {
        params: {
          path: { reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        body: { outcome: outcome ?? "exito" },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as Settlement;
  }

  async reconcileSettlement(
    token: AccessToken,
    reservationId: string,
    outcome: "exito" | "rechazo",
    signal?: AbortSignal,
  ): Promise<Settlement> {
    const result = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationId}/settlement-reconcile",
      {
        params: { path: { reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        body: { outcome },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as Settlement;
  }

  async getSettlementReceipt(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<SettlementReceipt | null> {
    const result = await apiClient.GET(
      "/local/booking-trial/reservations/{reservationId}/receipt",
      {
        params: { path: { reservationId } },
        headers: { Authorization: `Bearer ${token}` },
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) return null;
    const r = result.data.data;
    return {
      receipt_id: r.receipt_id,
      reservation_id: r.reservation_id,
      policy_version: r.policy_version,
      currency: r.currency,
      gross_rent_clp: r.gross_rent_clp,
      confirmed_refunds_clp: r.confirmed_refunds_clp,
      commission_clp: r.commission_clp,
      transfer_clp: r.transfer_clp,
      confirmed_at: r.confirmed_at,
      banner: r.banner,
    };
  }

  async getBusinessClock(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<BusinessClock> {
    const result = await apiClient.GET("/local/booking-trial/business-clock", {
      headers: { Authorization: `Bearer ${token}` },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return {
      current_time: result.data?.data?.now ?? new Date().toISOString(),
      time_zone: "America/Santiago",
    };
  }
}
