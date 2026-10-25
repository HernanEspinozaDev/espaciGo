import type {
  AdminCapability,
  AdminRole,
  AdminSection,
  GlobalAdminAccountState,
  GlobalAdminPublicationState,
} from "./types";

const canonicalUuid =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

const isoDatePattern =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/;

export function isValidUUID(id: unknown): id is string {
  return typeof id === "string" && canonicalUuid.test(id);
}

export function generateIdempotencyKey(): string {
  if (
    typeof crypto !== "undefined" &&
    typeof crypto.randomUUID === "function"
  ) {
    return crypto.randomUUID();
  }
  // Fallback RFC4122 v4 UUID generator
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export function isValidAdminReason(reason: unknown): reason is string {
  if (typeof reason !== "string") return false;
  const trimmed = reason.trim();
  if (trimmed.length < 5 || trimmed.length > 500) return false;
  if (/[\u0000-\u001f\u007f]/.test(reason)) return false;
  return true;
}

export function hasRequiredRole(
  userRoles: readonly string[] | undefined | null,
  requiredRole: AdminRole,
): boolean {
  if (!userRoles || !Array.isArray(userRoles)) return false;
  return userRoles.includes(requiredRole);
}

export function isAdminOrGlobalAdmin(
  userRoles: readonly string[] | undefined | null,
): boolean {
  if (!userRoles || !Array.isArray(userRoles)) return false;
  return (
    userRoles.includes("administrador") ||
    userRoles.includes("administrador_global")
  );
}

export function hasCapability(
  userCapabilities: readonly string[] | undefined | null,
  capability: AdminCapability,
): boolean {
  if (!userCapabilities || !Array.isArray(userCapabilities)) return false;
  return userCapabilities.includes(capability);
}

const SECTION_CAPABILITIES: Readonly<
  Record<AdminSection, readonly AdminCapability[]>
> = {
  overview: [],
  accounts: [
    "accounts.list",
    "accounts.read",
    "accounts.update",
    "accounts.block",
    "accounts.roles",
  ],
  publications: [
    "publications.list",
    "publications.read",
    "publications.update",
    "publications.moderate",
    "publications.retire",
  ],
  kpi: ["kpi.read"],
  audit: ["audit.read", "audit.export"],
  verifications: ["legacy.administration"],
  disputes: ["legacy.administration"],
  review_reports: ["legacy.administration"],
  notices: ["legacy.administration"],
  spaces: ["publications.list", "legacy.administration"],
  reports: ["kpi.read", "legacy.administration"],
  reservations: ["legacy.administration"],
  damage_claims: ["legacy.administration"],
};

export function canAccessSection(
  capabilities: readonly string[] | undefined | null,
  section: AdminSection,
  roles?: readonly string[] | undefined | null,
): boolean {
  // If user is global admin or local admin with appropriate role
  if (roles && roles.includes("administrador_global")) {
    // Global admin can access global sections or sections covered by capabilities
    if (section === "overview") return true;
  }
  if (roles && roles.includes("administrador")) {
    // Local admin can access local operations
    if (
      section === "verifications" ||
      section === "disputes" ||
      section === "review_reports" ||
      section === "notices" ||
      section === "spaces" ||
      section === "reports" ||
      section === "reservations" ||
      section === "damage_claims"
    ) {
      return true;
    }
  }

  const required = SECTION_CAPABILITIES[section];
  if (!required || required.length === 0) {
    return isAdminOrGlobalAdmin(roles);
  }

  if (!capabilities || !Array.isArray(capabilities)) return false;
  return required.some((cap) => capabilities.includes(cap));
}

export function isValidExpectedUpdatedAt(value: unknown): value is string {
  if (typeof value !== "string") return false;
  if (!isoDatePattern.test(value)) return false;
  return !Number.isNaN(Date.parse(value));
}

export function isConcurrencyConflict(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const record = error as Record<string, unknown>;
  if (record.status === 409) return true;
  if (
    typeof record.code === "string" &&
    record.code.toLowerCase().includes("conflict")
  ) {
    return true;
  }
  return false;
}

export function isRecentAuthRequired(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const record = error as Record<string, unknown>;
  if (record.status === 428) return true;
  if (
    typeof record.code === "string" &&
    (record.code.toLowerCase().includes("precondition_required") ||
      record.code.toLowerCase().includes("recent_auth"))
  ) {
    return true;
  }
  return false;
}

export function isForbiddenError(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const record = error as Record<string, unknown>;
  if (record.status === 403) return true;
  if (
    typeof record.code === "string" &&
    record.code.toLowerCase().includes("forbidden")
  ) {
    return true;
  }
  return false;
}

export function isValidPublicationState(
  state: unknown,
): state is GlobalAdminPublicationState {
  return (
    state === "borrador" ||
    state === "activa" ||
    state === "oculta" ||
    state === "eliminada"
  );
}

export function isValidAccountState(
  state: unknown,
): state is GlobalAdminAccountState {
  return (
    state === "correo_pendiente" ||
    state === "activo" ||
    state === "bloqueado" ||
    state === "baja_solicitada" ||
    state === "desidentificado"
  );
}

export function isValidVerificationDecision(
  decision: unknown,
): decision is "aprobar" | "rechazar" {
  return decision === "aprobar" || decision === "rechazar";
}

export function isValidDamageClaimDecision(
  decision: unknown,
): decision is "aceptado" | "rechazado" | "parcial" {
  return (
    decision === "aceptado" ||
    decision === "rechazado" ||
    decision === "parcial"
  );
}

export function isValidDisputeCloseReason(
  reasonCode: unknown,
): reasonCode is "ensayo_finalizado" | "registro_erroneo" | "duplicada" {
  return (
    reasonCode === "ensayo_finalizado" ||
    reasonCode === "registro_erroneo" ||
    reasonCode === "duplicada"
  );
}

export function isValidReviewReportAction(
  action: unknown,
): action is "dismiss" | "hide" {
  return action === "dismiss" || action === "hide";
}

export function isValidReviewReportReasonCode(
  reasonCode: unknown,
): reasonCode is
  "sin_infraccion" | "contenido_inadecuado" | "duplicado" | "error_registro" {
  return (
    reasonCode === "sin_infraccion" ||
    reasonCode === "contenido_inadecuado" ||
    reasonCode === "duplicado" ||
    reasonCode === "error_registro"
  );
}

export function isValidNoticeReopenReasonCode(
  reasonCode: unknown,
): reasonCode is "smtp_restaurado" | "reintento_operativo" {
  return (
    reasonCode === "smtp_restaurado" || reasonCode === "reintento_operativo"
  );
}
