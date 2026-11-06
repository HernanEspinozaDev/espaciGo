export type WizardLocationValidation =
  { valid: true } | { valid: false; message: string };

export function validateWizardLocation(
  regionCode: string,
  comunaCode: string,
): WizardLocationValidation {
  const comuna = comunaCode.trim();
  if ((regionCode === "") !== (comuna === "")) {
    return {
      valid: false,
      message:
        "Ingresa juntos el código de comuna y el código de región, o deja ambos vacíos.",
    };
  }
  if (
    comuna &&
    (!/^\d{2}$/.test(regionCode) ||
      !/^\d{5}$/.test(comuna) ||
      !comuna.startsWith(regionCode))
  ) {
    return {
      valid: false,
      message:
        "Usa un código de comuna de 5 dígitos que comience con el código de región seleccionado.",
    };
  }
  return { valid: true };
}
