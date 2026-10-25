import { Search } from "lucide-react";

export function PublicSearchForm({
  initialQuery = "",
}: {
  initialQuery?: string;
}) {
  return (
    <form
      action="/explorar"
      method="get"
      role="search"
      aria-label="Buscar publicaciones"
      className="mt-8 flex max-w-2xl flex-col gap-3 rounded-2xl border border-border bg-background p-3 sm:flex-row sm:items-center sm:rounded-full sm:pl-5"
    >
      <label htmlFor="home-search" className="sr-only">
        ¿Qué quieres arrendar?
      </label>
      <Search
        className="hidden size-5 shrink-0 text-muted-foreground sm:block"
        aria-hidden="true"
      />
      <input
        id="home-search"
        type="search"
        name="q"
        maxLength={120}
        defaultValue={initialQuery}
        placeholder="Casa, oficina, bodega, quincho…"
        className="h-11 min-w-0 flex-1 bg-transparent px-3 text-base outline-none placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring sm:px-0"
      />
      <button
        type="submit"
        className="inline-flex min-h-11 items-center justify-center gap-2 rounded-full bg-foreground px-6 text-sm font-medium text-background transition-colors hover:bg-foreground/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        Explorar arriendos
      </button>
    </form>
  );
}
