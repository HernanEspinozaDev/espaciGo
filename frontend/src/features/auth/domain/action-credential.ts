const canonicalUuid =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

declare const credentialBrand: unique symbol;
export interface ActionCredential {
  readonly tokenId: string;
  readonly token: string;
  readonly [credentialBrand]: true;
}

export type CredentialParseResult =
  | { readonly ok: true; readonly credential: ActionCredential }
  | { readonly ok: false };

const invalid: CredentialParseResult = Object.freeze({ ok: false });

export function createActionCredential(
  tokenId: string,
  token: string,
): CredentialParseResult {
  if (
    !canonicalUuid.test(tokenId) ||
    token.length < 1 ||
    token.length > 2048 ||
    /[\u0000-\u001f\u007f]/.test(tokenId) ||
    /[\u0000-\u001f\u007f]/.test(token)
  ) {
    return invalid;
  }
  return {
    ok: true,
    credential: Object.freeze({ tokenId, token }) as ActionCredential,
  };
}

export function parseActionFragment(hash: string): CredentialParseResult {
  if (!hash.startsWith("#") || hash.length > 8192) return invalid;
  const fields = new URLSearchParams(hash.slice(1));
  const entries = [...fields.entries()];
  if (
    entries.length !== 2 ||
    fields.getAll("token").length !== 1 ||
    fields.getAll("token_id").length !== 1 ||
    entries.some(([key]) => key !== "token" && key !== "token_id")
  ) {
    return invalid;
  }
  return createActionCredential(
    fields.get("token_id") ?? "",
    fields.get("token") ?? "",
  );
}
