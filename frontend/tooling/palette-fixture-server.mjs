#!/usr/bin/env node
import http from "node:http";
import { fileURLToPath } from "node:url";

export const DEFAULT_PORT = 18088;
export const DEFAULT_HOST = "127.0.0.1";

// Synthetic OpenAPI data according to planning/openapi.yaml & frontend types
const categoriesData = {
  items: [
    { code: "inmuebles", name: "Inmuebles y Vivienda", order: 1 },
    { code: "oficina", name: "Oficina y Cowork", order: 2 },
    { code: "bodega", name: "Bodega y Almacenaje", order: 3 },
    { code: "estacionamiento", name: "Estacionamiento", order: 4 },
  ],
};

const locationsData = {
  items: [
    {
      region_code: "13",
      region_name: "Metropolitana de Santiago",
      comuna_code: "13101",
      comuna_name: "Santiago",
    },
    {
      region_code: "13",
      region_name: "Metropolitana de Santiago",
      comuna_code: "13123",
      comuna_name: "Providencia",
    },
    {
      region_code: "13",
      region_name: "Metropolitana de Santiago",
      comuna_code: "13114",
      comuna_name: "Las Condes",
    },
  ],
};

const filtersByCode = {
  inmuebles: {
    category_code: "inmuebles",
    items: [
      {
        code: "tipo_inmueble",
        label: "Tipo de inmueble",
        type: "enum",
        order: 1,
        options: ["departamento", "casa", "habitacion"],
        profile_version: 1,
      },
      {
        code: "dormitorios",
        label: "Dormitorios",
        type: "integer",
        order: 2,
        minimum: 1,
        maximum: 10,
        profile_version: 1,
      },
      {
        code: "banos",
        label: "Baños",
        type: "integer",
        order: 3,
        minimum: 1,
        maximum: 10,
        profile_version: 1,
      },
      {
        code: "amoblado",
        label: "Amoblado",
        type: "boolean",
        order: 4,
        profile_version: 1,
      },
    ],
  },
  oficina: {
    category_code: "oficina",
    items: [
      {
        code: "puestos_trabajo",
        label: "Puestos de trabajo",
        type: "integer",
        order: 1,
        minimum: 1,
        maximum: 100,
        profile_version: 1,
      },
      {
        code: "mobiliario_incluido",
        label: "Mobiliario incluido",
        type: "boolean",
        order: 2,
        profile_version: 1,
      },
    ],
  },
};

const termsData = {
  items: [
    {
      id: "550e8400-e29b-41d4-a716-446655440000",
      code: "terms-local-v1",
      type: "terminos",
      sha256: "a".repeat(64),
      published_at: "2026-10-25T12:00:00Z",
      synthetic: true,
      document: {
        id: "550e8400-e29b-41d4-a716-446655440000",
        code: "terms-local-v1",
        type: "terminos",
        title: "Términos y Condiciones Sintéticos",
        scope: "local_test",
        status: "vigente",
        sections: [
          {
            heading: "Uso de Ensayo",
            body: "Contenido sintético para pruebas locales y verificación visual.",
          },
        ],
      },
    },
    {
      id: "660e8400-e29b-41d4-a716-446655440000",
      code: "privacy-local-v1",
      type: "privacidad",
      sha256: "b".repeat(64),
      published_at: "2026-10-25T12:00:00Z",
      synthetic: true,
      document: {
        id: "660e8400-e29b-41d4-a716-446655440000",
        code: "privacy-local-v1",
        type: "privacidad",
        title: "Política de Privacidad Sintética",
        scope: "local_test",
        status: "vigente",
        sections: [
          {
            heading: "Privacidad de Ensayo",
            body: "Sin persistencia de datos reales en fixture sintético.",
          },
        ],
      },
    },
  ],
};

// Empty synthetic catalog (honestly recorded limitation for visual comparison)
const emptyCatalog = {
  items: [],
};

export function createFixtureServer() {
  return http.createServer((req, res) => {
    // Guard: Read-only fixture (GET and HEAD only)
    if (req.method !== "GET" && req.method !== "HEAD") {
      res.writeHead(405, {
        "Content-Type": "application/json; charset=utf-8",
        Allow: "GET, HEAD",
        "Cache-Control": "no-store",
      });
      res.end(
        JSON.stringify({
          error: {
            code: "method_not_allowed",
            message: "Fixture is read-only GET",
          },
        }),
      );
      return;
    }

    const host = req.headers.host || `${DEFAULT_HOST}:${DEFAULT_PORT}`;
    const parsedUrl = new URL(req.url, `http://${host}`);
    let pathname = parsedUrl.pathname;

    // Normalize: strip leading /api/v1 if present
    if (pathname.startsWith("/api/v1")) {
      pathname = pathname.slice("/api/v1".length);
    }

    const sendJson = (status, payload) => {
      res.writeHead(status, {
        "Content-Type": "application/json; charset=utf-8",
        "Cache-Control": "no-store",
      });
      if (req.method === "HEAD") {
        res.end();
      } else {
        res.end(JSON.stringify(payload));
      }
    };

    if (pathname === "/public/space-categories") {
      return sendJson(200, categoriesData);
    }

    if (
      pathname.startsWith("/public/space-categories/") &&
      pathname.endsWith("/filters")
    ) {
      const parts = pathname.split("/");
      const code = parts[parts.length - 2];
      const filters = filtersByCode[code] || { category_code: code, items: [] };
      return sendJson(200, filters);
    }

    if (pathname === "/public/locations") {
      return sendJson(200, locationsData);
    }

    if (pathname === "/public/spaces") {
      return sendJson(200, emptyCatalog);
    }

    if (pathname.startsWith("/public/spaces/")) {
      return sendJson(404, {
        error: {
          code: "space_not_found",
          message: "Space not found in synthetic fixture",
        },
      });
    }

    if (pathname === "/auth/terms") {
      return sendJson(200, termsData);
    }

    return sendJson(404, {
      error: {
        code: "not_found",
        message: `Route ${pathname} not mapped in fixture`,
      },
    });
  });
}

const isMain = Boolean(
  process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1],
);
if (isMain) {
  const port = process.env.PORT ? parseInt(process.env.PORT, 10) : DEFAULT_PORT;
  const server = createFixtureServer();
  server.listen(port, DEFAULT_HOST, () => {
    console.log(
      `[palette-fixture-server] Read-only HTTP fixture running on http://${DEFAULT_HOST}:${port}`,
    );
    console.log(
      `[palette-fixture-server] Catalog: empty synthetic (recorded limitation)`,
    );
  });
  const shutdown = () => {
    server.close(() => {
      process.exit(0);
    });
  };
  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);
}
