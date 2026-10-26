"use client";

import { useCallback, useSyncExternalStore } from "react";
import { adminFacade } from "../admin-runtime";
import type { AdminSnapshot } from "../application/admin-facade";
import { authFacade } from "@/features/auth/auth-runtime";
import { canAccessSection, hasCapability } from "../domain/admin-rules";
import type {
  AdminCapability,
  AdminDamageClaimResolutionInput,
  AdminRole,
  AdminSection,
  AuditEventsQuery,
  GlobalAdminAccountQuery,
  GlobalAdminPublicationQuery,
} from "../domain/types";

export interface AdminView {
  snapshot: AdminSnapshot;
  canAccess(section: AdminSection): boolean;
  hasAdminCapability(capability: AdminCapability): boolean;
  activeRole: AdminRole | null;

  // Global admin
  loadGlobalMe(): Promise<void>;
  loadMe(): Promise<void>;
  loadOverview(): Promise<void>;
  loadKPIs(query: {
    from: string;
    until: string;
    time_zone: string;
  }): Promise<void>;
  loadGlobalAccounts(query?: GlobalAdminAccountQuery): Promise<void>;
  loadAccounts(query?: GlobalAdminAccountQuery): Promise<void>;
  selectGlobalAccount(accountId: string): Promise<void>;
  updateGlobalAccountProfile(
    accountId: string,
    input: {
      display_name: string;
      phone?: string | null;
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
  ): Promise<boolean>;
  setGlobalAccountRole(
    accountId: string,
    role: "arrendatario" | "arrendador",
    granted: boolean,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
  ): Promise<boolean>;
  updateAccountRole(
    accountId: string,
    role: "arrendatario" | "arrendador",
    granted: boolean,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
  ): Promise<boolean>;
  setGlobalLegacyAdminRole(
    accountId: string,
    granted: boolean,
    reason:
      | "aprovisionamiento_sintetico"
      | "revision_administrativa"
      | "retirada_acceso",
  ): Promise<boolean>;
  revokeGlobalAccountSessions(
    accountId: string,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
  ): Promise<boolean>;
  revokeAccountSessions(
    accountId: string,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
  ): Promise<boolean>;
  setGlobalAccountBlocked(
    accountId: string,
    blocked: boolean,
    reason: string,
  ): Promise<boolean>;
  blockAccount(accountId: string, reason: string): Promise<boolean>;
  unblockAccount(accountId: string, reason: string): Promise<boolean>;
  loadGlobalPublications(query?: GlobalAdminPublicationQuery): Promise<void>;
  loadPublications(query?: GlobalAdminPublicationQuery): Promise<void>;
  setGlobalPublicationState(
    publicationId: string,
    input: {
      state: "activa" | "oculta";
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
  ): Promise<boolean>;
  updatePublicationState(
    publicationId: string,
    input: {
      state: "activa" | "oculta";
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
  ): Promise<boolean>;
  retireGlobalPublication(
    publicationId: string,
    input: {
      owner_id: string;
      expected_updated_at: string;
      reason: string;
    },
  ): Promise<boolean>;
  retirePublication(
    publicationId: string,
    input: {
      owner_id: string;
      expected_updated_at: string;
      reason: string;
    },
  ): Promise<boolean>;
  loadGlobalAuditEvents(query: AuditEventsQuery): Promise<void>;
  loadAuditEvents(query: AuditEventsQuery): Promise<void>;
  exportGlobalAuditEvents(query: AuditEventsQuery): Promise<Blob | null>;
  exportAuditEvents(query: AuditEventsQuery): Promise<Blob | null>;

  // Local admin
  loadVerifications(): Promise<void>;
  loadRejectedVerifications(): Promise<void>;
  reviewVerification(
    id: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
  ): Promise<boolean>;
  revokeVerification(
    id: string,
    reason_code:
      "aprobacion_fixture_incorrecta" | "revision_fixture_actualizada",
  ): Promise<boolean>;
  loadDisputes(): Promise<void>;
  closeDispute(
    disputeId: string,
    reason_code: "ensayo_finalizado" | "registro_erroneo" | "duplicada",
  ): Promise<boolean>;
  loadReviewReports(): Promise<void>;
  moderateReviewReport(
    reportId: string,
    action: "dismiss" | "hide",
    reason_code:
      | "sin_infraccion"
      | "contenido_inadecuado"
      | "duplicado"
      | "error_registro",
    correlation_id?: string,
  ): Promise<boolean>;
  handleReviewReport(
    reportId: string,
    action: "dismiss" | "hide",
    reason_code:
      | "sin_infraccion"
      | "contenido_inadecuado"
      | "duplicado"
      | "error_registro",
    correlation_id?: string,
  ): Promise<boolean>;
  loadNotices(): Promise<void>;
  reopenNotice(
    noticeId: string,
    reason_code: "smtp_restaurado" | "reintento_operativo",
    correlation_id?: string,
  ): Promise<boolean>;
  loadLocalAccount(accountId: string): Promise<void>;
  blockLocalAccount(accountId: string, reason: string): Promise<boolean>;
  unblockLocalAccount(accountId: string, reason: string): Promise<boolean>;
  loadPublishedSpaces(): Promise<void>;
  loadReports(query: {
    from: string;
    until: string;
    timezone: string;
  }): Promise<void>;
  loadLocalReservations(query?: {
    reservation_id?: string;
    state?: string;
    created_from?: string;
    created_to?: string;
    page_size?: number;
    cursor?: string;
  }): Promise<void>;
  loadLocalReservation(reservationId: string): Promise<void>;
  loadDamageClaims(): Promise<void>;
  loadLocalDamageClaims(): Promise<void>;
  loadDamageClaim(claimId: string): Promise<void>;
  resolveDamageClaim(
    claimId: string,
    input: AdminDamageClaimResolutionInput,
  ): Promise<boolean>;

  // Memory & error gates
  clearError(): void;
  clearFeedback(): void;
  purgePrivateData(): void;
}

export function useAdmin(): AdminView {
  const snapshot = useSyncExternalStore(
    adminFacade.subscribe,
    adminFacade.getSnapshot,
    adminFacade.getServerSnapshot,
  );
  const authSnapshot = useSyncExternalStore(
    authFacade.subscribe,
    authFacade.getSnapshot,
    authFacade.getServerSnapshot,
  );

  // Only one role shown in UI per session
  const activeRole: AdminRole | null = ((): AdminRole | null => {
    if (authSnapshot.status !== "authenticated" || !authSnapshot.accountId) {
      return null;
    }
    const me =
      snapshot.me && snapshot.me.account_id === authSnapshot.accountId
        ? snapshot.me
        : null;
    const roles = (me?.roles ?? authSnapshot.roles) as readonly AdminRole[];
    if (roles.includes("administrador_global")) return "administrador_global";
    if (roles.includes("administrador")) return "administrador";
    if (roles.includes("arrendador")) return "arrendador";
    if (roles.includes("arrendatario")) return "arrendatario";
    return null;
  })();

  const canAccess = useCallback(
    (section: AdminSection): boolean => {
      if (authSnapshot.status !== "authenticated" || !authSnapshot.accountId) {
        return false;
      }
      const me =
        snapshot.me && snapshot.me.account_id === authSnapshot.accountId
          ? snapshot.me
          : null;
      const capabilities = (me?.capabilities ??
        authSnapshot.capabilities) as readonly AdminCapability[];
      const roles = (me?.roles ?? authSnapshot.roles) as readonly AdminRole[];
      return canAccessSection(capabilities, section, roles);
    },
    [snapshot.me, authSnapshot],
  );

  const hasAdminCapability = useCallback(
    (capability: AdminCapability): boolean => {
      if (authSnapshot.status !== "authenticated" || !authSnapshot.accountId) {
        return false;
      }
      const me =
        snapshot.me && snapshot.me.account_id === authSnapshot.accountId
          ? snapshot.me
          : null;
      const capabilities = (me?.capabilities ??
        authSnapshot.capabilities) as readonly AdminCapability[];
      return hasCapability(capabilities, capability);
    },
    [snapshot.me, authSnapshot],
  );

  const loadGlobalMe = useCallback(async () => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadGlobalMe();
  }, []);

  const loadOverview = useCallback(async () => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadOverview();
  }, []);

  const loadKPIs = useCallback(
    async (query: { from: string; until: string; time_zone: string }) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return;
      await adminFacade.loadKPIs(query);
    },
    [],
  );

