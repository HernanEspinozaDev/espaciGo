"use client";
import "client-only";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import type { AuthSnapshot } from "@/features/auth/application/auth-facade";
import {
  canAccessSection,
  generateIdempotencyKey,
  hasCapability,
  isConcurrencyConflict,
  isForbiddenError,
  isRecentAuthRequired,
  isValidAdminReason,
  isValidExpectedUpdatedAt,
} from "../domain/admin-rules";
interface AdminSessionScope {
  epoch: number;
  signal: AbortSignal;
  signals: AbortSignal[];
}
export type AdminAuthSource = {
  getSnapshot: () => AuthSnapshot;
  subscribe: (listener: () => void) => () => void;
  checkSession: () => Promise<boolean>;
};
export type AdminPrivatePurgeListener =
  | (() => void)
  | {
      onPrivatePurge?: () => void;
    };
function isUnauthorizedError(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const candidate = error as {
    status?: unknown;
    statusCode?: unknown;
    code?: unknown;
  };
  return (
    candidate.status === 401 ||
    candidate.statusCode === 401 ||
    candidate.code === "unauthorized" ||
    candidate.code === "UNAUTHORIZED"
  );
}
import type {
  AdminCapability,
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
  AdminSection,
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
import type { AdminPort } from "./admin.port";
export interface AdminSnapshot {
  readonly me: GlobalAdminMe | null;
  readonly overview: GlobalAdminOverview | null;
  readonly kpis: GlobalAdminKPIs | null;
  readonly globalAccounts: readonly GlobalAdminAccount[];
  readonly selectedGlobalAccount: GlobalAdminAccount | null;
  readonly globalPublications: readonly GlobalAdminPublication[];
  readonly globalAuditEvents: readonly AuditEvent[];
  readonly verifications: readonly AdminVerificationCase[];
  readonly rejectedVerifications: readonly AdminVerificationCase[];
  readonly disputes: readonly AdminLocalDispute[];
  readonly reviewReports: readonly AdminReviewReport[];
  readonly notices: readonly AdminLocalNotice[];
  readonly localAccounts: readonly AdminLocalAccount[];
  readonly publishedSpaces: readonly AdminPublishedSpace[];
  readonly reservations: readonly AdminReservationSummary[];
  readonly currentReservation: AdminReservationDetail | null;
  readonly damageClaims: readonly AdminDamageClaim[];
  readonly currentDamageClaim: AdminDamageClaim | null;
  readonly reservationsReport: AdminReservationsReport | null;
  readonly financeReport: AdminFinanceReport | null;
  readonly disputesReport: AdminDisputesReport | null;
  readonly loading: boolean;
  readonly actionLoading: boolean;
  readonly error: string | null;
  readonly recentAuthRequired: boolean;
  readonly concurrencyConflict: boolean;
}
const initialSnapshot: AdminSnapshot = Object.freeze({
  me: null,
  overview: null,
  kpis: null,
  globalAccounts: Object.freeze([]),
  selectedGlobalAccount: null,
  globalPublications: Object.freeze([]),
  globalAuditEvents: Object.freeze([]),
  verifications: Object.freeze([]),
  rejectedVerifications: Object.freeze([]),
  disputes: Object.freeze([]),
  reviewReports: Object.freeze([]),
  notices: Object.freeze([]),
  localAccounts: Object.freeze([]),
  publishedSpaces: Object.freeze([]),
  reservations: Object.freeze([]),
  currentReservation: null,
  damageClaims: Object.freeze([]),
  currentDamageClaim: null,
  reservationsReport: null,
  financeReport: null,
  disputesReport: null,
  loading: false,
  actionLoading: false,
  error: null,
  recentAuthRequired: false,
  concurrencyConflict: false,
});
export class AdminFacade {
  #port: AdminPort;
  #authSource?: AdminAuthSource;
  #onPrivatePurge?: () => void;
  #authUnsubscribe: (() => void) | null = null;
  #lastAuthSnapshot: AuthSnapshot | null = null;
  #abortController: AbortController | null = null;
  #epoch = 0;
  #sessionController = new AbortController();
  #snapshot: AdminSnapshot = initialSnapshot;
  #listeners = new Set<() => void>();
  constructor(
    port: AdminPort,
    authSource?: AdminAuthSource,
    onPrivatePurge?: AdminPrivatePurgeListener,
  ) {
    this.#port = port;
    this.#authSource = authSource;
    this.#onPrivatePurge =
      typeof onPrivatePurge === "function"
        ? onPrivatePurge
        : onPrivatePurge?.onPrivatePurge?.bind(onPrivatePurge);
    if (this.#authSource) {
      this.#lastAuthSnapshot = this.#authSource.getSnapshot();
      this.#authUnsubscribe = this.#authSource.subscribe(() => {
        this.#syncWithAuthSource();
      });
    }
  }
  #sameSession(prev: AuthSnapshot | null, current: AuthSnapshot): boolean {
    if (!prev) return false;
    if (prev.status !== "authenticated" || current.status !== "authenticated") {
      return false;
    }
    if (prev.accountId !== current.accountId) {
      return false;
    }
    if (prev.restrictedMode !== current.restrictedMode) {
      return false;
    }
    if (prev.expiresAt !== current.expiresAt) {
      return false;
    }
    const prevRoles = prev.roles ?? [];
    const currRoles = current.roles ?? [];
    if (
      prevRoles.length !== currRoles.length ||
      prevRoles.some((r: string, i: number) => r !== currRoles[i])
    ) {
      return false;
    }
    const prevCaps = prev.capabilities ?? [];
    const currCaps = current.capabilities ?? [];
    if (
      prevCaps.length !== currCaps.length ||
      prevCaps.some((c: string, i: number) => c !== currCaps[i])
    ) {
      return false;
    }
    return true;
  }
  #syncWithAuthSource(): void {
    if (!this.#authSource) return;
    const current = this.#authSource.getSnapshot();
    if (current.status !== "authenticated") {
      this.purgePrivateData();
    } else if (
      this.#lastAuthSnapshot &&
      !this.#sameSession(this.#lastAuthSnapshot, current)
    ) {
      this.purgePrivateData();
    }
    this.#lastAuthSnapshot = current;
  }
  #captureSession(): AdminSessionScope {
    return {
      epoch: this.#epoch,
      signal: this.#sessionController.signal,
      signals: [],
    };
  }
  #isSessionCurrent(scope: AdminSessionScope): boolean {
    return (
      scope.epoch === this.#epoch &&
      !scope.signal.aborted &&
      !scope.signals.some((signal) => signal.aborted)
    );
  }
  #sessionSignal(
    scope: AdminSessionScope,
    existing?: AbortSignal,
  ): AbortSignal {
    const signal = existing
      ? AbortSignal.any([scope.signal, existing])
      : scope.signal;
    scope.signals.push(signal);
    return signal;
  }
  async #waitSession<T>(
    value: T | Promise<T>,
    scope: AdminSessionScope,
  ): Promise<T> {
    const result = await value;
    if (!this.#isSessionCurrent(scope))
      throw new DOMException("Session changed", "AbortError");
    return result;
  }
  getSnapshot = (): AdminSnapshot => {
    return this.#snapshot;
  };
  getServerSnapshot = (): AdminSnapshot => {
    return initialSnapshot;
  };
  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  };
  cancel(): void {
    this.#epoch++;
    this.#sessionController.abort();
    this.#sessionController = new AbortController();
    if (this.#abortController) {
      this.#abortController.abort();
      this.#abortController = null;
    }
  }
  #setSnapshot(partial: Partial<AdminSnapshot>): void {
    this.#snapshot = Object.freeze({
      ...this.#snapshot,
      ...partial,
    });
    for (const listener of this.#listeners) {
      listener();
    }
  }
  clearError(): void {
    if (
      this.#snapshot.error ||
      this.#snapshot.recentAuthRequired ||
      this.#snapshot.concurrencyConflict
    ) {
      this.#setSnapshot({
        error: null,
        recentAuthRequired: false,
        concurrencyConflict: false,
      });
    }
  }
  #ensureSessionController(): {
    controller: AbortController;
    epoch: number;
  } {
    if (!this.#abortController || this.#abortController.signal.aborted) {
      this.#abortController = new AbortController();
    }
    return { controller: this.#abortController, epoch: this.#epoch };
  }
  #combineSignal(
    sessionSignal: AbortSignal,
    externalSignal?: AbortSignal,
  ): AbortSignal {
    if (!externalSignal) return sessionSignal;
    if (externalSignal.aborted) return externalSignal;
    if (sessionSignal.aborted) return sessionSignal;
    const anyFn = (
      AbortSignal as {
        any?: (signals: AbortSignal[]) => AbortSignal;
      }
    ).any;
    if (typeof anyFn === "function") {
      return anyFn([sessionSignal, externalSignal]);
    }
    const combined = new AbortController();
    const forward = () => combined.abort();
    sessionSignal.addEventListener("abort", forward, { once: true });
    externalSignal.addEventListener("abort", forward, { once: true });
    return combined.signal;
  }
  #isStale(epoch: number): boolean {
    return (
      epoch !== this.#epoch || Boolean(this.#abortController?.signal.aborted)
    );
  }
  /**
   * Gate Fase 9: Purga inmediata de datos privados en memoria tras revocación de sesión o HTTP 403
   */
  purgePrivateData(): void {
    this.cancel();
    privateBlobUrls.clear();
    try {
      this.#onPrivatePurge?.();
    } catch {
      // non-blocking purge listener
    }
    this.#setSnapshot({
      me: null,
      error: null,
      overview: null,
      kpis: null,
      globalAccounts: Object.freeze([]),
      selectedGlobalAccount: null,
      globalPublications: Object.freeze([]),
      globalAuditEvents: Object.freeze([]),
      verifications: Object.freeze([]),
      rejectedVerifications: Object.freeze([]),
      disputes: Object.freeze([]),
      reviewReports: Object.freeze([]),
      notices: Object.freeze([]),
      localAccounts: Object.freeze([]),
      publishedSpaces: Object.freeze([]),
      reservations: Object.freeze([]),
      currentReservation: null,
      damageClaims: Object.freeze([]),
      currentDamageClaim: null,
      reservationsReport: null,
      financeReport: null,
      disputesReport: null,
      loading: false,
      actionLoading: false,
      recentAuthRequired: false,
      concurrencyConflict: false,
    });
  }
  #handleError(err: unknown, fallbackMessage: string, epoch?: number): void {
    if (epoch !== undefined && this.#isStale(epoch)) {
      return;
    }
    if (isForbiddenError(err)) {
      this.purgePrivateData();
      void this.#authSource?.checkSession().catch(() => undefined);
      this.#setSnapshot({
        error:
          "Acceso denegado o sesión revocada. Los datos privados fueron purgados de la memoria.",
        loading: false,
        actionLoading: false,
      });
      return;
    }
    if (isUnauthorizedError(err)) {
      this.purgePrivateData();
      void this.#authSource?.checkSession().catch(() => undefined);
      this.#setSnapshot({
        error: "Sesión no autorizada o expirada.",
        loading: false,
        actionLoading: false,
      });
      return;
    }
    if (isRecentAuthRequired(err)) {
      this.#setSnapshot({
        recentAuthRequired: true,
        error:
          "Se requiere autenticación reciente para realizar esta operación administrativa sensible (código 428).",
        loading: false,
        actionLoading: false,
      });
      return;
    }
    if (isConcurrencyConflict(err)) {
      this.#setSnapshot({
        concurrencyConflict: true,
        error:
          "Conflicto de concurrencia: el recurso fue modificado por otra sesión administrativa (código 409).",
        loading: false,
        actionLoading: false,
      });
      return;
    }
    const message =
      err instanceof Error
        ? err.message
        : typeof (
              err as {
                message?: unknown;
              }
            )?.message === "string"
          ? (
              err as {
                message: string;
              }
            ).message
          : fallbackMessage;
    this.#setSnapshot({
      error: message,
      loading: false,
      actionLoading: false,
    });
  }
  // -----------------------------------------------------
  // Capabilities & Authorization Checks
  // -----------------------------------------------------
  canAccess(section: AdminSection): boolean {
    return canAccessSection(
      this.#snapshot.me?.capabilities,
      section,
      this.#snapshot.me?.roles,
    );
  }
  hasCapability(capability: AdminCapability): boolean {
    return hasCapability(this.#snapshot.me?.capabilities, capability);
  }
  // -----------------------------------------------------
  // Global Admin Actions
  // -----------------------------------------------------
  async loadGlobalMe(): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const { controller, epoch } = this.#ensureSessionController();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const me = await this.#waitSession(
        this.#port.getGlobalAdminMe(
          this.#sessionSignal(_sessionScope, controller.signal),
        ),
        _sessionScope,
      );
      if (this.#isStale(epoch)) return;
      this.#setSnapshot({ me, loading: false });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      if (this.#isStale(epoch)) return;
      this.#handleError(
        err,
        "No pudimos cargar las capacidades administrativas.",
        epoch,
      );
    }
  }
  async loadOverview(): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const { controller, epoch } = this.#ensureSessionController();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const overview = await this.#waitSession(
        this.#port.getGlobalAdminOverview(
          this.#sessionSignal(_sessionScope, controller.signal),
        ),
        _sessionScope,
      );
      if (this.#isStale(epoch)) return;
      this.#setSnapshot({ overview, loading: false });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      if (this.#isStale(epoch)) return;
      this.#handleError(err, "No pudimos cargar el resumen global.", epoch);
    }
  }
  async loadKPIs(query: {
    from: string;
    until: string;
    time_zone: string;
  }): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const kpis = await this.#waitSession(
        this.#port.getGlobalAdminKPIs(
          query,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({ kpis, loading: false });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar los KPIs globales.");
    }
  }
  async loadGlobalAccounts(query?: GlobalAdminAccountQuery): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const { items } = await this.#waitSession(
        this.#port.listGlobalAccounts(
          query,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        globalAccounts: Object.freeze(items),
        loading: false,
      });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar las cuentas globales.");
    }
  }
  async selectGlobalAccount(accountId: string): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const account = await this.#waitSession(
        this.#port.getGlobalAccount(
          accountId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({ selectedGlobalAccount: account, loading: false });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar el detalle de la cuenta.");
    }
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
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    if (!isValidExpectedUpdatedAt(input.expected_updated_at)) {
      this.#setSnapshot({
        error:
          "La fecha esperada de actualización (expected_updated_at) es inválida.",
      });
      return await this.#waitSession(false, _sessionScope);
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#waitSession(
        this.#port.updateGlobalAccountProfile(
          accountId,
          input,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      // Refresh selected account if currently loaded
      if (this.#snapshot.selectedGlobalAccount?.account_id === accountId) {
        await this.#waitSession(
          this.selectGlobalAccount(accountId),
          _sessionScope,
        );
      }
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos actualizar el perfil de la cuenta.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async setGlobalAccountRole(
    accountId: string,
    role: "arrendatario" | "arrendador",
    granted: boolean,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#waitSession(
        this.#port.setGlobalAccountRole(
          accountId,
          role,
          granted,
          reason,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      if (this.#snapshot.selectedGlobalAccount?.account_id === accountId) {
        await this.#waitSession(
          this.selectGlobalAccount(accountId),
          _sessionScope,
        );
      }
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos asignar el rol a la cuenta.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async setGlobalLegacyAdminRole(
    accountId: string,
    granted: boolean,
    reason:
      | "aprovisionamiento_sintetico"
      | "revision_administrativa"
      | "retirada_acceso",
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#waitSession(
        this.#port.setGlobalLegacyAdminRole(
          accountId,
          granted,
          reason,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      if (this.#snapshot.selectedGlobalAccount?.account_id === accountId) {
        await this.#waitSession(
          this.selectGlobalAccount(accountId),
          _sessionScope,
        );
      }
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos modificar el rol de administrador.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async revokeGlobalAccountSessions(
    accountId: string,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#waitSession(
        this.#port.revokeGlobalAccountSessions(
          accountId,
          reason,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      // If the revoked account is the current session account, purge private memory immediately
      if (this.#snapshot.me?.account_id === accountId) {
        this.purgePrivateData();
      }
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos revocar las sesiones de la cuenta.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async setGlobalAccountBlocked(
    accountId: string,
    blocked: boolean,
    reason: string,
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    if (!isValidAdminReason(reason)) {
      this.#setSnapshot({
        error:
          "El motivo de cambio de bloqueo debe contener al menos 5 caracteres válidos.",
      });
      return await this.#waitSession(false, _sessionScope);
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#waitSession(
        this.#port.setGlobalAccountBlocked(
          accountId,
          blocked,
          reason,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      if (this.#snapshot.selectedGlobalAccount?.account_id === accountId) {
        await this.#waitSession(
          this.selectGlobalAccount(accountId),
          _sessionScope,
        );
      }
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos modificar el bloqueo de la cuenta.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async loadGlobalPublications(
    query?: GlobalAdminPublicationQuery,
  ): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const pubs = await this.#waitSession(
        this.#port.listGlobalPublications(
          query,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        globalPublications: Object.freeze(pubs),
        loading: false,
      });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar las publicaciones globales.");
    }
  }
  async setGlobalPublicationState(
    publicationId: string,
    input: {
      state: "activa" | "oculta";
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    if (!isValidExpectedUpdatedAt(input.expected_updated_at)) {
      this.#setSnapshot({
        error:
          "La fecha esperada de actualización (expected_updated_at) es inválida.",
      });
      return await this.#waitSession(false, _sessionScope);
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#waitSession(
        this.#port.setGlobalPublicationState(
          publicationId,
          input,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      await this.#waitSession(this.loadGlobalPublications(), _sessionScope);
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos cambiar el estado de la publicación.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async retireGlobalPublication(
    publicationId: string,
    input: {
      owner_id: string;
      expected_updated_at: string;
      reason: string;
    },
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    if (!isValidAdminReason(input.reason)) {
      this.#setSnapshot({
        error:
          "El motivo de retiro debe contener al menos 5 caracteres válidos.",
      });
      return await this.#waitSession(false, _sessionScope);
    }
    if (!isValidExpectedUpdatedAt(input.expected_updated_at)) {
      this.#setSnapshot({
        error:
          "La fecha esperada de actualización (expected_updated_at) es inválida.",
      });
      return await this.#waitSession(false, _sessionScope);
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#waitSession(
        this.#port.retireGlobalPublication(
          publicationId,
          input,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      await this.#waitSession(this.loadGlobalPublications(), _sessionScope);
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos retirar la publicación.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async loadGlobalAuditEvents(query: AuditEventsQuery): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const events = await this.#waitSession(
        this.#port.listGlobalAuditEvents(
          query,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        globalAuditEvents: Object.freeze(events),
        loading: false,
      });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar los eventos de auditoría.");
    }
  }
  async exportGlobalAuditEvents(query: AuditEventsQuery): Promise<Blob | null> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return null;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const blob = await this.#waitSession(
        this.#port.exportGlobalAuditEvents(
          query,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(blob, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return null;
      }
      this.#handleError(err, "No pudimos exportar los eventos de auditoría.");
      return await this.#waitSession(null, _sessionScope);
    }
  }
  // -----------------------------------------------------
  // Local Admin Actions
  // -----------------------------------------------------
  async loadVerifications(): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const items = await this.#waitSession(
        this.#port.listVerifications(
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        verifications: Object.freeze(items),
        loading: false,
      });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar las verificaciones.");
    }
  }
  async loadRejectedVerifications(): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const items = await this.#waitSession(
        this.#port.listRejectedVerifications(
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        rejectedVerifications: Object.freeze(items),
        loading: false,
      });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(
        err,
        "No pudimos cargar las verificaciones rechazadas.",
      );
    }
  }
  async reviewVerification(
    id: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    if (decision === "rechazar" && !isValidAdminReason(reason)) {
      this.#setSnapshot({
        error:
          "El motivo de rechazo es obligatorio y debe tener al menos 5 caracteres válidos.",
      });
      return await this.#waitSession(false, _sessionScope);
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#waitSession(
        this.#port.reviewVerification(
          id,
          decision,
          reason,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      await this.#waitSession(this.loadVerifications(), _sessionScope);
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos resolver el caso de verificación.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async revokeVerification(
    id: string,
    reason_code:
      "aprobacion_fixture_incorrecta" | "revision_fixture_actualizada",
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const idempotencyKey = generateIdempotencyKey();
      await this.#waitSession(
        this.#port.revokeVerification(
          id,
          idempotencyKey,
          reason_code,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      await this.#waitSession(this.loadVerifications(), _sessionScope);
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos revocar la verificación.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async loadDisputes(): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const disputes = await this.#waitSession(
        this.#port.listDisputes(this.#sessionSignal(_sessionScope, undefined)),
        _sessionScope,
      );
      this.#setSnapshot({ disputes: Object.freeze(disputes), loading: false });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar las disputas locales.");
    }
  }
  async closeDispute(
    disputeId: string,
    reason_code: "ensayo_finalizado" | "registro_erroneo" | "duplicada",
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#waitSession(
        this.#port.closeDispute(
          disputeId,
          reason_code,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      await this.#waitSession(this.loadDisputes(), _sessionScope);
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos cerrar la disputa.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async loadReviewReports(): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const reports = await this.#waitSession(
        this.#port.listReviewReports(
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        reviewReports: Object.freeze(reports),
        loading: false,
      });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar los reportes de reseñas.");
    }
  }
  async moderateReviewReport(
    reportId: string,
    action: "dismiss" | "hide",
    reason_code:
      | "sin_infraccion"
      | "contenido_inadecuado"
      | "duplicado"
      | "error_registro",
    correlation_id?: string,
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const idempotencyKey = generateIdempotencyKey();
      await this.#waitSession(
        this.#port.moderateReviewReport(
          reportId,
          action,
          idempotencyKey,
          {
            reason_code,
            correlation_id: correlation_id ?? generateIdempotencyKey(),
          },
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      await this.#waitSession(this.loadReviewReports(), _sessionScope);
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos moderar el reporte de reseña.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async loadNotices(): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const notices = await this.#waitSession(
        this.#port.listNotices(this.#sessionSignal(_sessionScope, undefined)),
        _sessionScope,
      );
      this.#setSnapshot({ notices: Object.freeze(notices), loading: false });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar los avisos locales.");
    }
  }
  async reopenNotice(
    noticeId: string,
    reason_code: "smtp_restaurado" | "reintento_operativo",
    correlation_id?: string,
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const idempotencyKey = generateIdempotencyKey();
      await this.#waitSession(
        this.#port.reopenNotice(
          noticeId,
          idempotencyKey,
          {
            reason_code,
            correlation_id: correlation_id ?? generateIdempotencyKey(),
          },
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      await this.#waitSession(this.loadNotices(), _sessionScope);
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos reabrir el aviso.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async loadLocalAccount(accountId: string): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const account = await this.#waitSession(
        this.#port.getLocalAccount(
          accountId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      if (account) {
        this.#setSnapshot({
          localAccounts: Object.freeze([account]),
          loading: false,
        });
      } else {
        this.#setSnapshot({ loading: false });
      }
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar la cuenta local.");
    }
  }
  async blockLocalAccount(accountId: string, reason: string): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    if (!isValidAdminReason(reason)) {
      this.#setSnapshot({
        error:
          "El motivo de bloqueo local debe contener al menos 5 caracteres válidos.",
      });
      return await this.#waitSession(false, _sessionScope);
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#waitSession(
        this.#port.blockLocalAccount(
          accountId,
          reason,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        localAccounts: Object.freeze([updated]),
        actionLoading: false,
      });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos bloquear la cuenta local.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async unblockLocalAccount(
    accountId: string,
    reason: string,
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    if (!isValidAdminReason(reason)) {
      this.#setSnapshot({
        error:
          "El motivo de desbloqueo debe contener al menos 5 caracteres válidos.",
      });
      return await this.#waitSession(false, _sessionScope);
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#waitSession(
        this.#port.unblockLocalAccount(
          accountId,
          reason,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        localAccounts: Object.freeze([updated]),
        actionLoading: false,
      });
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos desbloquear la cuenta local.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
  async loadPublishedSpaces(): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const spaces = await this.#waitSession(
        this.#port.listPublishedSpaces(
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        publishedSpaces: Object.freeze(spaces),
        loading: false,
      });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar los espacios publicados.");
    }
  }
  async loadReports(query: {
    from: string;
    until: string;
    timezone: string;
  }): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const [reservationsReport, financeReport, disputesReport] =
        await this.#waitSession(
          Promise.all([
            this.#port.getReservationsReport(
              query,
              this.#sessionSignal(_sessionScope, undefined),
            ),
            this.#port.getFinanceReport(
              query,
              this.#sessionSignal(_sessionScope, undefined),
            ),
            this.#port.getDisputesReport(
              query,
              this.#sessionSignal(_sessionScope, undefined),
            ),
          ]),
          _sessionScope,
        );
      this.#setSnapshot({
        reservationsReport,
        financeReport,
        disputesReport,
        loading: false,
      });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar los reportes operativos.");
    }
  }
  async loadLocalReservations(query?: {
    reservation_id?: string;
    state?: string;
    created_from?: string;
    created_to?: string;
    page_size?: number;
    cursor?: string;
  }): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const items = await this.#waitSession(
        this.#port.listLocalReservations(
          query,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({ reservations: Object.freeze(items), loading: false });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar las reservas administrativas.");
    }
  }
  async loadLocalReservation(reservationId: string): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#waitSession(
        this.#port.getLocalReservation(
          reservationId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({ currentReservation: detail, loading: false });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar el detalle de la reserva.");
    }
  }
  async loadDamageClaims(): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const claims = await this.#waitSession(
        this.#port.listDamageClaims(
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        damageClaims: Object.freeze(claims),
        loading: false,
      });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar los reclamos por daños.");
    }
  }
  async loadDamageClaim(claimId: string): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return;
    }
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const claim = await this.#waitSession(
        this.#port.getDamageClaim(
          claimId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({ currentDamageClaim: claim, loading: false });
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return;
      }
      this.#handleError(err, "No pudimos cargar el reclamo por daños.");
    }
  }
  async resolveDamageClaim(
    claimId: string,
    input: AdminDamageClaimResolutionInput,
  ): Promise<boolean> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return false;
    }
    const _sessionScope = this.#captureSession();
    if (!isValidAdminReason(input.resolution_notes)) {
      this.#setSnapshot({
        error:
          "Las notas de resolución de daños deben contener al menos 5 caracteres válidos.",
      });
      return await this.#waitSession(false, _sessionScope);
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const idempotencyKey = generateIdempotencyKey();
      const updated = await this.#waitSession(
        this.#port.resolveDamageClaim(
          claimId,
          idempotencyKey,
          input,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        currentDamageClaim: updated,
        actionLoading: false,
      });
      await this.#waitSession(this.loadDamageClaims(), _sessionScope);
      return await this.#waitSession(true, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return false;
      }
      this.#handleError(err, "No pudimos resolver el reclamo por daños.");
      return await this.#waitSession(false, _sessionScope);
    }
  }
}
