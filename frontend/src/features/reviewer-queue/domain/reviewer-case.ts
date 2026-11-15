export type ReviewerQueueState =
  "enviada" | "en_revision" | "requiere_antecedentes";

export interface ReviewerSummary {
  readonly id: string;
  readonly state: ReviewerQueueState;
  readonly stateLabel: string;
  readonly revision: number | string;
  readonly documentExpiresAtObserved?: string;
}

const stateLabels: Readonly<Record<ReviewerQueueState, string>> = {
  enviada: "Enviada",
  en_revision: "En revisión",
  requiere_antecedentes: "Antecedentes solicitados",
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isRevision(value: unknown): value is number | string {
  if (typeof value === "number")
    return Number.isSafeInteger(value) && value > 0;
  return typeof value === "string" && /^[1-9]\d*$/.test(value);
}

function isReviewerQueueState(value: unknown): value is ReviewerQueueState {
  return typeof value === "string" && Object.hasOwn(stateLabels, value);
}

function isCivilDate(value: unknown): value is string {
  if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}$/.test(value))
    return false;
  const date = new Date(`${value}T00:00:00Z`);
  return (
    Number.isFinite(date.getTime()) && date.toISOString().slice(0, 10) === value
  );
}

export function toReviewerSummary(payload: unknown): ReviewerSummary[] {
  if (!isRecord(payload) || !Array.isArray(payload.items)) {
    throw new TypeError("Invalid reviewer queue response");
  }

  return payload.items.flatMap((item): ReviewerSummary[] => {
    if (!isRecord(item) || item.purpose !== "identity") return [];
    if (
      !isCanonicalReviewerCaseId(item.id) ||
      !isReviewerQueueState(item.state) ||
      !isRevision(item.revision)
    ) {
      return [];
    }

    return [
      {
        id: item.id,
        state: item.state,
        stateLabel: stateLabels[item.state],
        revision: item.revision,
        ...(isCivilDate(item.document_expires_at_observed)
          ? { documentExpiresAtObserved: item.document_expires_at_observed }
          : {}),
      },
    ];
  });
}

export type ReviewerIdentityEvidenceRole =
  "identity_front" | "identity_back" | "identity_selfie";

export type ReviewerIdentityFileState =
  "pendiente" | "limpio" | "bloqueado" | "error";

export interface ReviewerIdentityFile {
  readonly id: string;
  readonly evidenceRole: ReviewerIdentityEvidenceRole;
  readonly state: ReviewerIdentityFileState;
  readonly size: number;
  readonly mime: "image/jpeg" | "image/png";
  readonly pages: number;
}

export function reviewerIdentityEvidenceResourceKey(
  caseId: string,
  file: ReviewerIdentityFile,
): string {
  return JSON.stringify([
    caseId,
    file.id,
    file.evidenceRole,
    file.state,
    file.size,
    file.mime,
    file.pages,
  ]);
}

export interface ReviewerIdentityDetail {
  readonly id: string;
  readonly state: string;
  readonly stateLabel: string;
  readonly revision: number | string;
  readonly documentFormatDeclaredLabel?: string;
  readonly documentFormatObservedLabel?: string;
  readonly documentExpiresAtObserved?: string;
  readonly declaration: {
    readonly firstName: string;
    readonly lastName: string;
    readonly rut: string;
    readonly dateOfBirth: string;
  };
  readonly files: readonly ReviewerIdentityFile[];
}