  const loadGlobalAccounts = useCallback(
    async (query?: GlobalAdminAccountQuery) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return;
      await adminFacade.loadGlobalAccounts(query);
    },
    [],
  );

  const selectGlobalAccount = useCallback(async (accountId: string) => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.selectGlobalAccount(accountId);
  }, []);

  const updateGlobalAccountProfile = useCallback(
    async (
      accountId: string,
      input: {
        display_name: string;
        phone?: string | null;
        expected_updated_at: string;
        reason:
          "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
      },
    ) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.updateGlobalAccountProfile(accountId, input);
    },
    [],
  );

  const setGlobalAccountRole = useCallback(
    async (
      accountId: string,
      role: "arrendatario" | "arrendador",
      granted: boolean,
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
    ) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.setGlobalAccountRole(accountId, role, granted, reason);
    },
    [],
  );

  const setGlobalLegacyAdminRole = useCallback(
    async (
      accountId: string,
      granted: boolean,
      reason:
        | "aprovisionamiento_sintetico"
        | "revision_administrativa"
        | "retirada_acceso",
    ) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.setGlobalLegacyAdminRole(accountId, granted, reason);
    },
    [],
  );

  const revokeGlobalAccountSessions = useCallback(
    async (
      accountId: string,
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
    ) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.revokeGlobalAccountSessions(accountId, reason);
    },
    [],
  );

  const setGlobalAccountBlocked = useCallback(
    async (accountId: string, blocked: boolean, reason: string) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.setGlobalAccountBlocked(accountId, blocked, reason);
    },
    [],
  );

  const blockAccount = useCallback(
    async (accountId: string, reason: string) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.setGlobalAccountBlocked(accountId, true, reason);
    },
    [],
  );

  const unblockAccount = useCallback(
    async (accountId: string, reason: string) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.setGlobalAccountBlocked(accountId, false, reason);
    },
    [],
  );

  const loadGlobalPublications = useCallback(
    async (query?: GlobalAdminPublicationQuery) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return;
      await adminFacade.loadGlobalPublications(query);
    },
    [],
  );

  const setGlobalPublicationState = useCallback(
    async (
      publicationId: string,
      input: {
        state: "activa" | "oculta";
        expected_updated_at: string;
        reason:
          "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
      },
    ) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.setGlobalPublicationState(publicationId, input);
    },
    [],
  );

  const retireGlobalPublication = useCallback(
    async (
      publicationId: string,
      input: {
        owner_id: string;
        expected_updated_at: string;
        reason: string;
      },
    ) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.retireGlobalPublication(publicationId, input);
    },
    [],
  );

  const loadGlobalAuditEvents = useCallback(async (query: AuditEventsQuery) => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadGlobalAuditEvents(query);
  }, []);

  const exportGlobalAuditEvents = useCallback(
    async (query: AuditEventsQuery) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return null;
      return adminFacade.exportGlobalAuditEvents(query);
    },
    [],
  );

  const loadVerifications = useCallback(async () => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadVerifications();
  }, []);

  const loadRejectedVerifications = useCallback(async () => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadRejectedVerifications();
  }, []);

  const reviewVerification = useCallback(
    async (id: string, decision: "aprobar" | "rechazar", reason?: string) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.reviewVerification(id, decision, reason);
    },
    [],
  );

  const revokeVerification = useCallback(
    async (
      id: string,
      reason_code:
        "aprobacion_fixture_incorrecta" | "revision_fixture_actualizada",
    ) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.revokeVerification(id, reason_code);
    },
    [],
  );

  const loadDisputes = useCallback(async () => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadDisputes();
  }, []);

  const closeDispute = useCallback(
    async (
      disputeId: string,
      reason_code: "ensayo_finalizado" | "registro_erroneo" | "duplicada",
    ) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.closeDispute(disputeId, reason_code);
    },
    [],
  );

  const loadReviewReports = useCallback(async () => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadReviewReports();
  }, []);

  const moderateReviewReport = useCallback(
    async (
      reportId: string,
      action: "dismiss" | "hide",
      reason_code:
        | "sin_infraccion"
        | "contenido_inadecuado"
        | "duplicado"
        | "error_registro",
      correlation_id?: string,
    ) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.moderateReviewReport(
        reportId,
        action,
        reason_code,
        correlation_id,
      );
    },
    [],
  );

  const loadNotices = useCallback(async () => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadNotices();
  }, []);

  const reopenNotice = useCallback(
    async (
      noticeId: string,
      reason_code: "smtp_restaurado" | "reintento_operativo",
      correlation_id?: string,
    ) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.reopenNotice(noticeId, reason_code, correlation_id);
    },
    [],
  );

  const loadLocalAccount = useCallback(async (accountId: string) => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadLocalAccount(accountId);
  }, []);

  const blockLocalAccount = useCallback(
    async (accountId: string, reason: string) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.blockLocalAccount(accountId, reason);
    },
    [],
  );

  const unblockLocalAccount = useCallback(
    async (accountId: string, reason: string) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.unblockLocalAccount(accountId, reason);
    },
    [],
  );

  const loadPublishedSpaces = useCallback(async () => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadPublishedSpaces();
  }, []);

  const loadReports = useCallback(
    async (query: { from: string; until: string; timezone: string }) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return;
      await adminFacade.loadReports(query);
    },
    [],
  );

  const loadLocalReservations = useCallback(
    async (query?: {
      reservation_id?: string;
      state?: string;
      created_from?: string;
      created_to?: string;
      page_size?: number;
      cursor?: string;
    }) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return;
      await adminFacade.loadLocalReservations(query);
    },
    [],
  );

  const loadLocalReservation = useCallback(async (reservationId: string) => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadLocalReservation(reservationId);
  }, []);

  const loadDamageClaims = useCallback(async () => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadDamageClaims();
  }, []);

  const loadDamageClaim = useCallback(async (claimId: string) => {
    const isConfirmedSession =
      authFacade.getSnapshot().status === "authenticated";
    if (!isConfirmedSession) return;
    await adminFacade.loadDamageClaim(claimId);
  }, []);

  const resolveDamageClaim = useCallback(
    async (claimId: string, input: AdminDamageClaimResolutionInput) => {
      const isConfirmedSession =
        authFacade.getSnapshot().status === "authenticated";
      if (!isConfirmedSession) return false;
      return adminFacade.resolveDamageClaim(claimId, input);
    },
    [],
  );

  const clearError = useCallback(() => {
    adminFacade.clearError();
  }, []);

  const purgePrivateData = useCallback(() => {
    adminFacade.purgePrivateData();
  }, []);

  return {
    snapshot,
    canAccess,
    hasAdminCapability,
    activeRole,

    loadGlobalMe,
    loadMe: loadGlobalMe,
    loadOverview,
    loadKPIs,
    loadGlobalAccounts,
    loadAccounts: loadGlobalAccounts,
    selectGlobalAccount,
    updateGlobalAccountProfile,
    setGlobalAccountRole,
    updateAccountRole: setGlobalAccountRole,
    setGlobalLegacyAdminRole,
    revokeGlobalAccountSessions,
    revokeAccountSessions: revokeGlobalAccountSessions,
    setGlobalAccountBlocked,
    blockAccount,
    unblockAccount,
    loadGlobalPublications,
    loadPublications: loadGlobalPublications,
    setGlobalPublicationState,
    updatePublicationState: setGlobalPublicationState,
    retireGlobalPublication,
    retirePublication: retireGlobalPublication,
    loadGlobalAuditEvents,
    loadAuditEvents: loadGlobalAuditEvents,
    exportGlobalAuditEvents,
    exportAuditEvents: exportGlobalAuditEvents,

    loadVerifications,
    loadRejectedVerifications,
    reviewVerification,
    revokeVerification,
    loadDisputes,
    closeDispute,
    loadReviewReports,
    moderateReviewReport,
    handleReviewReport: moderateReviewReport,
    loadNotices,
    reopenNotice,
    loadLocalAccount,
    blockLocalAccount,
    unblockLocalAccount,
    loadPublishedSpaces,
    loadReports,
    loadLocalReservations,
    loadLocalReservation,
    loadDamageClaims,
    loadLocalDamageClaims: loadDamageClaims,
    loadDamageClaim,
    resolveDamageClaim,

    clearError,
    clearFeedback: clearError,
    purgePrivateData,
  };
}
