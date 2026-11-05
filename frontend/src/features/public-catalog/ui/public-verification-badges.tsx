import type { PublicVerification } from "../domain/verification";

export function PublicVerificationBadges({
  verification,
}: {
  verification: PublicVerification;
}) {
  if (!verification.identityVerified && !verification.spaceVerified)
    return null;
  return (
    <ul
      aria-label="Distintivos informativos de verificación"
      className="flex flex-wrap gap-2"
    >
      {verification.identityVerified ? (
        <li title="La verificación no acredita titularidad, legalidad ni constituye garantía o firma.">
          <span className="inline-flex rounded-full border border-border bg-secondary/60 px-2.5 py-1 text-xs font-medium text-foreground">
            Identidad verificada
          </span>
          <span className="sr-only">
            La verificación no acredita titularidad, legalidad ni constituye
            garantía o firma.
          </span>
        </li>
      ) : null}
      {verification.identityVerified && verification.spaceVerified ? (
        <li title="La revisión no acredita legalidad o titularidad ni constituye garantía o firma.">
          <span className="inline-flex rounded-full border border-border bg-secondary/60 px-2.5 py-1 text-xs font-medium text-foreground">
            Antecedentes del espacio revisados
          </span>
          <span className="sr-only">
            La revisión no acredita legalidad o titularidad ni constituye
            garantía o firma.
          </span>
        </li>
      ) : null}
    </ul>
  );
}
