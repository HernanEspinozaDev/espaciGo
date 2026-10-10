export interface APIErrorEnvelope {
  error?: { code?: string; message?: string; request_id?: string };
}

export function apiErrorMessage(body: APIErrorEnvelope, status: number): string {
  const code = body.error?.code ?? "unknown";
  const message = body.error?.message ?? "Error de API";
  if (status === 403 && code === "restricted_account") {
    return `Acceso restringido: ${message} (HTTP 403).`;
  }
  return `${message} (HTTP ${status}, ${code})`;
}
