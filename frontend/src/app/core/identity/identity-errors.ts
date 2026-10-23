export interface IdentityFieldError {
  field: string;
  code: string;
}

export class IdentityApiError extends Error {
  constructor(
    readonly status: number,
    readonly code?: string,
    readonly details: readonly IdentityFieldError[] = [],
  ) {
    super('La operación de identidad no fue aceptada por Go.');
  }
}

export function identityErrorMessage(error: unknown): string {
  if (error instanceof IdentityApiError) {
    const messages: Record<string, string> = {
      invalid_token: 'El enlace no es válido, ha vencido o ya se utilizó. Solicita uno nuevo.',
      email_unverified: 'Verifica tu correo para iniciar sesión. Puedes solicitar un nuevo enlace.',
      password_unchanged: 'Elige una contraseña diferente a la actual.',
      password_recently_used: 'Esa contraseña ya se utilizó recientemente. Elige una diferente.',
      password_confirmation_mismatch: 'Las contraseñas no coinciden.',
    };
    return (error.code ? messages[error.code] : undefined) ?? messageForStatus(error.status);
  }
  if (
    typeof error !== 'object' ||
    error === null ||
    !('status' in error) ||
    typeof error.status !== 'number'
  ) {
    return 'No pudimos completar la solicitud. Revisa tu conexión e inténtalo otra vez.';
  }

  return messageForStatus(error.status);
}

function messageForStatus(status: number): string {
  switch (status) {
    case 0:
      return 'No hay conexión con EspaciGo. Revisa tu conexión e inténtalo otra vez.';
    case 401:
      return 'El correo o la contraseña no coinciden, o la sesión ya no está vigente.';
    case 403:
      return 'La cuenta todavía no está verificada o el servicio no permite esta acción.';
    case 409:
      return 'Ya existe una cuenta con ese correo. Inicia sesión o recupera el acceso.';
    case 422:
      return 'Los datos no cumplen las reglas de EspaciGo. Revísalos e inténtalo otra vez.';
    case 429:
      return 'Se alcanzó el límite de intentos. Espera un momento antes de volver a probar.';
    case 503:
      return 'El servicio de correo no está disponible. Puedes reintentar el envío más tarde.';
    default:
      return 'El servicio no pudo completar la solicitud. Inténtalo otra vez.';
  }
}

export function identityFieldMessages(error: unknown): Record<string, string> {
  if (!(error instanceof IdentityApiError)) return {};
  const messages: Record<string, string> = {
    'email:invalid_email': 'Escribe un correo válido.',
    'email:already_registered':
      'Ya existe una cuenta con ese correo. Inicia sesión o recupera el acceso.',
    'password:required': 'Escribe tu contraseña.',
    'password:password_policy': 'Usa al menos 8 caracteres, una mayúscula, un número y un símbolo.',
    'terms_version_ids:consent_invalid': 'Vuelve a cargar y aceptar los documentos vigentes.',
  };
  return Object.fromEntries(
    error.details.flatMap(({ field, code }) => {
      const message = messages[`${field}:${code}`];
      return message ? [[field, message]] : [];
    }),
  );
}
