export function isAllowedLocalResource(
  urlValue: string,
  baseUrlValue: string,
): boolean {
  try {
    const baseOrigin = new URL(baseUrlValue).origin;
    const resource = new URL(urlValue);
    return (
      resource.origin === baseOrigin &&
      (resource.protocol === "http:" ||
        resource.protocol === "https:" ||
        resource.protocol === "blob:")
    );
  } catch {
    return false;
  }
}
