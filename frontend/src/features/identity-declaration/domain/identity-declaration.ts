const defaultIdentityDeclarationConsentVersion = "id-consent-20261108";
const configuredConsentVersion =
  process.env.NEXT_PUBLIC_IDENTITY_DECLARATION_CONSENT_VERSION?.trim();
if (
  configuredConsentVersion &&
  (configuredConsentVersion.length > 20 || /\s/.test(configuredConsentVersion))
) {
  throw new Error(
    "Identity declaration consent version configuration is invalid",
  );
}
export const identityDeclarationConsentVersion =
  configuredConsentVersion || defaultIdentityDeclarationConsentVersion;

export interface IdentityDeclaration {
  readonly id: string;
  readonly revision: number;
  readonly firstName: string;
  readonly lastName: string;
  readonly dateOfBirth: string;
  readonly rut: string;
  readonly consentVersion: string;
  readonly consentGivenAt: string;
}

export interface IdentityDeclarationInput {
  readonly firstName: string;
  readonly lastName: string;
  readonly dateOfBirth: string;
  readonly rut: string;
}

export interface IdentityDeclarationRequest {
  readonly first_name: string;
  readonly last_name: string;
  readonly date_of_birth: string;
  readonly rut: string;
  readonly id_country: "CHL";
  readonly document_type: "ID";
  readonly consent_given: true;
  readonly consent_version: string;
}

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
function validDate(value: unknown): value is string {
  if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}$/.test(value))
    return false;
  const [year, month, day] = value.split("-").map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  return (
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day
  );
}
function validateInput(input: IdentityDeclarationInput): void {
  for (const name of [input.firstName, input.lastName]) {
    if (typeof name !== "string" || !name.trim() || [...name].length > 150)
      throw new TypeError("Invalid identity declaration input");
  }
  if (
    !validDate(input.dateOfBirth) ||
    typeof input.rut !== "string" ||
    [...input.rut].length < 8 ||
    [...input.rut].length > 20
  )
    throw new TypeError("Invalid identity declaration input");
}

export function buildIdentityDeclarationRequest(
  input: IdentityDeclarationInput,
): IdentityDeclarationRequest {
  validateInput(input);
  return {
    first_name: input.firstName,
    last_name: input.lastName,
    date_of_birth: input.dateOfBirth,
    rut: input.rut,
    id_country: "CHL",
    document_type: "ID",
    consent_given: true,
    consent_version: identityDeclarationConsentVersion,
  };
}

export function parseIdentityDeclarationResponse(
  value: unknown,
): IdentityDeclaration {
  const keys = [
    "id",
    "revision",
    "first_name",
    "last_name",
    "date_of_birth",
    "rut",
    "id_country",
    "document_type",
    "consent_version",
    "consent_given_at",
  ];
  if (
    !record(value) ||
    Object.keys(value).length !== keys.length ||
    Object.keys(value).some((key) => !keys.includes(key))
  )
    throw new TypeError("Invalid identity declaration response");
  const uuid =
    typeof value.id === "string" &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
      value.id,
    );
  const revision =
    typeof value.revision === "number" &&
    Number.isInteger(value.revision) &&
    value.revision > 0;
  const names = [value.first_name, value.last_name].every(
    (v) => typeof v === "string" && !!v.trim() && [...v].length <= 150,
  );
  const rut =
    typeof value.rut === "string" &&
    [...value.rut].length >= 8 &&
    [...value.rut].length <= 20;
  const instant =
    typeof value.consent_given_at === "string" &&
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(
      value.consent_given_at,
    ) &&
    Number.isFinite(Date.parse(value.consent_given_at));
  if (
    !uuid ||
    !revision ||
    !names ||
    !validDate(value.date_of_birth) ||
    !rut ||
    value.id_country !== "CHL" ||
    value.document_type !== "ID" ||
    value.consent_version !== identityDeclarationConsentVersion ||
    !instant
  )
    throw new TypeError("Invalid identity declaration response");
  return {
    id: value.id as string,
    revision: value.revision as number,
    firstName: value.first_name as string,
    lastName: value.last_name as string,
    dateOfBirth: value.date_of_birth,
    rut: value.rut as string,
    consentVersion: value.consent_version,
    consentGivenAt: value.consent_given_at as string,
  };
}
