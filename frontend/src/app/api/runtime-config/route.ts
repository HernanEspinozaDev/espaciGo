export const dynamic = "force-dynamic";

export function GET(): Response {
  return Response.json(
    { apiBasePath: "/api/backend", sessionMode: "memory" },
    { headers: { "cache-control": "no-store" } },
  );
}
