import { CHILE_REGIONS, getComunasByRegion } from "./chile-dpa";

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
      message: "Selecciona región y comuna, o deja ambos campos vacíos.",
    };
  }
  if (comuna) {
    const regionExists = CHILE_REGIONS.some(({ code }) => code === regionCode);
    const validComuna = getComunasByRegion(regionCode).some(
      ({ code }) => code === comuna && code !== "12202",
    );
    if (!regionExists || !validComuna) {
      return {
        valid: false,
        message: "Selecciona una comuna válida para la región elegida.",
      };
    }
  }
  return { valid: true };
}
