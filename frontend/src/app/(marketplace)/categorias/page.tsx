import type { Metadata } from "next";
import Link from "next/link";
import { publicCatalog } from "@/features/public-catalog/infrastructure/catalog-runtime";
import { categoryLabel } from "@/features/public-catalog/domain/presentation";
import { CatalogUnavailable } from "@/features/public-catalog/ui/catalog-states";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Categorías de arriendo",
  description:
    "Explora las categorías públicas de arriendos residenciales y espacios flexibles de EspaciGo.",
  alternates: { canonical: "/categorias" },
};

export default async function CategoriesPage() {
  const result = await publicCatalog.categories();
  if (!result.ok) {
    return (
      <section className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
        <CatalogUnavailable
          title="No pudimos cargar las categorías"
          retryHref="/categorias"
        />
      </section>
    );
  }
  const categories = [...result.value.items].sort((a, b) => a.order - b.order);
  return (
    <section className="mx-auto max-w-7xl px-4 py-10 sm:px-6 sm:py-14 lg:px-8">
      <nav
        aria-label="Breadcrumb"
        className="mb-7 text-sm text-muted-foreground"
      >
        <Link href="/" className="hover:text-foreground">
          Inicio
        </Link>
        <span aria-hidden="true"> / </span>
        <span aria-current="page">Categorías</span>
      </nav>
      <header className="max-w-3xl border-b border-border pb-9">
        <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
          Desde el catálogo público
        </p>
        <h1 className="mt-3 text-4xl font-medium tracking-tight sm:text-5xl">
          Categorías de arriendo
        </h1>
        <p className="mt-4 text-lg leading-relaxed text-muted-foreground">
          Estas categorías corresponden a las que Go informa como activas. Que
          una categoría exista no significa que tenga publicaciones disponibles.
        </p>
      </header>
      {categories.length ? (
        <ul className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {categories.map((category) => (
            <li key={category.code}>
              <Link
                href={`/categorias/${encodeURIComponent(category.code)}`}
                className="group flex min-h-36 flex-col justify-between rounded-2xl border border-border p-6 transition-colors hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <span className="text-xs tabular-nums text-muted-foreground">
                  {String(category.order).padStart(2, "0")}
                </span>
                <span className="flex items-center justify-between gap-4 text-xl font-medium">
                  {categoryLabel(category)}
                  <span
                    aria-hidden="true"
                    className="text-muted-foreground transition-transform group-hover:translate-x-1"
                  >
                    →
                  </span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-10 border-t border-border py-10 text-muted-foreground">
          No hay categorías activas disponibles en el catálogo en este momento.
        </p>
      )}
    </section>
  );
}
