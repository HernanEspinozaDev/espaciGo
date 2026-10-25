import "client-only";

import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type { AccessToken } from "@/features/auth/application/auth.port";
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
  async listVerifications(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<VerificationCase[]> {
    const result = await apiClient.GET("/verifications", {
      headers: { Authorization: `Bearer ${token}` },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.items ?? [];
  }

  async getVerification(
    token: AccessToken,
    id: string,
    signal?: AbortSignal,
  ): Promise<VerificationCase | null> {
    const result = await apiClient.GET("/verifications/{id}", {
      params: {
        path: { id },
      },
      headers: { Authorization: `Bearer ${token}` },
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data ?? null;
  }

  async startVerification(
    token: AccessToken,
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
      headers: {
        Authorization: `Bearer ${token}`,
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
    token: AccessToken,
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
      headers: {
        Authorization: `Bearer ${token}`,
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
    token: AccessToken,
    id: string,
    signal?: AbortSignal,
  ): Promise<VerificationHistoryEntry[]> {
    const result = await apiClient.GET("/verifications/{id}/history", {
      params: {
        path: { id },
      },
      headers: { Authorization: `Bearer ${token}` },
      signal,
    });
    if (result.response.status === 404) return [];
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.items ?? [];
  }

  async listEvidence(
    token: AccessToken,
    id: string,
    signal?: AbortSignal,
  ): Promise<SyntheticEvidence[]> {
    const result = await apiClient.GET("/verifications/{id}/evidence", {
      params: {
        path: { id },
      },
      headers: { Authorization: `Bearer ${token}` },
      signal,
    });
    if (result.response.status === 404) return [];
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.items ?? [];
  }

  async uploadEvidence(
    token: AccessToken,
    id: string,
    signal?: AbortSignal,
  ): Promise<SyntheticEvidence> {
    const result = await apiClient.POST("/verifications/{id}/evidence", {
      params: {
        path: { id },
      },
      headers: { Authorization: `Bearer ${token}` },
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
    token: AccessToken,
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
        headers: { Authorization: ["Bearer", token].join(" ") },
        signal,
      },
    );
    if (result.response.status === 404) return;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async getEvidenceContent(
    token: AccessToken,
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
        headers: { Authorization: ["Bearer", token].join(" ") },
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

  async getEligibility(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<VerificationEligibilityItem[]> {
    const result = await apiClient.GET("/verifications/eligibility", {
      headers: { Authorization: `Bearer ${token}` },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.items ?? [];
  }
}
