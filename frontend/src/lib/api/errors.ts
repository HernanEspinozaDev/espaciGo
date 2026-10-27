export interface ApiFieldError {
  field: string;
  code: string;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId: string | null;
  readonly retryAfter: string | null;
  readonly fields: readonly ApiFieldError[];

  constructor(input: {
    status: number;
    code: string;
    requestId: string | null;
    retryAfter: string | null;
    fields: readonly ApiFieldError[];
  }) {
    super(`API request failed (${input.status}: ${input.code})`);
    this.name = "ApiError";
    this.status = input.status;
    this.code = input.code;
    this.requestId = input.requestId;
    this.retryAfter = input.retryAfter;
    this.fields = input.fields;
  }
}

export class ApiProtocolError extends Error {
  constructor() {
    super("The API response did not match the OpenAPI contract.");
    this.name = "ApiProtocolError";
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function safeCode(value: unknown, fallback: string): string {
  return typeof value === "string" && /^[a-z0-9_.-]{1,96}$/i.test(value)
    ? value
    : fallback;
}

function safeRequestId(value: unknown): string | null {
  return typeof value === "string" && /^[A-Za-z0-9._-]{1,128}$/.test(value)
    ? value
    : null;
}

export function normalizeApiError(
  response: Response,
  payload: unknown,
): ApiError {
  const envelope =
    isRecord(payload) && isRecord(payload.error) ? payload.error : {};
  const rawDetails = Array.isArray(envelope.details) ? envelope.details : [];
  const fields = rawDetails.flatMap((item) => {
    if (!isRecord(item)) return [];
    if (typeof item.field !== "string" || typeof item.code !== "string") {
      return [];
    }
    return [
      {
        field: item.field.slice(0, 96),
        code: safeCode(item.code, "invalid_value"),
      },
    ];
  });

  return new ApiError({
    status: response.status,
    code: safeCode(envelope.code, `http_${response.status}`),
    requestId:
      safeRequestId(envelope.request_id) ??
      safeRequestId(response.headers.get("x-request-id")),
    retryAfter: response.headers.get("retry-after"),
    fields,
  });
}

export function authErrorMessage(error: unknown): string {
  if (!(error instanceof ApiError)) {
    return "No pudimos conectar con el servicio. Intenta nuevamente.";
  }

  switch (error.code) {
    case "unauthenticated":
    case "invalid_credentials":
    case "unauthorized":
      return "El correo o la contraseña no son válidos.";
    case "email_not_verified":
    case "email_unverified":
      return "Debes verificar tu correo antes de ingresar. Revisa tu bandeja de entrada y la carpeta de spam.";
    case "login_blocked":
      return "El acceso está bloqueado temporalmente. Intenta más tarde.";
    case "rate_limited":
    case "too_many_requests":
      return "No pudimos completar la solicitud ahora. Espera antes de volver a intentarlo.";
    case "invalid_token":
      return "El enlace no es válido o ya no está disponible. Solicita instrucciones nuevas si corresponde.";
    case "password_confirmation_mismatch":
      return "Las contraseñas no coinciden.";
    case "password_unchanged":
      return "Elige una contraseña distinta de la actual.";
    case "password_recently_used":
      return "Elige una contraseña que no hayas usado recientemente.";
    case "consent_invalid":
      return "Los documentos cambiaron. Vuelve a revisarlos antes de continuar.";
    case "restricted_mode":
      return "La cuenta tiene acceso restringido a operaciones permitidas por el servicio.";
    case "remote_logout_unconfirmed":
      return "La sesión local se cerró, pero no se pudo confirmar la revocación remota en el servicio.";
    case "service_unavailable":
    case "api_unavailable":
    case "api_not_configured":
      return "El servicio no está disponible en este momento.";
    default:
      return "No fue posible completar la operación.";
  }
}
