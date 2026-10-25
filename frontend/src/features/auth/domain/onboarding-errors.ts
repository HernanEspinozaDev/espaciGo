const safeFieldCodes = new Set([
  "invalid_email",
  "password_policy",
  "consent_invalid",
  "password_confirmation_mismatch",
  "password_unchanged",
  "password_recently_used",
]);

export function onboardingErrorCopy(code: string): string {
  switch (code) {
    case "invalid_token":
      return "El enlace no es válido o ya no está disponible. Solicita instrucciones nuevas si corresponde.";
    case "email_unverified":
    case "email_not_verified":
      return "Verifica tu correo antes de iniciar sesión.";
    case "login_blocked":
      return "No pudimos completar el acceso. Intenta nuevamente más tarde.";
    case "rate_limited":
    case "too_many_requests":
      return "No pudimos completar la solicitud ahora. Espera antes de volver a intentarlo.";
    case "consent_invalid":
      return "Los documentos cambiaron. Vuelve a revisarlos antes de continuar.";
    case "password_confirmation_mismatch":
      return "Las contraseñas no coinciden.";
    case "password_unchanged":
      return "Elige una contraseña distinta de la actual.";
    case "password_recently_used":
      return "Elige una contraseña que no hayas usado recientemente.";
    case "email_registered":
    case "email_already_registered":
    case "email:already_registered":
      return "Este correo ya tiene una cuenta. Puedes acceder o solicitar recuperar el acceso.";
    case "invalid_credentials":
    case "unauthenticated":
    case "unauthorized":
      return "El correo o la contraseña no son válidos.";
    case "terms_unavailable":
      return "No pudimos cargar los documentos necesarios para el registro.";
    default:
      return safeFieldCodes.has(code)
        ? "Revisa la información indicada y vuelve a intentarlo."
        : "No fue posible completar la operación.";
  }
}

export function reissueAcceptedCopy(): string {
  return "Si la cuenta está pendiente y corresponde, intentaremos enviar nuevas instrucciones. No podemos confirmar la existencia de la cuenta ni la entrega del correo.";
}

export function recoveryAcceptedCopy(): string {
  return "Si existe una cuenta elegible, recibirás instrucciones en el correo asociado. No podemos confirmar la existencia de la cuenta ni la entrega del mensaje.";
}
