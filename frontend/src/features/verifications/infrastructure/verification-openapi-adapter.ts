import "client-only";

import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type { VerificationPort } from "../application/verification.port";
import type {
  SyntheticEvidence,
  VerificationCase,
  VerificationCorrectionCode,
  VerificationEligibilityItem,
  VerificationHistoryEntry,
  VerificationType,
} from "../domain/verification";

export class OpenApiVerificationAdapter implements VerificationPort {
  async listVerifications(signal?: AbortSignal): Promise<VerificationCase[]> {
    const result = await apiClient.GET("/verifications", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.items ?? [];
  }

  async getVerification(
    id: string,
    signal?: AbortSignal,
  ): Promise<VerificationCase | null> {
    const result = await apiClient.GET("/verifications/{id}", {
      params: {
        path: { id },
      },
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data ?? null;
  }

  async startVerification(
    idempotencyKey: string,
    type: VerificationType,
    signal?: AbortSignal,
  ): Promise<VerificationCase> {
    const result = await apiClient.POST("/verifications", {
      params: {
        header: {
          "Idempotency-Key": idempotencyKey,
        },
      },
      body: { type },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return result.data;
  }

  async retryVerification(
    id: string,
    idempotencyKey: string,
    correctionCode: VerificationCorrectionCode,
    signal?: AbortSignal,
  ): Promise<VerificationCase> {
    const result = await apiClient.POST("/verifications/{id}/retry", {
      params: {
        path: { id },
        header: {
          "Idempotency-Key": idempotencyKey,
        },
      },
      body: {
        correction_code: correctionCode as
          | "fixture_vigente_actualizado"
          | "antecedentes_fixture_actualizados"
          | "inicio_actividades_fixture_actualizadas",
      },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return result.data;
  }

  async listHistory(
    id: string,
    signal?: AbortSignal,
  ): Promise<VerificationHistoryEntry[]> {
    const result = await apiClient.GET("/verifications/{id}/history", {
      params: {
        path: { id },
      },
      signal,
    });
    if (result.response.status === 404) return [];
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.items ?? [];
  }

  async listEvidence(
    id: string,
    signal?: AbortSignal,
  ): Promise<SyntheticEvidence[]> {
    const result = await apiClient.GET("/verifications/{id}/evidence", {
      params: {
        path: { id },
      },
      signal,
    });
    if (result.response.status === 404) return [];
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.items ?? [];
  }

  async uploadEvidence(
    id: string,
    signal?: AbortSignal,
  ): Promise<SyntheticEvidence> {
    const result = await apiClient.POST("/verifications/{id}/evidence", {
      params: {
        path: { id },
      },
      body: { fixture_code: "synthetic-png-v1" },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return result.data;
  }

  async deleteEvidence(
    id: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.DELETE(
      "/verifications/{id}/evidence/{evidenceId}",
      {
        params: {
          path: { id, evidenceId },
        },
        signal,
      },
    );
    if (result.response.status === 404) return;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async getEvidenceContent(
    id: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<Blob | null> {
    const result = await apiClient.GET(
      "/verifications/{id}/evidence/{evidenceId}",
      {
        params: {
          path: { id, evidenceId },
        },
        parseAs: "blob",
        credentials: "same-origin",
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data as Blob) ?? (await result.response.blob());
  }

  async getEligibility(
    signal?: AbortSignal,
  ): Promise<VerificationEligibilityItem[]> {
    const result = await apiClient.GET("/verifications/eligibility", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.items ?? [];
  }
}
