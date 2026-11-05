export type PublicVerification = {
  identityVerified: boolean;
  spaceVerified: boolean;
};

export function publicVerification(value: unknown): PublicVerification {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return { identityVerified: false, spaceVerified: false };
  }
  const flags = value as Record<string, unknown>;
  const identityVerified = flags.identity_verified === true;
  return {
    identityVerified,
    spaceVerified: identityVerified && flags.space_verified === true,
  };
}
