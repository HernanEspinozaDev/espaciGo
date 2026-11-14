export function getRentalDemoBaseUrl(): URL | null {
  if (process.env.RENTAL_DEMO_ENABLED !== "1") return null;
  const raw = process.env.RENTAL_DEMO_BASE_URL;
  if (!raw) return null;
  try {
    const url = new URL(raw);
    if (
      url.protocol !== "http:" ||
      url.hostname !== "127.0.0.1" ||
      !url.port ||
      url.username ||
      url.password ||
      url.pathname !== "/" ||
      url.search ||
      url.hash
    )
      return null;
    return url;
  } catch {
    return null;
  }
}
