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

  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new Error("API_BACKEND_BASE_URL must use http or https");
  }

  if (url.username !== "" || url.password !== "" || url.search !== "" || url.hash !== "") {
    throw new Error("API_BACKEND_BASE_URL must not contain credentials, query, or hash");
  }

  if (url.pathname !== expectedPath) {
    if (url.pathname === "/" || url.pathname === "") {
      url.pathname = expectedPath;
    } else if (!url.pathname.endsWith(expectedPath)) {
      url.pathname = `${url.pathname.replace(/\/$/, "")}${expectedPath}`;
    }
  }

  return url;
}
