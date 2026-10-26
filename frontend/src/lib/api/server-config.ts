import "server-only";

const expectedPath = "/api/v1";

export function getApiBackendBaseUrl(): URL {
  const raw = process.env.API_BACKEND_BASE_URL || process.env.API_INTERNAL_BASE_URL;
  if (!raw) throw new Error("API_BACKEND_BASE_URL is not set");

  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error("API_BACKEND_BASE_URL is invalid");
  }

  if (
    url.protocol !== "http:" ||
    url.hostname !== "127.0.0.1" ||
    url.port !== "18082" ||
    url.pathname !== expectedPath ||
    url.username !== "" ||
    url.password !== "" ||
    url.search !== "" ||
    url.hash !== ""
  ) {
    throw new Error("API_BACKEND_BASE_URL is outside the local API allowlist");
  }

  return url;
}
