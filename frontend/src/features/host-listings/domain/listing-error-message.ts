import { ApiError } from "@/lib/api/errors";

const GENERIC_ERROR =
  "No fue posible completar la operación. Intenta nuevamente.";

export function listingErrorMessage(error: unknown): string {
  if (!(error instanceof ApiError)) return GENERIC_ERROR;

  switch (error.status) {
    case 401:
      return "Tu sesión venció. Inicia sesión nuevamente para continuar.";
    case 403:
      return "No tienes permiso para realizar esta operación.";
    case 412:
      return "Este espacio cambió. Actualiza la información antes de volver a guardar.";
    case 422:
      return "Revisa la información ingresada e intenta nuevamente.";
    case 428:
      return "Vuelve a verificar tu identidad para continuar.";
    default:
      return GENERIC_ERROR;
  }
}
