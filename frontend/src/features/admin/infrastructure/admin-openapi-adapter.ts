import "client-only";

import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type { AdminPort } from "../application/admin.port";
import type {
  AdminDamageClaim,
  AdminDamageClaimResolutionInput,
  AdminDisputesReport,
  AdminFinanceReport,
  AdminLocalAccount,
  AdminLocalDispute,
  AdminLocalNotice,
  AdminPublishedSpace,
  AdminReservationDetail,
  AdminReservationSummary,
  AdminReservationsReport,
  AdminReviewReport,
  AdminVerificationCase,
  AuditEvent,
  AuditEventsQuery,
  GlobalAdminAccount,
  GlobalAdminAccountQuery,
  GlobalAdminKPIs,
  GlobalAdminMe,
  GlobalAdminOverview,
  GlobalAdminPublication,
  GlobalAdminPublicationQuery,
} from "../domain/types";

export class OpenApiAdminAdapter implements AdminPort {
  // -----------------------------------------------------
  // Global Admin
  // -----------------------------------------------------

  async getGlobalAdminMe(signal?: AbortSignal): Promise<GlobalAdminMe> {
    const result = await apiClient.GET("/admin/global/me", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as GlobalAdminMe;
  }

  async getGlobalAdminOverview(
    signal?: AbortSignal,
  ): Promise<GlobalAdminOverview> {
    const result = await apiClient.GET("/admin/global/overview", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as GlobalAdminOverview;
  }

  async getGlobalAdminKPIs(
    query: { from: string; until: string; time_zone: string },
    signal?: AbortSignal,
  ): Promise<GlobalAdminKPIs> {
    const result = await apiClient.GET("/admin/global/kpis", {
      params: { query },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return result.data as unknown as GlobalAdminKPIs;
  }

  async listGlobalAuditEvents(
    query: AuditEventsQuery,
    signal?: AbortSignal,
  ): Promise<AuditEvent[]> {
    const result = await apiClient.GET("/admin/global/audit-events", {
      params: {
        query: query as never,
      },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    const payload = result.data as Record<string, unknown> | undefined;
    const dataObj = payload?.data as Record<string, unknown> | undefined;
    return (
      (dataObj?.items as unknown as AuditEvent[]) ??
      (payload?.items as unknown as AuditEvent[]) ??
      []
    );
  }

  async exportGlobalAuditEvents(
    query: AuditEventsQuery,
    signal?: AbortSignal,
  ): Promise<Blob> {
    const result = await apiClient.GET("/admin/global/audit-events/export", {
      params: {
        query: {
          ...query,
          snapshot_cutoff: query.snapshot_cutoff ?? new Date().toISOString(),
        } as never,
      },
      parseAs: "blob",
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data as Blob) ?? (await result.response.blob());
  }

  async listGlobalAccounts(
    query?: GlobalAdminAccountQuery,
    signal?: AbortSignal,
  ): Promise<{ items: GlobalAdminAccount[]; next_cursor?: string }> {
    const result = await apiClient.GET("/admin/global/accounts", {
      params: { query: query as never },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return {
      items:
        (result.data?.data?.items as unknown as GlobalAdminAccount[]) ?? [],
      next_cursor: result.data?.data?.next_cursor,
    };
  }

  async getGlobalAccount(
    accountId: string,
    signal?: AbortSignal,
  ): Promise<GlobalAdminAccount | null> {
    const result = await apiClient.GET("/admin/global/accounts/{accountID}", {
      params: { path: { accountID: accountId } },
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data as unknown as GlobalAdminAccount) ?? null;
  }

  async updateGlobalAccountProfile(
    accountId: string,
    input: {
      display_name: string;
      phone?: string | null;
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.PATCH(
      "/admin/global/accounts/{accountID}/profile",
      {
        params: { path: { accountID: accountId } },
        body: input,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async setGlobalAccountRole(
    accountId: string,
    role: "arrendatario" | "arrendador",
    granted: boolean,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.PUT(
      "/admin/global/accounts/{accountID}/roles/{role}",
      {
        params: { path: { accountID: accountId, role } },
        body: { granted, reason },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async setGlobalLegacyAdminRole(
    accountId: string,
    granted: boolean,
    reason:
      | "aprovisionamiento_sintetico"
      | "revision_administrativa"
      | "retirada_acceso",
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.PUT(
      "/admin/global/accounts/{accountID}/legacy-administrator",
      {
        params: { path: { accountID: accountId } },
        body: { granted, reason },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async revokeGlobalAccountSessions(
    accountId: string,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.POST(
      "/admin/global/accounts/{accountID}/sessions/revoke",
      {
        params: { path: { accountID: accountId } },
        body: { reason },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async setGlobalAccountBlocked(
    accountId: string,
    blocked: boolean,
    reason: string,
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.POST(
      "/admin/global/accounts/{accountID}/block",
      {
        params: { path: { accountID: accountId } },
        body: {
          blocked,
          reason: reason as never,
        },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async listGlobalPublications(
    query?: GlobalAdminPublicationQuery,
    signal?: AbortSignal,
  ): Promise<GlobalAdminPublication[]> {
    const result = await apiClient.GET("/admin/global/publications", {
      params: { query: query as never },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (
      (result.data?.data?.items as unknown as GlobalAdminPublication[]) ?? []
    );
  }

  async setGlobalPublicationState(
    publicationId: string,
    input: {
      state: "activa" | "oculta";
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.POST(
      "/admin/global/publications/{publicationID}/state",
      {
        params: { path: { publicationID: publicationId } },
        body: input,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async retireGlobalPublication(
    publicationId: string,
    input: {
      owner_id: string;
      expected_updated_at: string;
      reason: string;
    },
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.POST(
      "/admin/global/publications/{publicationID}/retire",
      {
        params: { path: { publicationID: publicationId } },
        body: input as never,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  // -----------------------------------------------------
  // Local / Operativa Admin
  // -----------------------------------------------------

  async listVerifications(
    signal?: AbortSignal,
  ): Promise<AdminVerificationCase[]> {
    const result = await apiClient.GET("/admin/verifications", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (
      (result.data as unknown as { items?: AdminVerificationCase[] })?.items ??
      []
    );
  }

  async listRejectedVerifications(
    signal?: AbortSignal,
  ): Promise<AdminVerificationCase[]> {
    const result = await apiClient.GET("/admin/verifications/failed", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (
      (result.data as unknown as { items?: AdminVerificationCase[] })?.items ??
      []
    );
  }

  async reviewVerification(
    id: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
    signal?: AbortSignal,
  ): Promise<AdminVerificationCase> {
    const result = await apiClient.POST("/admin/verifications/{id}/review", {
      params: { path: { id } },
      body: { decision, reason } as never,
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data as unknown as AdminVerificationCase;
  }

  async revokeVerification(
    id: string,
    idempotencyKey: string,
    reason_code:
      "aprobacion_fixture_incorrecta" | "revision_fixture_actualizada",
    signal?: AbortSignal,
  ): Promise<AdminVerificationCase> {
    const result = await apiClient.POST("/admin/verifications/{id}/revoke", {
      params: {
        path: { id },
        header: { "Idempotency-Key": idempotencyKey },
      },
      body: { reason_code },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data as unknown as AdminVerificationCase;
  }

  async listDisputes(signal?: AbortSignal): Promise<AdminLocalDispute[]> {
    const result = await apiClient.GET("/admin/disputes", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (
      (result.data as unknown as { items?: AdminLocalDispute[] })?.items ?? []
    );
  }

  async closeDispute(
    disputeId: string,
    reason_code: "ensayo_finalizado" | "registro_erroneo" | "duplicada",
    signal?: AbortSignal,
  ): Promise<AdminLocalDispute> {
    const result = await apiClient.POST("/admin/disputes/{dispute_id}/close", {
      params: { path: { dispute_id: disputeId } },
      body: { reason_code },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data as unknown as AdminLocalDispute;
  }

  async listReviewReports(signal?: AbortSignal): Promise<AdminReviewReport[]> {
    const result = await apiClient.GET("/admin/local/review-reports", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data?.items as unknown as AdminReviewReport[]) ?? [];
  }

  async moderateReviewReport(
    reportId: string,
    action: "dismiss" | "hide",
    idempotencyKey: string,
    input: {
      reason_code:
        | "sin_infraccion"
        | "contenido_inadecuado"
        | "duplicado"
        | "error_registro";
      correlation_id: string;
    },
    signal?: AbortSignal,
  ): Promise<AdminReviewReport> {
    const result = await apiClient.POST(
      "/admin/local/review-reports/{reportID}/{action}",
      {
        params: {
          path: { reportID: reportId, action },
          header: { "Idempotency-Key": idempotencyKey },
        },
        body: input,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.data as unknown as AdminReviewReport;
  }

  async listNotices(signal?: AbortSignal): Promise<AdminLocalNotice[]> {
    const result = await apiClient.GET("/admin/local/notices", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data?.items as unknown as AdminLocalNotice[]) ?? [];
  }

  async reopenNotice(
    noticeId: string,
    idempotencyKey: string,
    input: {
      reason_code: "smtp_restaurado" | "reintento_operativo";
      correlation_id: string;
    },
    signal?: AbortSignal,
  ): Promise<AdminLocalNotice> {
    const result = await apiClient.POST(
      "/admin/local/notices/{noticeID}/reopen",
      {
        params: {
          path: { noticeID: noticeId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        body: input,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.data as unknown as AdminLocalNotice;
  }

  async listLocalAuditEvents(
    query: {
      from: string;
      until: string;
      actor_id?: string;
      resource_type?: string;
      resource_id?: string;
      action?: string;
      result?: "exito" | "rechazo";
    },
    signal?: AbortSignal,
  ): Promise<AuditEvent[]> {
    const result = await apiClient.GET("/admin/local/audit-events", {
      params: { query: query as never },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data?.items as unknown as AuditEvent[]) ?? [];
  }

  async exportLocalAuditEvents(
    query: {
      from: string;
      until: string;
      actor_id?: string;
      resource_type?: string;
      resource_id?: string;
      action?: string;
      result?: "exito" | "rechazo";
    },
    signal?: AbortSignal,
  ): Promise<Blob> {
    const result = await apiClient.GET("/admin/local/audit-events/export", {
      params: { query: query as never },
      parseAs: "blob",
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data as Blob) ?? (await result.response.blob());
  }

  async getLocalAccount(
    accountId: string,
    signal?: AbortSignal,
  ): Promise<AdminLocalAccount | null> {
    const result = await apiClient.GET("/admin/local/accounts/{accountID}", {
      params: { path: { accountID: accountId } },
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data as unknown as AdminLocalAccount) ?? null;
  }

  async blockLocalAccount(
    accountId: string,
    reason: string,
    signal?: AbortSignal,
  ): Promise<AdminLocalAccount> {
    const result = await apiClient.POST(
      "/admin/local/accounts/{accountID}/block",
      {
        params: { path: { accountID: accountId } },
        body: { reason } as never,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.data as unknown as AdminLocalAccount;
  }

  async unblockLocalAccount(
    accountId: string,
    reason: string,
    signal?: AbortSignal,
  ): Promise<AdminLocalAccount> {
    const result = await apiClient.POST(
      "/admin/local/accounts/{accountID}/unblock",
      {
        params: { path: { accountID: accountId } },
        body: { reason } as never,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.data as unknown as AdminLocalAccount;
  }

  async listPublishedSpaces(
    signal?: AbortSignal,
  ): Promise<AdminPublishedSpace[]> {
    const result = await apiClient.GET("/admin/local/published-spaces", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data?.items as unknown as AdminPublishedSpace[]) ?? [];
  }

  async getPublishedSpace(
    spaceId: string,
    signal?: AbortSignal,
  ): Promise<AdminPublishedSpace | null> {
    const result = await apiClient.GET(
      "/admin/local/published-spaces/{spaceID}",
      {
        params: { path: { spaceID: spaceId } },
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data as unknown as AdminPublishedSpace) ?? null;
  }

  async getReservationsReport(
    query: { from: string; until: string; timezone: string },
    signal?: AbortSignal,
  ): Promise<AdminReservationsReport> {
    const result = await apiClient.GET("/admin/local/reports/reservations", {
      params: { query },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as AdminReservationsReport;
  }

  async getFinanceReport(
    query: { from: string; until: string; timezone: string },
    signal?: AbortSignal,
  ): Promise<AdminFinanceReport> {
    const result = await apiClient.GET("/admin/local/reports/finance", {
      params: { query },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as AdminFinanceReport;
  }

  async getDisputesReport(
    query: { from: string; until: string; timezone: string },
    signal?: AbortSignal,
  ): Promise<AdminDisputesReport> {
    const result = await apiClient.GET("/admin/local/reports/disputes", {
      params: { query },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return result.data.data as unknown as AdminDisputesReport;
  }

  async listLocalReservations(
    query?: {
      reservation_id?: string;
      state?: string;
      created_from?: string;
      created_to?: string;
      page_size?: number;
      cursor?: string;
    },
    signal?: AbortSignal,
  ): Promise<AdminReservationSummary[]> {
    const result = await apiClient.GET("/admin/local/reservations", {
      params: { query: query as never },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (
      (result.data?.data?.items as unknown as AdminReservationSummary[]) ?? []
    );
  }

  async getLocalReservation(
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<AdminReservationDetail | null> {
    const result = await apiClient.GET(
      "/admin/local/reservations/{reservationID}",
      {
        params: { path: { reservationID: reservationId } },
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data as unknown as AdminReservationDetail) ?? null;
  }

  async listDamageClaims(signal?: AbortSignal): Promise<AdminDamageClaim[]> {
    const result = await apiClient.GET("/admin/local/damage-claims", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data?.items as unknown as AdminDamageClaim[]) ?? [];
  }

  async getDamageClaim(
    claimId: string,
    signal?: AbortSignal,
  ): Promise<AdminDamageClaim | null> {
    const result = await apiClient.GET("/admin/local/damage-claims/{claimID}", {
      params: { path: { claimID: claimId } },
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data?.data as unknown as AdminDamageClaim) ?? null;
  }

  async resolveDamageClaim(
    claimId: string,
    idempotencyKey: string,
    input: AdminDamageClaimResolutionInput,
    signal?: AbortSignal,
  ): Promise<AdminDamageClaim> {
    const result = await apiClient.POST(
      "/admin/local/damage-claims/{claimID}/resolution",
      {
        params: {
          path: { claimID: claimId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        body: input as never,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.data as unknown as AdminDamageClaim;
  }
}
