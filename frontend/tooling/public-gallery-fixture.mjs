import http from "node:http";

const port = Number(process.env.PUBLIC_GALLERY_FIXTURE_PORT ?? 18089);
const spaceId = "00000000-0000-4000-8000-000000000007";
const imageIds = [
  "00000000-0000-4000-8000-000000000071",
  "00000000-0000-4000-8000-000000000072",
];
const webp = Buffer.from(
  "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA",
  "base64",
);
const listing = {
  space_id: spaceId,
  category_code: "oficina",
  category_name: "Oficina",
  title: "Espacio sintético",
  description: "Ficha sintética para ensayo público de galería.",
  area_m2: 12,
  rate_unit: "hora",
  base_price_clp: 12000,
  currency: "CLP",
  region_code: null,
  region_name: null,
  comuna_code: null,
  comuna_name: null,
  estimated_total_clp: null,
  usage_rules: "Uso sintético de prueba.",
  cover_image_id: imageIds[0],
  features: [],
  images: imageIds.map((image_id, index) => ({
    image_id,
    is_cover: index === 0,
    source: "media",
    object_key: "PRIVATE_OBJECT_KEY_MARKER",
  })),
  verification: { identity_verified: false, space_verified: false },
  // Unexpected upstream fields must not enter the rendered public document.
  owner_id: "PRIVATE_OWNER_ID_MARKER",
  exact_address: "PRIVATE_ADDRESS_MARKER",
};

http
  .createServer((request, response) => {
    const pathname = new URL(request.url, "http://127.0.0.1").pathname;
    if (pathname === "/healthz") {
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ status: "ok" }));
      return;
    }
    if (
      request.method === "GET" &&
      pathname === `/api/v1/public/spaces/${spaceId}`
    ) {
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ data: listing }));
      return;
    }
    if (request.method === "GET" && pathname === "/api/v1/public/spaces") {
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ items: [listing] }));
      return;
    }
    if (
      request.method === "GET" &&
      (pathname === "/api/v1/public/space-categories" ||
        pathname === "/api/v1/public/locations")
    ) {
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ items: [] }));
      return;
    }
    const media = pathname.match(
      /^\/api\/v1\/public\/spaces\/([^/]+)\/images\/([^/]+)\/variants\/(display|thumbnail)$/,
    );
    if (request.method === "GET" && media) {
      const [, requestedSpaceId, imageId, variant] = media;
      if (
        requestedSpaceId === spaceId &&
        imageIds.includes(imageId) &&
        ["display", "thumbnail"].includes(variant)
      ) {
        response.writeHead(200, {
          "content-type": "image/webp",
          "cache-control": "no-store",
        });
        response.end(webp);
        return;
      }
    }
    response.writeHead(404, { "content-type": "application/json" });
    response.end(JSON.stringify({ error: { code: "not_found" } }));
  })
  .listen(port, "127.0.0.1");
