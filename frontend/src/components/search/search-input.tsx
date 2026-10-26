import { Search } from "lucide-react";
import Link from "next/link";

export function SearchInput() {
  return (
    <>
      <Link
        href="/explorar"
        aria-label="Buscar arriendos"
        className="hidden rounded-full p-2 transition-colors hover:bg-secondary lg:inline-flex xl:hidden focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <Search className="h-6 w-6" strokeWidth={1.75} aria-hidden="true" />
      </Link>
      <form
        action="/explorar"
        method="get"
        role="search"
        aria-label="Buscar publicaciones"
        className="relative hidden xl:block"
      >
        <Search
          className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
          strokeWidth={1.75}
          aria-hidden="true"
        />
        <label htmlFor="header-public-search" className="sr-only">
          Buscar arriendos
        </label>
        <input
          id="header-public-search"
          type="search"
          name="q"
          maxLength={120}
          placeholder="Buscar arriendos"
          className="h-10 w-56 rounded-full border border-border bg-background pl-9 pr-3 text-sm text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
      </form>
    </>
  );
}
