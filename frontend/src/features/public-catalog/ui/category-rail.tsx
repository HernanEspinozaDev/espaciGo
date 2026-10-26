import Link from "next/link";
import type { PublicSpaceCategory } from "../domain/types";
import { categoryLabel } from "../domain/presentation";

export function CategoryRail({
  categories,
}: {
  categories: readonly PublicSpaceCategory[];
}) {
  return (
    <section className="mx-auto max-w-7xl px-4 py-14 sm:px-6 sm:py-20 lg:px-8">
      <div className="mb-7 flex items-end justify-between gap-6">
        <div>
          <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
            Explora por tipo
          </p>
          <h2 className="mt-2 text-2xl font-medium tracking-tight sm:text-3xl">
            Categorías del catálogo
          </h2>
        </div>
        <Link
          href="/categorias"
          className="hidden min-h-11 items-center text-sm font-medium text-muted-foreground underline-offset-4 hover:text-foreground hover:underline sm:inline-flex"
        >
          Ver categorías
        </Link>
      </div>
      <nav aria-label="Categorías de arriendo">
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {categories.map((category) => (
            <li key={category.code}>
              <Link
                href={`/categorias/${encodeURIComponent(category.code)}`}
                className="group flex min-h-24 items-end justify-between gap-3 rounded-2xl border border-border bg-background p-5 transition-colors hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <span className="max-w-[18ch] text-base font-medium leading-snug text-foreground sm:text-lg">
                  {categoryLabel(category)}
                </span>
                <span
                  aria-hidden="true"
                  className="text-lg text-muted-foreground transition-transform group-hover:translate-x-1"
                >
                  →
                </span>
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </section>
  );
}
