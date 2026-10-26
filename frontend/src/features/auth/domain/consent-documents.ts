import type { components, Readable } from "@/lib/api/generated/openapi";

export type ConsentDocument = Readable<
  components["schemas"]["TermsResponse"]
>["items"][number];
export type ConsentSet = Readonly<{
  terms: ConsentDocument;
  privacy: ConsentDocument;
}>;

const canonicalUuid =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const sha256 = /^[0-9a-f]{64}$/;
const utcTimestamp =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/;

function complete(document: ConsentDocument): boolean {
  const full = document.document;
  return Boolean(
    canonicalUuid.test(document.id) &&
    typeof document.code === "string" &&
    document.code.trim() &&
    sha256.test(document.sha256) &&
    typeof document.published_at === "string" &&
    utcTimestamp.test(document.published_at) &&
    !Number.isNaN(Date.parse(document.published_at)) &&
    document.synthetic === true &&
    full &&
    full.id === document.id &&
    full.code === document.code &&
    full.type === document.type &&
    typeof full.title === "string" &&
    full.title.trim() &&
    typeof full.scope === "string" &&
    full.scope.trim() &&
    typeof full.status === "string" &&
    full.status.trim() &&
    Array.isArray(full.sections) &&
    full.sections.length > 0 &&
    full.sections.every(
      (section) =>
        typeof section.heading === "string" &&
        section.heading.trim().length > 0 &&
        typeof section.body === "string" &&
        section.body.trim().length > 0,
    ),
  );
}

export function validateConsentDocuments(value: unknown): ConsentSet | null {
  if (
    !value ||
    typeof value !== "object" ||
    !Array.isArray((value as { items?: unknown }).items)
  )
    return null;
  const items = (value as { items: ConsentDocument[] }).items;
  const terms = items.filter((item) => item?.type === "terminos");
  const privacy = items.filter((item) => item?.type === "privacidad");
  if (
    items.length !== 2 ||
    terms.length !== 1 ||
    privacy.length !== 1 ||
    terms[0]?.id === privacy[0]?.id ||
    !complete(terms[0]!) ||
    !complete(privacy[0]!)
  ) {
    return null;
  }
  return Object.freeze({ terms: terms[0]!, privacy: privacy[0]! });
}
