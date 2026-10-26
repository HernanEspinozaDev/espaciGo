import type { Metadata } from "next";
import Link from "next/link";
import {
  parseCatalogQuery,
  hasCatalogFilters,
} from "@/features/public-catalog/domain/query-state";
import type {
  RawCatalogParams,
  PublicCategoryFilter,
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

export async function generateMetadata({
  searchParams,
}: {
  searchParams: Promise<RawCatalogParams>;
}): Promise<Metadata> {
  const query = parseCatalogQuery(await searchParams);
  const filtered = hasCatalogFilters(query);
  return {
    title: query.q ? `Búsqueda: ${query.q}` : "Explorar arriendos",
    description:
      "Explora publicaciones públicas de arriendos residenciales y espacios flexibles en EspaciGo.",
    alternates: { canonical: "/explorar" },
    robots: filtered
      ? { index: false, follow: true }
      : { index: true, follow: true },
  };
}

function first(params: RawCatalogParams, key: string) {
  const value = params[key];
  return typeof value === "string" ? value : undefined;
}

export default async function ExplorePage({
  searchParams,
}: {
  searchParams: Promise<RawCatalogParams>;
}) {
  const raw = await searchParams;
  const [categoriesResult, locationsResult] = await Promise.all([
    publicCatalog.categories(),
    publicCatalog.locations({ limit: 400 }),
  ]);

  let effectiveParams: RawCatalogParams = { ...raw };
  let invalidCategory = false;
  const requestedCategory = first(raw, "category_code");
  if (
    categoriesResult.ok &&
    requestedCategory &&
    !categoriesResult.value.items.some(
      (category) => category.code === requestedCategory,
    )
  ) {
    effectiveParams = {
      ...effectiveParams,
      category_code: undefined,
      attribute: undefined,
    };
    invalidCategory = true;
  }

  const categoryCode = first(effectiveParams, "category_code");
  let categoryFilters: readonly PublicCategoryFilter[] = [];
  let filterProfileUnavailable = false;
  if (categoryCode) {
    const profile = await publicCatalog.categoryFilters(categoryCode);
    if (profile.ok) categoryFilters = profile.value.items;
    else if (profile.error.status !== 404) filterProfileUnavailable = true;
  }

  const { query, result } = await publicCatalog.spacesFromParams(
    effectiveParams,
    categoryFilters,
  );
  const categories = categoriesResult.ok ? categoriesResult.value.items : [];
  const locations = locationsResult.ok ? locationsResult.value.items : [];
  const rawAttributes = raw.attribute;
  const requestedAttributeCount = Array.isArray(rawAttributes)
    ? rawAttributes.length
    : rawAttributes
      ? 1
      : 0;
  const attributesWereDiscarded =
    requestedAttributeCount > query.attribute.length;

  return (
    <section className="mx-auto max-w-7xl px-4 pb-24 pt-8 sm:px-6 sm:pt-12 lg:px-8">
      <header className="mb-8 border-b border-border pb-8 sm:mb-10 sm:pb-10">
        <nav
          aria-label="Breadcrumb"
          className="mb-6 text-sm text-muted-foreground"
        >
          <Link href="/" className="hover:text-foreground">
            Inicio
          </Link>
          <span aria-hidden="true"> / </span>
          <span aria-current="page">Explorar</span>
        </nav>
        <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
          Catálogo público
        </p>
        <h1 className="mt-3 text-3xl font-medium tracking-tight sm:text-5xl">
          Explorar arriendos
        </h1>
        <p className="mt-3 max-w-2xl text-muted-foreground">
          Busca inmuebles residenciales y espacios flexibles por los campos que
          Go expone públicamente.
        </p>
        <PublicSearchForm initialQuery={query.q ?? ""} />
      </header>

      <div className="grid gap-8 lg:grid-cols-[16rem_minmax(0,1fr)] lg:gap-10">
        <CatalogFilters
          actionPath="/explorar"
          categories={categories}
          locations={locations}
          filters={categoryFilters}
          query={query}
        />
        <div className="min-w-0">
          <div className="mb-7 flex flex-col gap-4 border-b border-border pb-5 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm text-muted-foreground">
              {query.q
                ? `Resultados para “${query.q}”`
                : "Publicaciones del catálogo"}
            </p>
            <CatalogSortControls actionPath="/explorar" query={query} />
          </div>

          {invalidCategory ? (
            <p
              role="status"
              className="mb-5 rounded-xl border border-border p-4 text-sm text-muted-foreground"
            >
              La categoría seleccionada ya no está activa; se omitió ese filtro.
            </p>
          ) : null}
          {!categoriesResult.ok ? (
            <p
              role="status"
              className="mb-5 rounded-xl border border-border p-4 text-sm text-muted-foreground"
            >
              Las categorías no están disponibles temporalmente; puedes seguir
              explorando sin ese selector.
            </p>
          ) : null}
          {!locationsResult.ok ? (
            <p
              role="status"
              className="mb-5 rounded-xl border border-border p-4 text-sm text-muted-foreground"
            >
              Las ubicaciones no están disponibles temporalmente; puedes seguir
              explorando sin ese filtro.
            </p>
          ) : null}
          {filterProfileUnavailable || attributesWereDiscarded ? (
            <p
              role="status"
              className="mb-5 rounded-xl border border-border p-4 text-sm text-muted-foreground"
            >
              Algunos filtros de categoría no pudieron validarse con el perfil
              público y se omitieron.
            </p>
          ) : null}

          {!result.ok ? (
            result.error.kind === "invalid" ? (
              <section role="alert" className="border-y border-border py-12">
                <h2 className="text-2xl font-medium">
                  No pudimos aplicar esos filtros
                </h2>
                <p className="mt-3 max-w-xl text-muted-foreground">
                  Revisa los valores o restablece la búsqueda para continuar.
                </p>
                <Link
                  href="/explorar"
                  className="mt-6 inline-flex min-h-11 items-center rounded-full bg-foreground px-5 text-sm font-medium text-background"
                >
                  Restablecer filtros
                </Link>
              </section>
            ) : (
              <CatalogUnavailable title="No pudimos cargar las publicaciones" />
            )
          ) : result.value.items.length ? (
            <PublicCatalogGrid
              spaces={result.value.items}
              query={query}
              nextCursor={result.value.next_cursor}
              actionPath="/explorar"
            />
          ) : (
            <EmptyCatalog
              title={
                query.q
                  ? `No encontramos publicaciones para “${query.q}”`
                  : hasCatalogFilters(query)
                    ? "No hay publicaciones que coincidan con estos filtros"
                    : "Aún no hay publicaciones públicas en el catálogo"
              }
              description={
                hasCatalogFilters(query)
                  ? "Prueba con otra palabra, ubicación o combinación de filtros."
                  : "Vuelve a explorar más tarde para revisar las publicaciones que Go habilite para el catálogo público."
              }
            />
          )}
        </div>
      </div>
    </section>
  );
}
