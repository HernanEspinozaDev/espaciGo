import http from "node:http";

const port = Number(process.env.PUBLIC_CALENDAR_FIXTURE_PORT ?? 18088);
const id = "00000000-0000-4000-8000-000000000001";
const missingId = "00000000-0000-4000-8000-000000000002";
const inactiveId = "00000000-0000-4000-8000-000000000003";
const unavailableSpaceId = "00000000-0000-4000-8000-000000000004";
const invalidTimeZoneId = "00000000-0000-4000-8000-000000000005";
const supportedAliasId = "00000000-0000-4000-8000-000000000006";

let unavailableDetailReads = 0;
let unavailableDetailStatus = null;
let unavailableDetailId = null;
const archivedIds = new Set();
let archivedDetailReads = 0;
let archivedDetailStatus = null;
let archivedDetailId = null;
let publicListReads = 0;
let lastPublicListQuery = null;
let lastPublicListItems = null;
const space = (spaceId, weekly_schedule) => ({
  space_id: spaceId,
  category_code: "oficina",
  category_name: "Oficina",
  title: "Espacio sintético",
  description: "Ficha sintética para ensayo público.",
  area_m2: 12,
  rate_unit: "hora",
  base_price_clp: 12000,
  currency: "CLP",
  estimated_total_clp: null,
  usage_rules: "Uso sintético de prueba.",
  features: [],
  images: [],
  verification: { identity_verified: false, space_verified: false },
  // Synthetic upstream-only fields; the public adapter must drop these.
  owner_id: "PRIVATE_OWNER_ID_MARKER",
  exact_address: "PRIVATE_ADDRESS_MARKER",
  owner_contact: "PRIVATE_CONTACT_MARKER",
  ...(weekly_schedule ? { weekly_schedule } : {}),
});
const active = {
  enabled: true,
  time_zone: "America/Santiago",
  days: [
    { weekday: 1, periods: [] },
    { weekday: 2, periods: [{ open: "14:15", close: "16:45" }] },
    { weekday: 3, periods: [] },
    { weekday: 4, periods: [] },
    { weekday: 5, periods: [] },
    { weekday: 6, periods: [] },
    { weekday: 7, periods: [] },
  ],
};
const inactive = { ...active, enabled: false };
const invalidTimeZone = { ...active, time_zone: "../UTC" };
const supportedAlias = { ...active, time_zone: "EST" };
http
  .createServer((req, res) => {
    const path = new URL(req.url, "http://127.0.0.1").pathname;
    const match = path.match(/\/public\/spaces\/([^/]+)$/);
    res.setHeader("content-type", "application/json");

    const archiveMatch = path.match(/^\/__fixture\/archive\/([^/]+)$/);
    if (archiveMatch && req.method === "POST" && archiveMatch[1] === id) {
      archivedIds.add(id);
      res.writeHead(204);
      res.end();
      return;
    }
    if (path === "/api/v1/public/spaces" && req.method === "GET") {
      const query = new URL(req.url, "http://127.0.0.1").searchParams.get("q");
      publicListReads++;
      lastPublicListQuery = query;
      const matchesQuery =
        !query ||
        "Espacio sintético"
          .normalize("NFC")
          .toLocaleLowerCase("es-CL")
          .includes(query.normalize("NFC").toLocaleLowerCase("es-CL"));
      const items =
        !archivedIds.has(id) && matchesQuery ? [space(id, active)] : [];
      lastPublicListItems = items.length;
      res.writeHead(200);
      res.end(JSON.stringify({ items }));
      return;
    }
    if (match) {
      if (archivedIds.has(match[1])) {
        archivedDetailReads++;
        archivedDetailStatus = 404;
        archivedDetailId = match[1];
        res.writeHead(404);
        res.end(JSON.stringify({ error: { code: "not_found" } }));
        return;
      }
      if (match[1] === missingId) {
        res.end(JSON.stringify({ data: space(missingId, undefined) }));
        return;
      }
      if (match[1] === unavailableSpaceId) {
        // Model a known non-public listing whose public detail is deliberately hidden.
        unavailableDetailReads++;
        unavailableDetailStatus = 404;
        unavailableDetailId = match[1];
        res.writeHead(404);
        res.end(JSON.stringify({ error: { code: "not_found" } }));
        return;
      }
      if (
        ![id, inactiveId, invalidTimeZoneId, supportedAliasId].includes(
          match[1],
        )
      ) {
        res.writeHead(404);
        res.end(JSON.stringify({ error: { code: "not_found" } }));
        return;
      }
      const schedule =
        match[1] === inactiveId
          ? inactive
          : match[1] === invalidTimeZoneId
            ? invalidTimeZone
            : match[1] === supportedAliasId
              ? supportedAlias
              : active;
      const data = space(match[1], schedule);
      res.end(JSON.stringify({ data }));
      return;
    }
    if (path === "/__fixture/observations") {
      res.writeHead(200);
      res.end(
        JSON.stringify({
          unavailableDetailReads,
          unavailableDetailStatus,
          unavailableDetailId,
          archivedDetailReads,
          archivedDetailStatus,
          archivedDetailId,
          publicListReads,
          lastPublicListQuery,
          lastPublicListItems,
        }),
      );
      return;
    }
    if (path === "/healthz") {
      res.writeHead(200);
      res.end(JSON.stringify({ status: "ok" }));
      return;
    }
    if (path === "/api/v1/public/space-categories") {
      res.end(JSON.stringify({ items: [] }));
      return;
    }
    if (path === "/api/v1/public/locations") {
      res.end(JSON.stringify({ items: [] }));
      return;
    }
    res.writeHead(404);
    res.end(JSON.stringify({ error: { code: "not_found" } }));
  })
  .listen(port, "127.0.0.1");
