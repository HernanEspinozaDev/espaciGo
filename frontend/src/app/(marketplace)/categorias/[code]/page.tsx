import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { hasCatalogFilters } from "@/features/public-catalog/domain/query-state";
import { parseCatalogQuery } from "@/features/public-catalog/domain/query-state";
import { categoryLabel } from "@/features/public-catalog/domain/presentation";
import type {
  PublicCategoryFilter,
  RawCatalogParams,
} from "@/features/public-catalog/domain/types";
import { publicCatalog } from "@/features/public-catalog/infrastructure/catalog-runtime";
import { CatalogFilters } from "@/features/public-catalog/ui/catalog-filters";
import { CatalogSortControls } from "@/features/public-catalog/ui/catalog-sort-controls";
import {
  CatalogUnavailable,
  EmptyCatalog,
} from "@/features/public-catalog/ui/catalog-states";
import { PublicCatalogGrid } from "@/features/public-catalog/ui/catalog-sections";
import { PublicSearchForm } from "@/features/public-catalog/ui/public-search-form";

export const dynamic = "force-dynamic";

function pathFor(code: string) {
  return `/categorias/${encodeURIComponent(code)}`;
}

export async function generateMetadata({
  params,
  searchParams,
}: {
  params: Promise<{ code: string }>;
  searchParams: Promise<RawCatalogParams>;
}): Promise<Metadata> {
  const { code } = await params;
  const raw = await searchParams;
  const [categories, query] = await Promise.all([
    publicCatalog.categories(),
    Promise.resolve(parseCatalogQuery({ ...raw, category_code: undefined })),
  ]);
  const category = categories.ok
    ? categories.value.items.find((item) => item.code === code)
    : undefined;
  const title = category ? categoryLabel(category) : "Categoría";
  const hasRawAttributes = Boolean(
    raw.attribute &&
    (Array.isArray(raw.attribute) ? raw.attribute.length : raw.attribute),
  );
  return {
    title: `${title} en arriendo`,
    description: `Explora publicaciones públicas de ${title} en EspaciGo, si están disponibles en el catálogo.`,
    alternates: { canonical: pathFor(code) },
    robots:
      hasRawAttributes || hasCatalogFilters(query)
        ? { index: false, follow: true }
        : { index: true, follow: true },
  };
}

export default async function CategoryPage({
  params,
  searchParams,
}: {
  params: Promise<{ code: string }>;
  searchParams: Promise<RawCatalogParams>;
}) {
  const [{ code }, raw] = await Promise.all([params, searchParams]);
  const categoriesResult = await publicCatalog.categories();
  if (!categoriesResult.ok) {
    return (
      <section className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
        <CatalogUnavailable
          title="No pudimos verificar esta categoría"
          retryHref={pathFor(code)}
        />
      </section>
    );
  }
  const categories = [...categoriesResult.value.items].sort(
    (a, b) => a.order - b.order,
  );
  const category = categories.find((item) => item.code === code);
  if (!category) notFound();

  const [locationsResult, profileResult] = await Promise.all([
    publicCatalog.locations({ limit: 400 }),
    publicCatalog.categoryFilters(code),
  ]);
  let categoryFilters: readonly PublicCategoryFilter[] = [];
  const profileUnavailable =
    !profileResult.ok && profileResult.error.status !== 404;
  if (profileResult.ok) categoryFilters = profileResult.value.items;

  const { query, result } = await publicCatalog.spacesFromParams(
    { ...raw, category_code: code },
    categoryFilters,
  );
  const locations = locationsResult.ok ? locationsResult.value.items : [];
  const residential = category.code === "inmuebles";

  return (
    <section className="mx-auto max-w-7xl px-4 pb-24 pt-8 sm:px-6 sm:pt-12 lg:px-8">
      <nav
        aria-label="Breadcrumb"
        className="mb-7 text-sm text-muted-foreground"
      >
        <Link href="/" className="hover:text-foreground">
          Inicio
        </Link>
        <span aria-hidden="true"> / </span>
        <Link href="/categorias" className="hover:text-foreground">
          Categorías
        </Link>
        <span aria-hidden="true"> / </span>
        <span aria-current="page">{categoryLabel(category)}</span>
      </nav>
      <header className="mb-8 border-b border-border pb-8">
        <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
          Categoría activa
        </p>
        <h1 className="mt-3 text-4xl font-medium tracking-tight sm:text-5xl">
          {categoryLabel(category)} en arriendo
        </h1>
        {residential ? (
          <p className="mt-4 max-w-3xl leading-relaxed text-muted-foreground">
            Busca casas, departamentos, habitaciones, estudios, cabañas y casas
            de veraneo cuando haya anuncios públicos elegibles. La categoría no
            implica inventario disponible.
          </p>
        ) : null}
        <PublicSearchForm initialQuery={query.q ?? ""} />
      </header>
      <div className="grid gap-8 lg:grid-cols-[16rem_minmax(0,1fr)] lg:gap-10">
        <CatalogFilters
          actionPath={pathFor(code)}
          categories={categories}
          locations={locations}
          filters={categoryFilters}
          query={query}
          fixedCategoryCode={code}
        />
        <div className="min-w-0">
          <div className="mb-7 flex flex-col gap-4 border-b border-border pb-5 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm text-muted-foreground">
              Publicaciones del catálogo
            </p>
            <CatalogSortControls actionPath={pathFor(code)} query={query} />
          </div>
          {!locationsResult.ok || profileUnavailable ? (
            <p
              role="status"
              className="mb-5 rounded-xl border border-border p-4 text-sm text-muted-foreground"
            >
              Algunos filtros no están disponibles temporalmente; las
              publicaciones se consultan igualmente en Go.
            </p>
          ) : null}
          {!result.ok ? (
            result.error.kind === "invalid" ? (
              <section role="alert" className="border-y border-border py-12">
                <h2 className="text-2xl font-medium">
                  No pudimos aplicar esos filtros
                </h2>
                <p className="mt-3 text-muted-foreground">
                  Restablece los filtros y vuelve a intentarlo.
                </p>
                <Link
                  href={pathFor(code)}
                  className="mt-6 inline-flex min-h-11 items-center rounded-full bg-foreground px-5 text-sm font-medium text-background"
                >
                  Restablecer filtros
                </Link>
              </section>
            ) : (
              <CatalogUnavailable
                title="No pudimos cargar esta categoría"
                retryHref={pathFor(code)}
              />
            )
          ) : result.value.items.length ? (
            <PublicCatalogGrid
              spaces={result.value.items}
              query={query}
              nextCursor={result.value.next_cursor}
              actionPath={pathFor(code)}
            />
          ) : (
            <EmptyCatalog
              title={
                residential
                  ? "Aún no hay publicaciones residenciales"
                  : `Aún no hay publicaciones públicas en ${categoryLabel(category)}`
              }
              description={
                residential
                  ? "La categoría admite varias modalidades residenciales, pero hoy no hay anuncios elegibles para mostrar."
                  : "Las publicaciones se mostrarán cuando Go las habilite para el catálogo público."
              }
              ownerCta={residential}
            />
          )}
        </div>
      </div>
    </section>
  );
}
