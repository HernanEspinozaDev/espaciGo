"use client";

import { useCallback, useSyncExternalStore } from "react";
import { adminFacade } from "../admin-runtime";
import type { AdminSnapshot } from "../application/admin-facade";
import { useAuth } from "@/features/auth/ui/use-auth";
import type { AccessToken } from "@/features/auth/application/auth.port";
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
  loadGlobalMe(token?: AccessToken | null): Promise<void>;
  loadMe(token?: AccessToken | null): Promise<void>;
  loadOverview(token?: AccessToken | null): Promise<void>;
  loadKPIs(
    query: { from: string; until: string; time_zone: string },
    token?: AccessToken | null,
  ): Promise<void>;
  loadGlobalAccounts(
    query?: GlobalAdminAccountQuery,
    token?: AccessToken | null,
  ): Promise<void>;
  loadAccounts(
    query?: GlobalAdminAccountQuery,
    token?: AccessToken | null,
  ): Promise<void>;
  selectGlobalAccount(
    accountId: string,
    token?: AccessToken | null,
  ): Promise<void>;
  updateGlobalAccountProfile(
    accountId: string,
    input: {
      display_name: string;
      phone?: string | null;
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
    token?: AccessToken | null,
  ): Promise<boolean>;
  setGlobalAccountRole(
    accountId: string,
    role: "arrendatario" | "arrendador",
    granted: boolean,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
    token?: AccessToken | null,
  ): Promise<boolean>;
  updateAccountRole(
    accountId: string,
    role: "arrendatario" | "arrendador",
    granted: boolean,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
    token?: AccessToken | null,
  ): Promise<boolean>;
  setGlobalLegacyAdminRole(
    accountId: string,
    granted: boolean,
    reason:
      | "aprovisionamiento_sintetico"
      | "revision_administrativa"
      | "retirada_acceso",
    token?: AccessToken | null,
  ): Promise<boolean>;
  revokeGlobalAccountSessions(
    accountId: string,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
    token?: AccessToken | null,
  ): Promise<boolean>;
  revokeAccountSessions(
    accountId: string,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
    token?: AccessToken | null,
  ): Promise<boolean>;
  setGlobalAccountBlocked(
    accountId: string,
    blocked: boolean,
    reason: string,
    token?: AccessToken | null,
  ): Promise<boolean>;
  blockAccount(
    accountId: string,
    reason: string,
    token?: AccessToken | null,
  ): Promise<boolean>;
  unblockAccount(
    accountId: string,
    reason: string,
    token?: AccessToken | null,
  ): Promise<boolean>;
  loadGlobalPublications(
    query?: GlobalAdminPublicationQuery,
    token?: AccessToken | null,
  ): Promise<void>;
  loadPublications(
    query?: GlobalAdminPublicationQuery,
    token?: AccessToken | null,
  ): Promise<void>;
  setGlobalPublicationState(
    publicationId: string,
    input: {
      state: "activa" | "oculta";
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
    token?: AccessToken | null,
  ): Promise<boolean>;
  updatePublicationState(
    publicationId: string,
    input: {
      state: "activa" | "oculta";
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
    token?: AccessToken | null,
  ): Promise<boolean>;
  retireGlobalPublication(
    publicationId: string,
    input: {
      owner_id: string;
      expected_updated_at: string;
      reason: string;
    },
    token?: AccessToken | null,
  ): Promise<boolean>;
  retirePublication(
    publicationId: string,
    input: {
      owner_id: string;
      expected_updated_at: string;
      reason: string;
    },
    token?: AccessToken | null,
  ): Promise<boolean>;
  loadGlobalAuditEvents(
    query: AuditEventsQuery,
    token?: AccessToken | null,
  ): Promise<void>;
  loadAuditEvents(
    query: AuditEventsQuery,
    token?: AccessToken | null,
  ): Promise<void>;
  exportGlobalAuditEvents(
    query: AuditEventsQuery,
    token?: AccessToken | null,
  ): Promise<Blob | null>;
  exportAuditEvents(
    query: AuditEventsQuery,
    token?: AccessToken | null,
  ): Promise<Blob | null>;

  // Local admin
  loadVerifications(token?: AccessToken | null): Promise<void>;
  loadRejectedVerifications(token?: AccessToken | null): Promise<void>;
  reviewVerification(
    id: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
    token?: AccessToken | null,
  ): Promise<boolean>;
  revokeVerification(
    id: string,
    reason_code:
      "aprobacion_fixture_incorrecta" | "revision_fixture_actualizada",
    token?: AccessToken | null,
  ): Promise<boolean>;
  loadDisputes(token?: AccessToken | null): Promise<void>;
  closeDispute(
    disputeId: string,
    reason_code: "ensayo_finalizado" | "registro_erroneo" | "duplicada",
    token?: AccessToken | null,
  ): Promise<boolean>;
  loadReviewReports(token?: AccessToken | null): Promise<void>;
  moderateReviewReport(
    reportId: string,
    action: "dismiss" | "hide",
    reason_code:
      | "sin_infraccion"
      | "contenido_inadecuado"
      | "duplicado"
      | "error_registro",
    correlation_id?: string,
    token?: AccessToken | null,
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
    token?: AccessToken | null,
  ): Promise<boolean>;
  loadNotices(token?: AccessToken | null): Promise<void>;
  reopenNotice(
    noticeId: string,
    reason_code: "smtp_restaurado" | "reintento_operativo",
    correlation_id?: string,
    token?: AccessToken | null,
  ): Promise<boolean>;
  loadLocalAccount(
    accountId: string,
    token?: AccessToken | null,
  ): Promise<void>;
  blockLocalAccount(
    accountId: string,
    reason: string,
    token?: AccessToken | null,
  ): Promise<boolean>;
  unblockLocalAccount(
    accountId: string,
    reason: string,
    token?: AccessToken | null,
  ): Promise<boolean>;
  loadPublishedSpaces(token?: AccessToken | null): Promise<void>;
  loadReports(
    query: { from: string; until: string; timezone: string },
    token?: AccessToken | null,
  ): Promise<void>;
  loadLocalReservations(
    query?: {
      reservation_id?: string;
      state?: string;
      created_from?: string;
      created_to?: string;
      page_size?: number;
      cursor?: string;
    },
    token?: AccessToken | null,
  ): Promise<void>;
  loadLocalReservation(
    reservationId: string,
    token?: AccessToken | null,
  ): Promise<void>;
  loadDamageClaims(token?: AccessToken | null): Promise<void>;
  loadLocalDamageClaims(token?: AccessToken | null): Promise<void>;
  loadDamageClaim(claimId: string, token?: AccessToken | null): Promise<void>;
  resolveDamageClaim(
    claimId: string,
    input: AdminDamageClaimResolutionInput,
    token?: AccessToken | null,
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
  const { getAccessToken, snapshot: authSnapshot } = useAuth();

  const resolveToken = useCallback(
    (explicitToken?: AccessToken | null): AccessToken | null => {
      return explicitToken ?? getAccessToken();
    },
    [getAccessToken],
  );

  // Only one role shown in UI per session
  const activeRole: AdminRole | null = ((): AdminRole | null => {
    const roles =
      snapshot.me?.roles ?? (authSnapshot.roles as readonly AdminRole[]);
    if (roles.includes("administrador_global")) return "administrador_global";
    if (roles.includes("administrador")) return "administrador";
    if (roles.includes("arrendador")) return "arrendador";
    if (roles.includes("arrendatario")) return "arrendatario";
    return null;
  })();

  const canAccess = useCallback(
    (section: AdminSection): boolean => {
      const capabilities =
        snapshot.me?.capabilities ??
        (authSnapshot.capabilities as readonly AdminCapability[]);
      const roles =
        snapshot.me?.roles ?? (authSnapshot.roles as readonly AdminRole[]);
      return canAccessSection(capabilities, section, roles);
    },
    [snapshot.me, authSnapshot],
  );

  const hasAdminCapability = useCallback(
    (capability: AdminCapability): boolean => {
      const capabilities =
        snapshot.me?.capabilities ??
        (authSnapshot.capabilities as readonly AdminCapability[]);
      return hasCapability(capabilities, capability);
    },
    [snapshot.me, authSnapshot],
  );

  const loadGlobalMe = useCallback(
    async (token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadGlobalMe(tok);
    },
    [resolveToken],
  );

  const loadOverview = useCallback(
    async (token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadOverview(tok);
    },
    [resolveToken],
  );

  const loadKPIs = useCallback(
    async (
      query: { from: string; until: string; time_zone: string },
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadKPIs(tok, query);
    },
    [resolveToken],
  );

  const loadGlobalAccounts = useCallback(
    async (query?: GlobalAdminAccountQuery, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadGlobalAccounts(tok, query);
    },
    [resolveToken],
  );

  const selectGlobalAccount = useCallback(
    async (accountId: string, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.selectGlobalAccount(tok, accountId);
    },
    [resolveToken],
  );

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
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.updateGlobalAccountProfile(tok, accountId, input);
    },
    [resolveToken],
  );

  const setGlobalAccountRole = useCallback(
    async (
      accountId: string,
      role: "arrendatario" | "arrendador",
      granted: boolean,
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.setGlobalAccountRole(
        tok,
        accountId,
        role,
        granted,
        reason,
      );
    },
    [resolveToken],
  );

  const setGlobalLegacyAdminRole = useCallback(
    async (
      accountId: string,
      granted: boolean,
      reason:
        | "aprovisionamiento_sintetico"
        | "revision_administrativa"
        | "retirada_acceso",
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.setGlobalLegacyAdminRole(
        tok,
        accountId,
        granted,
        reason,
      );
    },
    [resolveToken],
  );

  const revokeGlobalAccountSessions = useCallback(
    async (
      accountId: string,
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.revokeGlobalAccountSessions(tok, accountId, reason);
    },
    [resolveToken],
  );

  const setGlobalAccountBlocked = useCallback(
    async (
      accountId: string,
      blocked: boolean,
      reason: string,
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.setGlobalAccountBlocked(
        tok,
        accountId,
        blocked,
        reason,
      );
    },
    [resolveToken],
  );

  const blockAccount = useCallback(
    async (accountId: string, reason: string, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.setGlobalAccountBlocked(tok, accountId, true, reason);
    },
    [resolveToken],
  );

  const unblockAccount = useCallback(
    async (accountId: string, reason: string, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.setGlobalAccountBlocked(tok, accountId, false, reason);
    },
    [resolveToken],
  );

  const loadGlobalPublications = useCallback(
    async (query?: GlobalAdminPublicationQuery, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadGlobalPublications(tok, query);
    },
    [resolveToken],
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
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.setGlobalPublicationState(tok, publicationId, input);
    },
    [resolveToken],
  );

  const retireGlobalPublication = useCallback(
    async (
      publicationId: string,
      input: {
        owner_id: string;
        expected_updated_at: string;
        reason: string;
      },
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.retireGlobalPublication(tok, publicationId, input);
    },
    [resolveToken],
  );

  const loadGlobalAuditEvents = useCallback(
    async (query: AuditEventsQuery, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadGlobalAuditEvents(tok, query);
    },
    [resolveToken],
  );

  const exportGlobalAuditEvents = useCallback(
    async (query: AuditEventsQuery, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return null;
      return adminFacade.exportGlobalAuditEvents(tok, query);
    },
    [resolveToken],
  );

  const loadVerifications = useCallback(
    async (token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadVerifications(tok);
    },
    [resolveToken],
  );

  const loadRejectedVerifications = useCallback(
    async (token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadRejectedVerifications(tok);
    },
    [resolveToken],
  );

  const reviewVerification = useCallback(
    async (
      id: string,
      decision: "aprobar" | "rechazar",
      reason?: string,
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.reviewVerification(tok, id, decision, reason);
    },
    [resolveToken],
  );

  const revokeVerification = useCallback(
    async (
      id: string,
      reason_code:
        "aprobacion_fixture_incorrecta" | "revision_fixture_actualizada",
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.revokeVerification(tok, id, reason_code);
    },
    [resolveToken],
  );

  const loadDisputes = useCallback(
    async (token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadDisputes(tok);
    },
    [resolveToken],
  );

  const closeDispute = useCallback(
    async (
      disputeId: string,
      reason_code: "ensayo_finalizado" | "registro_erroneo" | "duplicada",
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.closeDispute(tok, disputeId, reason_code);
    },
    [resolveToken],
  );

  const loadReviewReports = useCallback(
    async (token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadReviewReports(tok);
    },
    [resolveToken],
  );

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
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.moderateReviewReport(
        tok,
        reportId,
        action,
        reason_code,
        correlation_id,
      );
    },
    [resolveToken],
  );

  const loadNotices = useCallback(
    async (token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadNotices(tok);
    },
    [resolveToken],
  );

  const reopenNotice = useCallback(
    async (
      noticeId: string,
      reason_code: "smtp_restaurado" | "reintento_operativo",
      correlation_id?: string,
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.reopenNotice(
        tok,
        noticeId,
        reason_code,
        correlation_id,
      );
    },
    [resolveToken],
  );

  const loadLocalAccount = useCallback(
    async (accountId: string, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadLocalAccount(tok, accountId);
    },
    [resolveToken],
  );

  const blockLocalAccount = useCallback(
    async (accountId: string, reason: string, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.blockLocalAccount(tok, accountId, reason);
    },
    [resolveToken],
  );

  const unblockLocalAccount = useCallback(
    async (accountId: string, reason: string, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.unblockLocalAccount(tok, accountId, reason);
    },
    [resolveToken],
  );

  const loadPublishedSpaces = useCallback(
    async (token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadPublishedSpaces(tok);
    },
    [resolveToken],
  );

  const loadReports = useCallback(
    async (
      query: { from: string; until: string; timezone: string },
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadReports(tok, query);
    },
    [resolveToken],
  );

  const loadLocalReservations = useCallback(
    async (
      query?: {
        reservation_id?: string;
        state?: string;
        created_from?: string;
        created_to?: string;
        page_size?: number;
        cursor?: string;
      },
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadLocalReservations(tok, query);
    },
    [resolveToken],
  );

  const loadLocalReservation = useCallback(
    async (reservationId: string, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadLocalReservation(tok, reservationId);
    },
    [resolveToken],
  );

  const loadDamageClaims = useCallback(
    async (token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadDamageClaims(tok);
    },
    [resolveToken],
  );

  const loadDamageClaim = useCallback(
    async (claimId: string, token?: AccessToken | null) => {
      const tok = resolveToken(token);
      if (!tok) return;
      await adminFacade.loadDamageClaim(tok, claimId);
    },
    [resolveToken],
  );

  const resolveDamageClaim = useCallback(
    async (
      claimId: string,
      input: AdminDamageClaimResolutionInput,
      token?: AccessToken | null,
    ) => {
      const tok = resolveToken(token);
      if (!tok) return false;
      return adminFacade.resolveDamageClaim(tok, claimId, input);
    },
    [resolveToken],
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