const caseStateLabels: Readonly<Record<string, string>> = {
  borrador: "Borrador",
  enviada: "Enviada",
  en_revision: "En revisión",
  requiere_antecedentes: "Antecedentes solicitados",
  aprobada: "Aprobada",
  rechazada: "Rechazada",
  vencida: "Vencida",
  revocada: "Revocada",
};
const fileStates = new Set<ReviewerIdentityFileState>([
  "pendiente",
  "limpio",
  "bloqueado",
  "error",
]);
const mimeTypes = new Set<ReviewerIdentityFile["mime"]>([
  "image/jpeg",
  "image/png",
]);
const evidenceRoles = new Set<ReviewerIdentityEvidenceRole>([
  "identity_front",
  "identity_back",
  "identity_selfie",
]);
const formatLabels: Readonly<Record<string, string>> = {
  legacy: "Antiguo",
  new: "Nuevo",
  unsure: "No determinado",
  undetermined: "Indeterminado",
};
const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

export function isCanonicalReviewerCaseId(value: unknown): value is string {
  return typeof value === "string" && uuidPattern.test(value);
}

export function isCanonicalReviewerFileId(value: unknown): value is string {
  return typeof value === "string" && uuidPattern.test(value);
}

export function toReviewerIdentityDetail(
  payload: unknown,
  expectedId: string,
): ReviewerIdentityDetail {
  if (!isRecord(payload) || !isCanonicalReviewerCaseId(expectedId)) {
    throw new TypeError("Invalid reviewer identity detail");
  }
  if (
    payload.id !== expectedId ||
    payload.purpose !== "identity" ||
    typeof payload.state !== "string" ||
    !Object.hasOwn(caseStateLabels, payload.state) ||
    !isRevision(payload.revision) ||
    !isRecord(payload.identity_declaration) ||
    !Array.isArray(payload.files)
  ) {
    throw new TypeError("Invalid reviewer identity detail");
  }

  const declaration = payload.identity_declaration;
  if (
    typeof declaration.first_name !== "string" ||
    declaration.first_name.length === 0 ||
    typeof declaration.last_name !== "string" ||
    declaration.last_name.length === 0 ||
    typeof declaration.rut !== "string" ||
    declaration.rut.length === 0 ||
    !isCivilDate(declaration.date_of_birth)
  ) {
    throw new TypeError("Invalid reviewer identity declaration");
  }

  const declaredFormat = payload.document_format_declared;
  if (
    declaredFormat !== undefined &&
    (typeof declaredFormat !== "string" ||
      !["legacy", "new", "unsure"].includes(declaredFormat))
  ) {
    throw new TypeError("Invalid declared document format");
  }
  const observedFormat = payload.document_format_observed;
  if (
    observedFormat !== undefined &&
    (typeof observedFormat !== "string" ||
      !["legacy", "new", "undetermined"].includes(observedFormat))
  ) {
    throw new TypeError("Invalid observed document format");
  }
  const observedExpiry = payload.document_expires_at_observed;
  if (observedExpiry !== undefined && !isCivilDate(observedExpiry)) {
    throw new TypeError("Invalid observed document expiry");
  }

  const files = payload.files.map((value): ReviewerIdentityFile => {
    if (
      !isRecord(value) ||
      typeof value.id !== "string" ||
      !uuidPattern.test(value.id) ||
      typeof value.state !== "string" ||
      !fileStates.has(value.state as ReviewerIdentityFileState) ||
      typeof value.size !== "number" ||
      !Number.isSafeInteger(value.size) ||
      value.size < 1 ||
      value.size > 10 * 1024 * 1024 ||
      typeof value.mime !== "string" ||
      !mimeTypes.has(value.mime as ReviewerIdentityFile["mime"]) ||
      typeof value.pages !== "number" ||
      !Number.isInteger(value.pages) ||
      value.pages < 1 ||
      value.pages > 20
    ) {
      throw new TypeError("Invalid reviewer identity file");
    }
    const role = value.evidence_role;
    if (
      typeof role !== "string" ||
      !evidenceRoles.has(role as ReviewerIdentityEvidenceRole)
    ) {
      throw new TypeError("Invalid reviewer identity evidence role");
    }
    return {
      id: value.id,
      state: value.state as ReviewerIdentityFileState,
      evidenceRole: role as ReviewerIdentityEvidenceRole,
      size: value.size,
      mime: value.mime as ReviewerIdentityFile["mime"],
      pages: value.pages,
    };
  });
  const roles = new Set(files.map((file) => file.evidenceRole));
  if (
    files.length !== evidenceRoles.size ||
    roles.size !== evidenceRoles.size ||
    ![...evidenceRoles].every((role) => roles.has(role))
  ) {
    throw new TypeError("Invalid reviewer identity evidence roles");
  }

  return {
    id: payload.id,
    state: payload.state,
    stateLabel: caseStateLabels[payload.state],
    revision: payload.revision,
    ...(typeof declaredFormat === "string"
      ? { documentFormatDeclaredLabel: formatLabels[declaredFormat] }
      : {}),
    ...(typeof observedFormat === "string"
      ? { documentFormatObservedLabel: formatLabels[observedFormat] }
      : {}),
    ...(typeof observedExpiry === "string"
      ? { documentExpiresAtObserved: observedExpiry }
      : {}),
    declaration: {
      firstName: declaration.first_name,
      lastName: declaration.last_name,
      rut: declaration.rut,
      dateOfBirth: declaration.date_of_birth,
    },
    files,
  };
}

