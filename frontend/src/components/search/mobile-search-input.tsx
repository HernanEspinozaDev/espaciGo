"use client";

import { Search } from "lucide-react";

export function MobileSearchInput({ onNavigate }: { onNavigate: () => void }) {
  return (
    <search aria-label="Buscar publicaciones">
      <form action="/explorar" method="get" onSubmit={onNavigate}>
        <label htmlFor="mobile-public-search" className="sr-only">
          Buscar arriendos
        </label>
        <div className="relative">
          <Search
            className="pointer-events-none absolute left-4 top-1/2 h-5 w-5 -translate-y-1/2 text-muted-foreground"
            strokeWidth={1.75}
            aria-hidden="true"
          />
          <input
            id="mobile-public-search"
            type="search"
            name="q"
            maxLength={120}
            placeholder="Casa, oficina, quincho…"
            enterKeyHint="search"
            className="h-12 w-full rounded-full border border-border bg-background pl-12 pr-4 text-base text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />
        </div>
      </form>
    </search>
  );
}
