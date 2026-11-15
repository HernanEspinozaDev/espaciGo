import "client-only";

export type IdentityDeclarationPayload = {
  first_name: string;
  last_name: string;
  date_of_birth: string;
  rut: string;
  id_country: "CHL";
  document_type: "ID";
  consent_given: boolean;
  consent_version: string;
};

export type IdentityDeclarationResponse = {
  id: string;
  revision: number;
  first_name: string;
  last_name: string;
  date_of_birth: string;
  rut: string;
  id_country: "CHL";
  document_type: "ID";
  consent_version: string;
  consent_given_at: string;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/backend${path}`, {
    ...init,
    credentials: "same-origin",
    cache: "no-store",
    headers: {
      ...(init?.body !== undefined
        ? { "content-type": "application/json" }
        : {}),
      ...init?.headers,
    },
  });
  if (!response.ok) {
    let code = "";
    try {
      const body = (await response.json()) as { code?: string };
      if (typeof body?.code === "string") code = body.code;
    } catch {
      // ignore non-JSON error body
    }
    if (response.status === 409 || code === "rut_already_claimed") {
      throw new Error(
        "No se pudo registrar este RUT. Revisa los datos ingresados o solicita ayuda.",
      );
    }
    if (
      path === "/verifications/identity-declaration" &&
      init?.method === "POST"
    ) {
      throw new Error(
        "No pudimos guardar tus datos de identidad. Inténtalo nuevamente.",
      );
    }

    throw new Error("No fue posible consultar el estado de identidad.");
  }
  return (await response.json()) as T;
}

export async function getIdentityDeclaration(): Promise<IdentityDeclarationResponse | null> {
  const response = await fetch(
    "/api/backend/verifications/identity-declaration",
    {
      credentials: "same-origin",
      cache: "no-store",
    },
  );
  if (response.status === 404) return null;
  if (!response.ok)
    throw new Error("No fue posible consultar la declaración de identidad.");
  return (await response.json()) as IdentityDeclarationResponse;
}

export function saveIdentityDeclaration(
  payload: IdentityDeclarationPayload,
): Promise<IdentityDeclarationResponse> {
  return request<IdentityDeclarationResponse>(
    "/verifications/identity-declaration",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
  );
}
