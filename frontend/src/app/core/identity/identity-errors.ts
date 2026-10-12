import { HttpErrorResponse } from '@angular/common/http';

export class IdentityApiError extends Error {
  constructor(readonly status: number) {
    super('La operación de identidad no fue aceptada por Go.');
  }
}

export function identityErrorMessage(error: unknown): string {
  if (error instanceof IdentityApiError) return messageForStatus(error.status);
  if (!(error instanceof HttpErrorResponse)) {
    return 'No pudimos completar la solicitud. Revisa tu conexión e inténtalo otra vez.';
  }

  return messageForStatus(error.status);
}

function messageForStatus(status: number): string {
  switch (status) {
    case 0:
      return 'No hay conexión con EspaciGo. Comprueba que el servicio local esté disponible.';
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
      return 'El correo local no está disponible. Puedes reintentar el envío más tarde.';
    default:
      return 'El servicio no pudo completar la solicitud. Inténtalo otra vez.';
  }
}