export type IdentityReviewDecision = "approve" | "reject" | "needs_evidence";
export type IdentityReviewDocumentFormat = "legacy" | "new" | "undetermined";
export type IdentityReviewChecklistField =
  | "names_match"
  | "rut_matches"
  | "birth_date_and_majority_match"
  | "portrait_matches_selfie"
  | "evidence_legible"
  | "document_unexpired"
  | "issue_date_and_format_verified";

export type IdentityReviewChecklist = Readonly<
  Record<IdentityReviewChecklistField, boolean | null>
>;

export interface IdentityReviewDraft {
  readonly decision: IdentityReviewDecision;
  readonly reasonCode: string;
  readonly documentFormatObserved: IdentityReviewDocumentFormat | "";
  readonly documentExpiresAtObserved: string;
  readonly checklist: IdentityReviewChecklist;
}

export interface IdentityReviewDecisionRequest {
  readonly capability: "review_identity";
  readonly decision: IdentityReviewDecision;
  readonly expected_revision: number;
  readonly reason_code?: string;
  readonly document_format_observed: IdentityReviewDocumentFormat;
  readonly document_expires_at_observed?: string;
  readonly identity_review_checklist: Readonly<
    Record<IdentityReviewChecklistField, boolean>
  >;
}

const identityReviewChecklistFields: readonly IdentityReviewChecklistField[] = [
  "names_match",
  "rut_matches",
  "birth_date_and_majority_match",
  "portrait_matches_selfie",
  "evidence_legible",
  "document_unexpired",
  "issue_date_and_format_verified",
];

function isIdentityCivilDate(value: unknown): value is string {
  if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}$/.test(value))
    return false;
  const parsed = new Date(`${value}T00:00:00Z`);
  return (
    Number.isFinite(parsed.getTime()) &&
    parsed.toISOString().slice(0, 10) === value
  );
}

function isIdentityReviewDocumentFormat(
  value: unknown,
): value is IdentityReviewDocumentFormat {
  return value === "legacy" || value === "new" || value === "undetermined";
}

function santiagoCivilDate(now: Date): string | null {
  if (!Number.isFinite(now.getTime())) return null;
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Santiago",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(now);
  const part = (type: string) =>
    parts.find((item) => item.type === type)?.value;
  const year = part("year");
  const month = part("month");
  const day = part("day");
  return year && month && day ? `${year}-${month}-${day}` : null;
}

function safeRevision(value: number | string): number | null {
  if (typeof value === "string" && !/^[1-9]\d*$/.test(value)) return null;
  const revision = typeof value === "number" ? value : Number(value);
  return Number.isSafeInteger(revision) && revision > 0 ? revision : null;
}

