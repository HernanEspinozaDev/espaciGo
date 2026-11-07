import http from "node:http";

const port = Number(process.env.PUBLIC_CALENDAR_FIXTURE_PORT ?? 18088);
const id = "00000000-0000-4000-8000-000000000001";
const missingId = "00000000-0000-4000-8000-000000000002";
const inactiveId = "00000000-0000-4000-8000-000000000003";
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
http
  .createServer((req, res) => {
    const path = new URL(req.url, "http://127.0.0.1").pathname;
    const match = path.match(/\/public\/spaces\/([^/]+)$/);
    res.setHeader("content-type", "application/json");
    if (match) {
      if (match[1] === missingId) {
        res.end(JSON.stringify({ data: space(missingId, undefined) }));
        return;
      }
      if (![id, inactiveId].includes(match[1])) {
        res.writeHead(404);
        res.end(JSON.stringify({ error: { code: "not_found" } }));
        return;
      }
      const data = space(match[1], match[1] === inactiveId ? inactive : active);
      res.end(JSON.stringify({ data }));
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
