import { ArrowLeft, ArrowRight } from "lucide-react";
import Link from "next/link";

function buildSearchUrl({
  query,
  page,
  sort,
  category,
}: {
  query: string;
  page: number;
  sort?: string;
  category?: string;
}) {
  const params = new URLSearchParams({ q: query });
  if (page > 1) params.set("page", String(page));
  if (sort && sort !== "newest") params.set("sort", sort);
  if (category) params.set("category", category);
  return `/search?${params.toString()}`;
}

export function SearchPagination({
  currentPage,
  totalPages,
  query,
  sort,
  category,
}: {
  currentPage: number;
  totalPages: number;
  query: string;
  sort?: string;
  category?: string;
}) {
  if (totalPages <= 1) return null;
  const previous =
    currentPage > 1
      ? buildSearchUrl({ query, page: currentPage - 1, sort, category })
      : null;
  const next =
    currentPage < totalPages
      ? buildSearchUrl({ query, page: currentPage + 1, sort, category })
      : null;

  return (
    <nav
      aria-label="Paginación de resultados"
      className="mt-20 flex items-center justify-between border-t border-border pt-8"
    >
      {previous ? (
        <Link
          href={previous}
          rel="prev"
          className="group inline-flex min-h-11 items-center gap-3 text-xs uppercase tracking-[0.18em] text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft
            className="h-3.5 w-3.5 transition-transform group-hover:-translate-x-0.5"
            aria-hidden="true"
          />{" "}
          Anterior
        </Link>
      ) : (
        <span className="text-xs uppercase tracking-[0.18em] text-muted-foreground/50">
          Anterior
        </span>
      )}
      <span className="text-sm tabular-nums text-muted-foreground">
        <span className="font-medium text-foreground">{currentPage}</span> /{" "}
        {totalPages}
      </span>
      {next ? (
        <Link
          href={next}
          rel="next"
          className="group inline-flex min-h-11 items-center gap-3 text-xs uppercase tracking-[0.18em] text-muted-foreground transition-colors hover:text-foreground"
        >
          Siguiente{" "}
          <ArrowRight
            className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5"
            aria-hidden="true"
          />
        </Link>
      ) : (
        <span className="text-xs uppercase tracking-[0.18em] text-muted-foreground/50">
          Siguiente
        </span>
      )}
    </nav>
  );
}