export function buildReviewerIdentityDecisionRequest(
  draft: IdentityReviewDraft,
  revision: number | string,
  now = new Date(),
): IdentityReviewDecisionRequest | null {
  if (
    draft.decision !== "approve" &&
    draft.decision !== "reject" &&
    draft.decision !== "needs_evidence"
  ) {
    return null;
  }
  if (
    !draft.checklist ||
    typeof draft.checklist !== "object" ||
    Array.isArray(draft.checklist)
  ) {
    return null;
  }
  if (!isIdentityReviewDocumentFormat(draft.documentFormatObserved)) {
    return null;
  }
  const observedFormat = draft.documentFormatObserved;
  const expectedRevision = safeRevision(revision);
  const today = santiagoCivilDate(now);
  const dateProvided = draft.documentExpiresAtObserved !== "";
  const checklistComplete = identityReviewChecklistFields.every(
    (field) => typeof draft.checklist[field] === "boolean",
  );
  const checklistPassed = identityReviewChecklistFields.every(
    (field) => draft.checklist[field] === true,
  );
  const validReasons: readonly string[] =
    draft.decision === "reject"
      ? [
          "documento_ilegible",
          "documento_invalido",
          "documento_vencido",
          "cambio_titularidad",
        ]
      : ["antecedentes_incompletos"];
  if (
    expectedRevision === null ||
    today === null ||
    !checklistComplete ||
    (dateProvided && !isIdentityCivilDate(draft.documentExpiresAtObserved)) ||
    (draft.decision === "approve" &&
      (draft.reasonCode !== "" ||
        observedFormat === "undetermined" ||
        !dateProvided ||
        draft.documentExpiresAtObserved < today ||
        !checklistPassed)) ||
    (draft.decision !== "approve" &&
      !validReasons.includes(draft.reasonCode)) ||
    (draft.decision === "reject" &&
      draft.reasonCode === "documento_vencido" &&
      (!dateProvided || draft.documentExpiresAtObserved >= today))
  ) {
    return null;
  }

  return {
    capability: "review_identity",
    decision: draft.decision,
    expected_revision: expectedRevision,
    ...(draft.reasonCode ? { reason_code: draft.reasonCode } : {}),
    document_format_observed: observedFormat,
    ...(dateProvided
      ? { document_expires_at_observed: draft.documentExpiresAtObserved }
      : {}),
    identity_review_checklist: Object.fromEntries(
      identityReviewChecklistFields.map((field) => [
        field,
        draft.checklist[field],
      ]),
    ) as Record<IdentityReviewChecklistField, boolean>,
  };
}

export interface ReviewerIdentityDecisionResult {
  readonly id: string;
  readonly state: "aprobada" | "rechazada" | "requiere_antecedentes";
  readonly revision: number;
}

const resultStateByDecision: Readonly<
  Record<IdentityReviewDecision, ReviewerIdentityDecisionResult["state"]>
> = {
  approve: "aprobada",
  reject: "rechazada",
  needs_evidence: "requiere_antecedentes",
};

export function toReviewerDecisionResult(
  payload: unknown,
  expectedId: string,
  expectedRevision: number,
  decision: IdentityReviewDecision,
): ReviewerIdentityDecisionResult {
  const nextRevision = expectedRevision + 1;
  if (
    !isRecord(payload) ||
    !isCanonicalReviewerCaseId(expectedId) ||
    !Number.isSafeInteger(expectedRevision) ||
    expectedRevision < 1 ||
    !Number.isSafeInteger(nextRevision) ||
    !Object.hasOwn(resultStateByDecision, decision) ||
    payload.id !== expectedId ||
    payload.purpose !== "identity" ||
    payload.state !== resultStateByDecision[decision] ||
    safeRevision(payload.revision as number | string) !== nextRevision
  ) {
    throw new TypeError("Invalid reviewer decision response");
  }

  return {
    id: expectedId,
    state: resultStateByDecision[decision],
    revision: nextRevision,
  };
}
