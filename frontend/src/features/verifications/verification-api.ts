import "client-only";

export type DiditIntentPayload = {
  first_name: string;
  last_name: string;
  date_of_birth: string;
  rut: string;
  consent: boolean;
  consent_version: string;
};

export type DiditIntentResponse = {
  attempt_id: string;
  session_id: string;
  session_url: string;
  status: string;
};

export type DiditStatusResponse = {
  id: string;
  account_id: string;
  status: string;
  decision_status?: string;
  badge_status: "unverified" | "pending" | "in_review" | "verified" | "rejected";
  verified_at?: string;
  rejection_reason?: string;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/backend${path}`, {
    ...init,
    credentials: "same-origin",
    cache: "no-store",
    headers: { "content-type": "application/json", ...init?.headers },
  });
  if (!response.ok) throw new Error("No fue posible consultar el estado de identidad.");
  return (await response.json()) as T;
}

export async function createDiditIntent(
  payload: DiditIntentPayload,
): Promise<DiditIntentResponse> {
  if (!payload.consent) throw new Error("Se requiere consentimiento explícito.");
  return request<DiditIntentResponse>("/verifications/didit/intent", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function getDiditIntent(id: string): Promise<DiditStatusResponse> {
  if (!/^[0-9a-f-]{36}$/i.test(id)) throw new Error("Intento de verificación no válido.");
  return request<DiditStatusResponse>(`/verifications/didit/intent/${encodeURIComponent(id)}`);
}
